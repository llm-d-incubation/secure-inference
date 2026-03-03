package runnable

import (
	"context"
	"fmt"
	"net"

	"google.golang.org/grpc"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// GRPCServer wraps a gRPC server as a controller-runtime manager.Runnable.
// Handles graceful shutdown when the manager's context is cancelled.
func GRPCServer(name string, srv *grpc.Server, port int) manager.Runnable {
	return manager.RunnableFunc(func(ctx context.Context) error {
		logger := logf.FromContext(ctx).WithName(name)

		lc := net.ListenConfig{}
		lis, err := lc.Listen(ctx, "tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			return fmt.Errorf("failed to listen on port %d: %w", port, err)
		}

		// Graceful shutdown when context is cancelled
		go func() {
			<-ctx.Done()
			logger.Info("Shutting down gRPC server")
			srv.GracefulStop()
		}()

		logger.Info("Starting gRPC server", "port", port)
		if err := srv.Serve(lis); err != nil {
			return fmt.Errorf("gRPC server %s failed: %w", name, err)
		}
		return nil
	})
}
