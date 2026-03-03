package server

import (
	"context"
	"fmt"

	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"

	v1alpha1 "github.com/llm-d-incubation/secure-inference/api/v1alpha1"
	"github.com/llm-d-incubation/secure-inference/pkg/adapterselection"
	"github.com/llm-d-incubation/secure-inference/pkg/auth"
	"github.com/llm-d-incubation/secure-inference/pkg/policyengine"
	"github.com/llm-d-incubation/secure-inference/pkg/store"
	"github.com/llm-d-incubation/secure-inference/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var logger = logf.Log.WithName("ext-auth")

const (
	resultHeader       = "x-ext-authz-check-result"
	resultAllowed      = "allowed"
	resultDenied       = "denied"
	notApplicable      = "not-applicable"
	modelRewriteHeader = "x-gateway-model-name-rewrite"
)

type extAuthzServerV3 struct {
	authenticator                auth.Authenticator
	store                        store.ReadStore
	engine                       policyengine.PolicyEngine
	selector                     adapterselection.Selector // may be nil
	alwaysActiveAdapterSelection bool                      // when false, adapter selection requires x-adapter-selection: true header
}

func (s *extAuthzServerV3) logRequest(result string, request *authv3.CheckRequest) {
	httpAttrs := request.GetAttributes().GetRequest().GetHttp()
	logger.V(1).Info("Request", "result", result, "host", httpAttrs.GetHost(), "path", httpAttrs.GetPath())
}

func (s *extAuthzServerV3) allow(request *authv3.CheckRequest) *authv3.CheckResponse {
	s.logRequest("allowed", request)
	return &authv3.CheckResponse{
		HttpResponse: &authv3.CheckResponse_OkResponse{
			OkResponse: &authv3.OkHttpResponse{
				Headers: []*corev3.HeaderValueOption{
					{
						Header: &corev3.HeaderValue{
							Key:   resultHeader,
							Value: resultAllowed,
						},
					},
				},
			},
		},
		Status: &status.Status{Code: int32(codes.OK)},
	}
}

func (s *extAuthzServerV3) allowWithModelRewrite(request *authv3.CheckRequest, loraModelID string) *authv3.CheckResponse {
	s.logRequest("allowed", request)
	headers := []*corev3.HeaderValueOption{
		{
			Header: &corev3.HeaderValue{
				Key:   resultHeader,
				Value: resultAllowed,
			},
		},
	}
	if loraModelID != "" {
		headers = append(headers, &corev3.HeaderValueOption{
			Header: &corev3.HeaderValue{
				Key:   modelRewriteHeader,
				Value: loraModelID,
			},
		})
	}
	return &authv3.CheckResponse{
		HttpResponse: &authv3.CheckResponse_OkResponse{
			OkResponse: &authv3.OkHttpResponse{
				Headers: headers,
			},
		},
		Status: &status.Status{Code: int32(codes.OK)},
	}
}

func (s *extAuthzServerV3) allowWithoutClaims(request *authv3.CheckRequest) *authv3.CheckResponse {
	s.logRequest("allowed", request)
	return &authv3.CheckResponse{
		HttpResponse: &authv3.CheckResponse_OkResponse{
			OkResponse: &authv3.OkHttpResponse{
				Headers: []*corev3.HeaderValueOption{
					{
						Header: &corev3.HeaderValue{
							Key:   resultHeader,
							Value: notApplicable,
						},
					},
				},
			},
		},
		Status: &status.Status{Code: int32(codes.OK)},
	}
}

func (s *extAuthzServerV3) deny(request *authv3.CheckRequest, reason string) *authv3.CheckResponse {
	s.logRequest("denied", request)
	return &authv3.CheckResponse{
		HttpResponse: &authv3.CheckResponse_DeniedResponse{
			DeniedResponse: &authv3.DeniedHttpResponse{
				Status: &typev3.HttpStatus{Code: typev3.StatusCode_Forbidden},
				Body:   fmt.Sprintf("Denied: %s", reason),
				Headers: []*corev3.HeaderValueOption{
					{
						Header: &corev3.HeaderValue{
							Key:   resultHeader,
							Value: resultDenied,
						},
					},
				},
			},
		},
		Status: &status.Status{Code: int32(codes.PermissionDenied)},
	}
}

func (s *extAuthzServerV3) Check(ctx context.Context, request *authv3.CheckRequest) (*authv3.CheckResponse, error) {
	// Parse the request into the internal representation
	ir, err := types.ParseCheckRequest(request)
	if err != nil {
		logger.V(1).Info("Failed to parse request", "error", err)
		return s.deny(request, err.Error()), nil
	}

	// Unprotected paths pass through without auth
	if ir.Type == types.RequestTypeUnknown {
		return s.allowWithoutClaims(request), nil
	}

	// Authenticate the request
	result, err := s.authenticator.Authenticate(ctx, ir)
	if err != nil {
		logger.V(1).Info("Authentication failed", "error", err)
		return s.deny(request, err.Error()), nil
	}

	// Check user exists in store
	isUserValid, err := s.store.UserExists(ctx, result.UserID)
	if err != nil {
		logger.Error(err, "Error validating username", "user", result.UserID)
		return s.deny(request, "user validation error"), nil
	}
	if !isUserValid {
		logger.V(1).Info("User not valid", "user", result.UserID)
		return s.deny(request, "user not registered"), nil
	}

	// /v1/models — allow for any valid user
	if ir.Type == types.RequestTypeListModels {
		logger.V(1).Info("List models allowed")
		return s.allow(request), nil
	}

	// Completion/chat paths require a model ID
	if ir.ModelID == "" {
		logger.V(1).Info("Missing model key in request body")
		return s.deny(request, "missing model field in request body"), nil
	}

	// Get user and model from store for policy evaluation
	user, err := s.store.GetUser(ctx, result.UserID)
	if err != nil {
		logger.Error(err, "Failed to get user", "user", result.UserID)
		return s.deny(request, "user not found"), nil
	}

	model, err := s.store.GetModel(ctx, ir.ModelID)
	if err != nil {
		logger.Error(err, "Failed to get model", "model", ir.ModelID)
		return s.deny(request, "model not found"), nil
	}

	decision, err := s.engine.CheckAccess(ctx, user, model)
	if err != nil {
		logger.Error(err, "Failed to evaluate policy")
		return s.deny(request, "policy evaluation error"), nil
	}

	if decision {
		// Try adapter selection if enabled and the model is a base model
		if s.selector != nil && model.Type == v1alpha1.ModelTypeBase {
			shouldSelect := s.alwaysActiveAdapterSelection
			if !shouldSelect {
				if val, ok := ir.Headers["x-adapter-selection"]; ok && val == "true" {
					shouldSelect = true
				}
			}

			if shouldSelect && ir.PromptText() != "" {
				loraModels, err := s.store.ListModelsByType(ctx, v1alpha1.ModelTypeLora)
				if err == nil {
					var candidates []*v1alpha1.ModelSpec
					for i := range loraModels {
						if loraModels[i].BaseModelId == ir.ModelID {
							candidates = append(candidates, &loraModels[i])
						}
					}

					if len(candidates) > 0 {
						allowedLoras, err := s.engine.GetAllowedModels(ctx, user, candidates)
						if err == nil && len(allowedLoras) > 0 {
							selected, matched := s.selector.Pick(ctx, ir, allowedLoras)
							if matched {
								return s.allowWithModelRewrite(request, selected.Id), nil
							}
						}
					}
				}
			}
		}
		return s.allow(request), nil
	}

	return s.deny(request, "access denied by policy"), nil
}

// ExtAuthzServer wraps the ext-auth gRPC v3 server.
type ExtAuthzServer struct {
	grpcV3 *extAuthzServerV3
}

// V3 returns the v3 authorization server implementation for gRPC registration.
func (s *ExtAuthzServer) V3() authv3.AuthorizationServer {
	return s.grpcV3
}

// NewExtAuthzServer creates an ext-auth server backed by an Authenticator, PolicyEngine, and Store.
// selector may be nil if adapter selection is disabled.
// alwaysActiveAdapterSelection controls whether adapter selection runs unconditionally (true)
// or only when the x-adapter-selection: true header is present (false).
func NewExtAuthzServer(
	engine policyengine.PolicyEngine,
	st store.ReadStore,
	authenticator auth.Authenticator,
	selector adapterselection.Selector,
	alwaysActiveAdapterSelection bool,
) *ExtAuthzServer {
	return &ExtAuthzServer{
		grpcV3: &extAuthzServerV3{
			engine:                       engine,
			store:                        st,
			authenticator:                authenticator,
			selector:                     selector,
			alwaysActiveAdapterSelection: alwaysActiveAdapterSelection,
		},
	}
}
