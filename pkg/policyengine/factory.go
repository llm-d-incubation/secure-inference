package policyengine

import (
	"context"
	"fmt"

	"github.com/llm-d-incubation/secure-inference/pkg/config"
	"github.com/llm-d-incubation/secure-inference/pkg/policyengine/opa"
)

// NewPolicyEngineFromConfig creates a PolicyEngine from a ComponentConfig.
func NewPolicyEngineFromConfig(ctx context.Context, cfg config.ComponentConfig) (PolicyEngine, error) {
	switch cfg.Type {
	case "opa", "":
		return opa.New(ctx, cfg)
	default:
		return nil, fmt.Errorf("unknown policy engine type: %s", cfg.Type)
	}
}
