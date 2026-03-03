package policyengine

import (
	"context"

	v1alpha1 "github.com/llm-d-incubation/secure-inference/api/v1alpha1"
)

// PolicyEngine defines the interface for policy evaluation.
// Implementations perform pure access control decisions — no store dependency.
// Callers provide struct pointers directly.
type PolicyEngine interface {
	// CheckAccess evaluates whether a user is allowed to access a model.
	CheckAccess(ctx context.Context, user *v1alpha1.UserSpec, model *v1alpha1.ModelSpec) (bool, error)

	// GetAllowedModels filters a list of models to those the user is allowed to access.
	GetAllowedModels(ctx context.Context, user *v1alpha1.UserSpec, models []*v1alpha1.ModelSpec) ([]*v1alpha1.ModelSpec, error)
}
