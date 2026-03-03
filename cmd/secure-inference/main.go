package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sync/atomic"

	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"

	accesscontrolv1alpha1 "github.com/llm-d-incubation/secure-inference/api/v1alpha1"
	"github.com/llm-d-incubation/secure-inference/pkg/adapterselection"
	"github.com/llm-d-incubation/secure-inference/pkg/auth"
	"github.com/llm-d-incubation/secure-inference/pkg/config"
	"github.com/llm-d-incubation/secure-inference/pkg/controller"
	"github.com/llm-d-incubation/secure-inference/pkg/policyengine"
	"github.com/llm-d-incubation/secure-inference/pkg/runnable"
	"github.com/llm-d-incubation/secure-inference/pkg/server"
	"github.com/llm-d-incubation/secure-inference/pkg/store"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(accesscontrolv1alpha1.AddToScheme(scheme))
}

func main() {
	var (
		configPath           string
		authPort             int
		metricsAddr          string
		healthAddr           string
		enableLeaderElection bool
	)

	flag.StringVar(&configPath, "config", "", "Path to config file (default: built-in defaults)")
	flag.IntVar(&authPort, "auth-port", 9000, "Port for the ext-auth gRPC server")
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metrics endpoint binds to")
	flag.StringVar(&healthAddr, "health-probe-bind-address", ":8081", "The address the health probe endpoint binds to")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager")

	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	logf.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	logger := logf.Log.WithName("secure-inference")

	ctx := context.Background()

	// Load config
	cfg := config.DefaultConfig()
	if configPath != "" {
		var err error
		cfg, err = config.LoadConfig(configPath)
		if err != nil {
			logger.Error(err, "Failed to load config file")
			os.Exit(1)
		}
		logger.Info("Loaded config file", "path", configPath)
	}

	if err := cfg.Validate(); err != nil {
		logger.Error(err, "Invalid configuration")
		os.Exit(1)
	}

	// Initialize data store from config
	dataStore, err := store.NewStoreFromConfig(ctx, cfg.DataStore)
	if err != nil {
		logger.Error(err, "Failed to create data store")
		os.Exit(1)
	}
	defer dataStore.Close()

	// Initialize policy engine from config
	engine, err := policyengine.NewPolicyEngineFromConfig(ctx, cfg.PolicyEngine)
	if err != nil {
		logger.Error(err, "Failed to create policy engine")
		os.Exit(1)
	}
	logger.Info("Policy engine initialized", "type", cfg.PolicyEngine.Type)

	// Shared synced flag for readiness
	var synced atomic.Bool

	// Create controller-runtime manager
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: healthAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "secure-inference.llm-d.io",
	})
	if err != nil {
		logger.Error(err, "Unable to create manager")
		os.Exit(1)
	}

	// Register reconcilers — controllers use Store directly
	if err = (&controller.UserReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Store:  dataStore,
		Synced: &synced,
	}).SetupWithManager(mgr); err != nil {
		logger.Error(err, "Unable to create controller", "controller", "User")
		os.Exit(1)
	}

	if err = (&controller.ModelReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Store:  dataStore,
		Synced: &synced,
	}).SetupWithManager(mgr); err != nil {
		logger.Error(err, "Unable to create controller", "controller", "Model")
		os.Exit(1)
	}

	// Create adapter selector from config
	selector, err := adapterselection.NewAdapterSelectionFromConfig(ctx, cfg.AdapterSelection)
	if err != nil {
		logger.Error(err, "Failed to create adapter selector")
		os.Exit(1)
	}
	if selector != nil {
		logger.Info("Adapter selection enabled",
			"type", cfg.AdapterSelection.Type,
			"url", cfg.AdapterSelection.Parameters["url"],
		)
	}

	// Create authenticator from config
	authenticator, err := auth.NewAuthenticatorFromConfig(ctx, cfg.Auth)
	if err != nil {
		logger.Error(err, "Failed to create authenticator")
		os.Exit(1)
	}

	// Create ext-auth gRPC server — holds engine + store + authenticator + selector
	authServer := server.NewExtAuthzServer(engine, dataStore, authenticator, selector, cfg.AdapterSelection.AlwaysActive)

	grpcSrv := grpc.NewServer()
	authv3.RegisterAuthorizationServer(grpcSrv, authServer.V3())

	// Register ext-auth as a Runnable (runs on ALL replicas, not just leader)
	if err = mgr.Add(runnable.NoLeaderElection(
		runnable.GRPCServer("ext-auth", grpcSrv, authPort),
	)); err != nil {
		logger.Error(err, "Unable to add ext-auth gRPC server")
		os.Exit(1)
	}

	// Register health checks
	if err = mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		logger.Error(err, "Unable to set up health check")
		os.Exit(1)
	}
	if err = mgr.AddReadyzCheck("readyz", func(req *http.Request) error {
		if !synced.Load() {
			return fmt.Errorf("policy engine not synced")
		}
		return nil
	}); err != nil {
		logger.Error(err, "Unable to set up readiness check")
		os.Exit(1)
	}

	logger.Info("Starting secure-inference", "auth-port", authPort)
	if err = mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		logger.Error(err, "Problem running manager")
		os.Exit(1)
	}
}
