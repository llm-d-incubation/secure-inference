package store

import (
	"context"
	"testing"

	"github.com/llm-d-incubation/secure-inference/pkg/config"
)

func TestNewStoreFromConfig_Memory(t *testing.T) {
	ctx := context.Background()
	s, err := NewStoreFromConfig(ctx, config.ComponentConfig{Type: "memory"})
	if err != nil {
		t.Fatalf("NewStoreFromConfig failed for memory: %v", err)
	}
	if s == nil {
		t.Error("Store is nil")
	}
	if err := s.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestNewStoreFromConfig_EmptyDefaultsToMemory(t *testing.T) {
	ctx := context.Background()
	s, err := NewStoreFromConfig(ctx, config.ComponentConfig{Type: ""})
	if err != nil {
		t.Fatalf("NewStoreFromConfig failed for empty type: %v", err)
	}
	if s == nil {
		t.Error("Store is nil")
	}
	s.Close()
}

func TestNewStoreFromConfig_InvalidType(t *testing.T) {
	ctx := context.Background()
	_, err := NewStoreFromConfig(ctx, config.ComponentConfig{Type: "invalid-type"})
	if err == nil {
		t.Error("Expected error for invalid store type")
	}
}
