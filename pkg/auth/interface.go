package auth

import (
	"context"

	"github.com/llm-d-incubation/secure-inference/pkg/types"
)

// Authenticator validates a request and identifies the user.
type Authenticator interface {
	Authenticate(ctx context.Context, req *types.InferenceRequest) (*types.AuthResult, error)
}
