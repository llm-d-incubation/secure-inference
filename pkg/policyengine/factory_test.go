package policyengine_test

import (
	"context"
	"testing"

	"github.com/llm-d-incubation/secure-inference/pkg/config"
	"github.com/llm-d-incubation/secure-inference/pkg/policyengine"
	"github.com/llm-d-incubation/secure-inference/pkg/policyengine/opa"
)

func TestNewPolicyEngineFromConfig_OPA(t *testing.T) {
	ctx := context.Background()
	engine, err := policyengine.NewPolicyEngineFromConfig(ctx, config.ComponentConfig{Type: "opa"})
	if err != nil {
		t.Fatalf("NewPolicyEngineFromConfig failed for opa: %v", err)
	}
	if engine == nil {
		t.Error("Engine is nil")
	}
}

func TestNewPolicyEngineFromConfig_EmptyDefaultsToOPA(t *testing.T) {
	ctx := context.Background()
	engine, err := policyengine.NewPolicyEngineFromConfig(ctx, config.ComponentConfig{Type: ""})
	if err != nil {
		t.Fatalf("NewPolicyEngineFromConfig failed for empty type: %v", err)
	}
	if engine == nil {
		t.Error("Engine is nil")
	}
}

func TestNewPolicyEngineFromConfig_InvalidType(t *testing.T) {
	ctx := context.Background()
	_, err := policyengine.NewPolicyEngineFromConfig(ctx, config.ComponentConfig{Type: "invalid"})
	if err == nil {
		t.Error("Expected error for invalid engine type")
	}
}

func TestOPAEngine_ImplementsInterface(t *testing.T) {
	var _ policyengine.PolicyEngine = (*opa.Engine)(nil)
}
