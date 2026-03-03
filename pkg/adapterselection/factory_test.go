package adapterselection

import (
	"context"
	"strings"
	"testing"

	"github.com/llm-d-incubation/secure-inference/pkg/config"
)

func TestNewAdapterSelectionFromConfig_Empty(t *testing.T) {
	cfg := config.AdapterSelectionConfig{
		ComponentConfig: config.ComponentConfig{Type: ""},
	}
	sel, err := NewAdapterSelectionFromConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sel != nil {
		t.Error("expected nil selector for empty type")
	}
}

func TestNewAdapterSelectionFromConfig_Semantic(t *testing.T) {
	cfg := config.AdapterSelectionConfig{
		ComponentConfig: config.ComponentConfig{
			Type: "semantic",
			Parameters: map[string]string{
				"url": "http://localhost:8000",
			},
		},
	}
	sel, err := NewAdapterSelectionFromConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sel == nil {
		t.Fatal("expected non-nil selector")
	}
}

func TestNewAdapterSelectionFromConfig_SemanticWithThreshold(t *testing.T) {
	cfg := config.AdapterSelectionConfig{
		ComponentConfig: config.ComponentConfig{
			Type: "semantic",
			Parameters: map[string]string{
				"url":                 "http://localhost:8000",
				"similarityThreshold": "0.9",
			},
		},
	}
	sel, err := NewAdapterSelectionFromConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sel == nil {
		t.Fatal("expected non-nil selector")
	}
}

func TestNewAdapterSelectionFromConfig_UnknownType(t *testing.T) {
	cfg := config.AdapterSelectionConfig{
		ComponentConfig: config.ComponentConfig{Type: "random"},
	}
	_, err := NewAdapterSelectionFromConfig(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
	if !strings.Contains(err.Error(), "unknown adapter selection type") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNewAdapterSelectionFromConfig_MissingURL(t *testing.T) {
	cfg := config.AdapterSelectionConfig{
		ComponentConfig: config.ComponentConfig{
			Type:       "semantic",
			Parameters: map[string]string{},
		},
	}
	_, err := NewAdapterSelectionFromConfig(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error for missing url")
	}
	if !strings.Contains(err.Error(), "url") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNewAdapterSelectionFromConfig_InvalidThreshold(t *testing.T) {
	cfg := config.AdapterSelectionConfig{
		ComponentConfig: config.ComponentConfig{
			Type: "semantic",
			Parameters: map[string]string{
				"url":                 "http://localhost:8000",
				"similarityThreshold": "not-a-number",
			},
		},
	}
	_, err := NewAdapterSelectionFromConfig(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error for invalid threshold")
	}
	if !strings.Contains(err.Error(), "similarityThreshold") {
		t.Errorf("unexpected error: %v", err)
	}
}
