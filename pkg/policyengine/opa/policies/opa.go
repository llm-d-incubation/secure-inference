package policies

import (
	"context"
	"fmt"

	v1alpha1 "github.com/llm-d-incubation/secure-inference/api/v1alpha1"
	"github.com/open-policy-agent/opa/v1/rego"
)

// OPAEngine wraps the Open Policy Agent for policy evaluation.
type OPAEngine struct {
	checkAccessQuery   rego.PreparedEvalQuery
	allowedModelsQuery rego.PreparedEvalQuery
}

// NewOPAEngine creates a new OPA engine with embedded policies.
func NewOPAEngine(ctx context.Context) (*OPAEngine, error) {
	// Prepare query for checking access (is_user_allowed)
	checkAccessQuery, err := rego.New(
		rego.Query("data.auth.is_user_allowed"),
		rego.Module("access.rego", AccessPolicy),
	).PrepareForEval(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare check access query: %w", err)
	}

	// Prepare query for getting allowed models (allowed_models)
	allowedModelsQuery, err := rego.New(
		rego.Query("data.auth.allowed_models"),
		rego.Module("access.rego", AccessPolicy),
	).PrepareForEval(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare allowed models query: %w", err)
	}

	return &OPAEngine{
		checkAccessQuery:   checkAccessQuery,
		allowedModelsQuery: allowedModelsQuery,
	}, nil
}

// CheckAccess evaluates if a user can access a model using OPA.
// Returns true if access is allowed, false otherwise.
func (e *OPAEngine) CheckAccess(ctx context.Context, user *v1alpha1.UserSpec, model *v1alpha1.ModelSpec) (bool, error) {
	input := map[string]interface{}{
		"user_obj":  user,
		"model_obj": model,
	}

	results, err := e.checkAccessQuery.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		return false, fmt.Errorf("failed to evaluate policy: %w", err)
	}

	if len(results) == 0 {
		return false, fmt.Errorf("no results from policy evaluation")
	}

	// Extract boolean result
	allowed, ok := results[0].Expressions[0].Value.(bool)
	if !ok {
		return false, fmt.Errorf("unexpected result type: %T", results[0].Expressions[0].Value)
	}

	return allowed, nil
}

// GetAllowedModels returns the list of model IDs that a user can access.
func (e *OPAEngine) GetAllowedModels(
	ctx context.Context, user *v1alpha1.UserSpec, models []v1alpha1.ModelSpec,
) ([]string, error) {
	input := map[string]interface{}{
		"user_obj":   user,
		"model_objs": models,
	}

	results, err := e.allowedModelsQuery.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate policy: %w", err)
	}

	if len(results) == 0 {
		return []string{}, nil
	}

	// Extract set of allowed model IDs
	// OPA can return sets as either map[string]interface{} or []interface{}
	value := results[0].Expressions[0].Value
	var allowed []string

	// OPA represents sets differently depending on their internal state:
	// as map[string]interface{} (object keys) or []interface{} (array).
	// Both cases must be handled.
	switch v := value.(type) {
	case map[string]interface{}:
		// OPA set represented as object
		allowed = make([]string, 0, len(v))
		for modelID := range v {
			allowed = append(allowed, modelID)
		}
	case []interface{}:
		// OPA set represented as array
		allowed = make([]string, 0, len(v))
		for _, item := range v {
			if modelID, ok := item.(string); ok {
				allowed = append(allowed, modelID)
			}
		}
	default:
		return nil, fmt.Errorf("unexpected result type: %T", value)
	}

	return allowed, nil
}
