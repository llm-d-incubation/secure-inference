package mock

import (
	"context"

	v1alpha1 "github.com/llm-d-incubation/secure-inference/api/v1alpha1"
	"github.com/llm-d-incubation/secure-inference/pkg/types"
)

// Selector is a mock adapter selector for testing.
type Selector struct {
	Model   *v1alpha1.ModelSpec
	Matched bool
}

// Pick returns the configured mock result.
func (s *Selector) Pick(_ context.Context, _ *types.InferenceRequest, _ []*v1alpha1.ModelSpec) (*v1alpha1.ModelSpec, bool) {
	return s.Model, s.Matched
}
