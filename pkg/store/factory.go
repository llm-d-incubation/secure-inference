package store

import (
	"context"
	"fmt"

	"github.com/llm-d-incubation/secure-inference/pkg/config"
	"github.com/llm-d-incubation/secure-inference/pkg/store/memory"
)

const (
	// StoreTypeMemory uses in-memory storage.
	StoreTypeMemory = "memory"
)

// NewStoreFromConfig creates a new Store from a ComponentConfig.
func NewStoreFromConfig(ctx context.Context, cfg config.ComponentConfig) (Store, error) {
	switch cfg.Type {
	case StoreTypeMemory, "":
		return memory.New(), nil
	default:
		return nil, fmt.Errorf("unknown store type: %s (valid options: memory)", cfg.Type)
	}
}
