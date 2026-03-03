package opa

import (
	"context"
	"fmt"

	v1alpha1 "github.com/llm-d-incubation/secure-inference/api/v1alpha1"
	"github.com/llm-d-incubation/secure-inference/pkg/config"
	"github.com/llm-d-incubation/secure-inference/pkg/policyengine/opa/policies"
)

// Engine implements PolicyEngine using OPA for policy evaluation.
// It holds only the OPA engine — no store. Callers provide data directly.
type Engine struct {
	opa *policies.OPAEngine
}

// New creates a new OPA-backed PolicyEngine from a ComponentConfig.
func New(ctx context.Context, cfg config.ComponentConfig) (*Engine, error) {
	opaEngine, err := policies.NewOPAEngine(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create OPA engine: %w", err)
	}
	return &Engine{opa: opaEngine}, nil
}

// CheckAccess evaluates whether a user is allowed to access a model.
func (e *Engine) CheckAccess(ctx context.Context, user *v1alpha1.UserSpec, model *v1alpha1.ModelSpec) (bool, error) {
	allowed, err := e.opa.CheckAccess(ctx, user, model)
	if err != nil {
		return false, fmt.Errorf("policy evaluation failed: %w", err)
	}
	return allowed, nil
}

// GetAllowedModels filters the provided models to those the user is allowed to access.
func (e *Engine) GetAllowedModels(
	ctx context.Context, user *v1alpha1.UserSpec, models []*v1alpha1.ModelSpec,
) ([]*v1alpha1.ModelSpec, error) {
	// Convert []*ModelSpec → []ModelSpec for OPA engine
	flat := make([]v1alpha1.ModelSpec, 0, len(models))
	for _, m := range models {
		flat = append(flat, *m)
	}

	allowedModelIDs, err := e.opa.GetAllowedModels(ctx, user, flat)
	if err != nil {
		return nil, fmt.Errorf("policy evaluation failed: %w", err)
	}

	// Build result from allowed IDs
	allowedSet := make(map[string]struct{}, len(allowedModelIDs))
	for _, id := range allowedModelIDs {
		allowedSet[id] = struct{}{}
	}

	result := make([]*v1alpha1.ModelSpec, 0, len(allowedModelIDs))
	for _, model := range models {
		if _, ok := allowedSet[model.Id]; ok {
			result = append(result, model)
		}
	}

	return result, nil
}
