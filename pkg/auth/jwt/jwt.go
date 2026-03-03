package jwt

import (
	"context"
	"crypto/rsa"
	"fmt"
	"strings"

	"github.com/llm-d-incubation/secure-inference/pkg/config"
	"github.com/llm-d-incubation/secure-inference/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var logger = logf.Log.WithName("jwt")

const bearer = "Bearer"

// Authenticator validates requests using JWT tokens.
type Authenticator struct {
	publicKey *rsa.PublicKey
}

// New creates a JWT Authenticator from a ComponentConfig.
// Required parameter: "publicKeyPath".
func New(ctx context.Context, cfg config.ComponentConfig) (*Authenticator, error) {
	keyPath := cfg.Parameters["publicKeyPath"]
	if keyPath == "" {
		return nil, fmt.Errorf("jwt authenticator: 'publicKeyPath' parameter is required")
	}

	pubKey, err := LoadPublicKey(keyPath)
	if err != nil {
		return nil, fmt.Errorf("jwt authenticator: failed to load public key: %w", err)
	}

	return &Authenticator{publicKey: pubKey}, nil
}

// Authenticate extracts and validates a Bearer JWT from the request's authorization header.
func (a *Authenticator) Authenticate(ctx context.Context, req *types.InferenceRequest) (*types.AuthResult, error) {
	authString, ok := req.Headers["authorization"]
	if !ok {
		return nil, fmt.Errorf("missing authorization header")
	}

	authFields := strings.Fields(authString)
	if len(authFields) != 2 || authFields[0] != bearer {
		return nil, fmt.Errorf("invalid authorization header format, expected 'Bearer <token>'")
	}

	token := authFields[1]
	logger.V(1).Info("Validating JWT token")

	claims, err := ValidateJWTWithKey(token, a.publicKey)
	if err != nil {
		return nil, fmt.Errorf("invalid JWT token: %w", err)
	}

	logger.V(2).Info("JWT claims validated", "username", claims.Username, "role", claims.Role, "expiresAt", claims.ExpiresAt)

	return &types.AuthResult{UserID: claims.Username}, nil
}
