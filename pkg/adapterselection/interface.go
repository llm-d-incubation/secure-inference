package adapterselection

import (
	"context"

	v1alpha1 "github.com/llm-d-incubation/secure-inference/api/v1alpha1"
	"github.com/llm-d-incubation/secure-inference/pkg/types"
)

// Selector picks the best LoRA adapter for a request from allowed models.
type Selector interface {
	// Pick selects a LoRA adapter from the allowed models for the given request.
	// Returns the selected model and true if found, or nil and false otherwise.
	Pick(ctx context.Context, req *types.InferenceRequest, allowedModels []*v1alpha1.ModelSpec) (*v1alpha1.ModelSpec, bool)
}
