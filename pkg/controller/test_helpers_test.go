package controller

import (
	"github.com/llm-d-incubation/secure-inference/pkg/store/memory"
)

// newTestStore creates an in-memory store for tests.
func newTestStore() *memory.Store {
	return memory.New()
}
