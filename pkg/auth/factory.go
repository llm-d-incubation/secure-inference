package auth

import (
	"context"
	"fmt"

	"github.com/llm-d-incubation/secure-inference/pkg/auth/jwt"
	"github.com/llm-d-incubation/secure-inference/pkg/config"
)

// NewAuthenticatorFromConfig creates an Authenticator from a ComponentConfig.
func NewAuthenticatorFromConfig(ctx context.Context, cfg config.ComponentConfig) (Authenticator, error) {
	switch cfg.Type {
	case "jwt", "":
		return jwt.New(ctx, cfg)
	default:
		return nil, fmt.Errorf("unknown authenticator type: %s", cfg.Type)
	}
}
