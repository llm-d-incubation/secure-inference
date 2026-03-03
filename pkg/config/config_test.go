package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.DataStore.Type != "memory" {
		t.Errorf("expected dataStore.type=memory, got %q", cfg.DataStore.Type)
	}
	if cfg.PolicyEngine.Type != "opa" {
		t.Errorf("expected policyEngine.type=opa, got %q", cfg.PolicyEngine.Type)
	}
	if cfg.AdapterSelection.Type != "" {
		t.Errorf("expected adapterSelection.type=\"\", got %q", cfg.AdapterSelection.Type)
	}
	if cfg.AdapterSelection.AlwaysActive {
		t.Error("expected adapterSelection.alwaysActive=false by default")
	}
	if cfg.Auth.Type != "jwt" {
		t.Errorf("expected auth.type=jwt, got %q", cfg.Auth.Type)
	}
	if cfg.Auth.Parameters["publicKeyPath"] != "/etc/ssl/certs/llm-d-ca.pem" {
		t.Errorf("expected auth publicKeyPath default, got %q", cfg.Auth.Parameters["publicKeyPath"])
	}
}

func TestDefaultConfig_Validates(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Errorf("default config should be valid: %v", err)
	}
}

func TestLoadConfig_Full(t *testing.T) {
	content := `
apiVersion: secure-inference.llm-d.io/v1alpha1
kind: SecureInferenceConfig
dataStore:
  type: memory
policyEngine:
  type: opa
auth:
  type: jwt
  parameters:
    publicKeyPath: /custom/path/key.pem
adapterSelection:
  type: semantic
  alwaysActive: true
  parameters:
    url: "http://localhost:8000"
    similarityThreshold: "0.8"
`
	path := writeTemp(t, content)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.DataStore.Type != "memory" {
		t.Errorf("expected dataStore.type=memory, got %q", cfg.DataStore.Type)
	}
	if cfg.AdapterSelection.Type != "semantic" {
		t.Errorf("expected adapterSelection.type=semantic, got %q", cfg.AdapterSelection.Type)
	}
	if cfg.AdapterSelection.Parameters["url"] != "http://localhost:8000" {
		t.Errorf("expected url=http://localhost:8000, got %q", cfg.AdapterSelection.Parameters["url"])
	}
	if cfg.AdapterSelection.Parameters["similarityThreshold"] != "0.8" {
		t.Errorf("expected similarityThreshold=0.8, got %q", cfg.AdapterSelection.Parameters["similarityThreshold"])
	}
	if !cfg.AdapterSelection.AlwaysActive {
		t.Error("expected adapterSelection.alwaysActive=true")
	}
	if cfg.Auth.Type != "jwt" {
		t.Errorf("expected auth.type=jwt, got %q", cfg.Auth.Type)
	}
	if cfg.Auth.Parameters["publicKeyPath"] != "/custom/path/key.pem" {
		t.Errorf("expected custom publicKeyPath, got %q", cfg.Auth.Parameters["publicKeyPath"])
	}
}

func TestLoadConfig_DefaultsFillMissing(t *testing.T) {
	content := `
apiVersion: secure-inference.llm-d.io/v1alpha1
kind: SecureInferenceConfig
`
	path := writeTemp(t, content)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	// Defaults should still be applied since we unmarshal into DefaultConfig()
	if cfg.DataStore.Type != "memory" {
		t.Errorf("expected dataStore.type=memory, got %q", cfg.DataStore.Type)
	}
	if cfg.PolicyEngine.Type != "opa" {
		t.Errorf("expected policyEngine.type=opa, got %q", cfg.PolicyEngine.Type)
	}
	if cfg.Auth.Type != "jwt" {
		t.Errorf("expected auth.type=jwt, got %q", cfg.Auth.Type)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	path := writeTemp(t, "{{invalid yaml")
	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestValidate_UnknownStoreType(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DataStore.Type = "redis"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for unknown store type")
	}
	if !strings.Contains(err.Error(), "unknown dataStore type") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_UnknownPolicyEngineType(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PolicyEngine.Type = "casbin"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for unknown policyEngine type")
	}
	if !strings.Contains(err.Error(), "unknown policyEngine type") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_UnknownAuthType(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Auth.Type = "oauth2"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for unknown auth type")
	}
	if !strings.Contains(err.Error(), "unknown auth type") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_JWTMissingPublicKeyPath(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Auth.Parameters = map[string]string{}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for missing publicKeyPath")
	}
	if !strings.Contains(err.Error(), "publicKeyPath") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_UnknownAdapterSelectionType(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AdapterSelection.Type = "random"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for unknown adapterSelection type")
	}
	if !strings.Contains(err.Error(), "unknown adapterSelection type") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_SemanticMissingURL(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AdapterSelection.Type = "semantic"
	cfg.AdapterSelection.Parameters = map[string]string{}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for missing url")
	}
	if !strings.Contains(err.Error(), "url") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_SemanticInvalidThreshold(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AdapterSelection.Type = "semantic"
	cfg.AdapterSelection.Parameters = map[string]string{
		"url":                 "http://localhost:8000",
		"similarityThreshold": "not-a-number",
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for invalid threshold")
	}
	if !strings.Contains(err.Error(), "similarityThreshold") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_SemanticValidConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AdapterSelection.Type = "semantic"
	cfg.AdapterSelection.Parameters = map[string]string{
		"url":                 "http://localhost:8000",
		"similarityThreshold": "0.7",
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("valid semantic config should pass: %v", err)
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	cfg := &SecureInferenceConfig{
		DataStore:    ComponentConfig{Type: "redis"},
		PolicyEngine: ComponentConfig{Type: "casbin"},
		Auth:         ComponentConfig{Type: "oauth2"},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation errors")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "dataStore") || !strings.Contains(errMsg, "policyEngine") || !strings.Contains(errMsg, "auth") {
		t.Errorf("expected all three errors, got: %v", err)
	}
}

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
