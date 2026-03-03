package opa

import (
	"context"
	"testing"

	v1alpha1 "github.com/llm-d-incubation/secure-inference/api/v1alpha1"
	"github.com/llm-d-incubation/secure-inference/pkg/config"
)

func newTestEngine(t *testing.T) *Engine {
	t.Helper()
	engine, err := New(context.Background(), config.ComponentConfig{Type: "opa"})
	if err != nil {
		t.Fatalf("Failed to create OPA engine: %v", err)
	}
	return engine
}

func TestEngine_CheckAccess(t *testing.T) {
	engine := newTestEngine(t)
	ctx := context.Background()

	user := &v1alpha1.UserSpec{
		Id:         "alice",
		Attributes: map[string]string{"role": "systems_role"},
	}
	model := &v1alpha1.ModelSpec{
		Id:   "test-model",
		Type: v1alpha1.ModelTypeBase,
		AccessPolicy: v1alpha1.ModelAccessPolicy{
			UserAttributes: map[string][]string{
				"role": {"systems_role"},
			},
		},
	}

	allowed, err := engine.CheckAccess(ctx, user, model)
	if err != nil {
		t.Fatalf("CheckAccess failed: %v", err)
	}
	if !allowed {
		t.Error("Expected access to be allowed")
	}
}

func TestEngine_CheckAccess_Denied(t *testing.T) {
	engine := newTestEngine(t)
	ctx := context.Background()

	user := &v1alpha1.UserSpec{
		Id:         "bob",
		Attributes: map[string]string{"role": "guest"},
	}
	model := &v1alpha1.ModelSpec{
		Id:   "test-model",
		Type: v1alpha1.ModelTypeBase,
		AccessPolicy: v1alpha1.ModelAccessPolicy{
			UserAttributes: map[string][]string{
				"role": {"admin"},
			},
		},
	}

	allowed, err := engine.CheckAccess(ctx, user, model)
	if err != nil {
		t.Fatalf("CheckAccess failed: %v", err)
	}
	if allowed {
		t.Error("Expected access to be denied")
	}
}

func TestEngine_GetAllowedModels(t *testing.T) {
	engine := newTestEngine(t)
	ctx := context.Background()

	user := &v1alpha1.UserSpec{
		Id:         "alice",
		Attributes: map[string]string{"role": "systems_role"},
	}

	models := []*v1alpha1.ModelSpec{
		{
			Id:   "model1",
			Type: v1alpha1.ModelTypeBase,
			AccessPolicy: v1alpha1.ModelAccessPolicy{
				UserAttributes: map[string][]string{
					"role": {"systems_role"},
				},
			},
		},
		{
			Id:   "model2",
			Type: v1alpha1.ModelTypeLora,
			AccessPolicy: v1alpha1.ModelAccessPolicy{
				UserAttributes: map[string][]string{
					"role": {"admin"},
				},
			},
		},
	}

	allowed, err := engine.GetAllowedModels(ctx, user, models)
	if err != nil {
		t.Fatalf("GetAllowedModels failed: %v", err)
	}
	if len(allowed) != 1 {
		t.Errorf("Expected 1 allowed model, got %d", len(allowed))
	}
	if len(allowed) > 0 && allowed[0].Id != "model1" {
		t.Errorf("Expected model1, got %s", allowed[0].Id)
	}
}

func TestEngine_GetAllowedModels_Multiple(t *testing.T) {
	engine := newTestEngine(t)
	ctx := context.Background()

	user := &v1alpha1.UserSpec{
		Id:         "alice",
		Attributes: map[string]string{"role": "systems_role"},
	}

	models := []*v1alpha1.ModelSpec{
		{
			Id:   "model1",
			Type: v1alpha1.ModelTypeBase,
			AccessPolicy: v1alpha1.ModelAccessPolicy{
				UserAttributes: map[string][]string{
					"role": {"systems_role"},
				},
			},
		},
		{
			Id:   "model2",
			Type: v1alpha1.ModelTypeLora,
			AccessPolicy: v1alpha1.ModelAccessPolicy{
				UserAttributes: map[string][]string{
					"role": {"systems_role"},
				},
			},
		},
	}

	allowed, err := engine.GetAllowedModels(ctx, user, models)
	if err != nil {
		t.Fatalf("GetAllowedModels failed: %v", err)
	}
	if len(allowed) != 2 {
		t.Errorf("Expected 2 allowed models, got %d", len(allowed))
	}
}
