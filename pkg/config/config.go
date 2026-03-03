package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"sigs.k8s.io/yaml"
)

// SecureInferenceConfig is the top-level configuration for the secure-inference server.
type SecureInferenceConfig struct {
	APIVersion       string                 `json:"apiVersion"`
	Kind             string                 `json:"kind"`
	DataStore        ComponentConfig        `json:"dataStore"`
	PolicyEngine     ComponentConfig        `json:"policyEngine"`
	AdapterSelection AdapterSelectionConfig `json:"adapterSelection"`
	Auth             ComponentConfig        `json:"auth"`
}

// ComponentConfig is a generic configuration block for a pluggable component.
// Type selects the strategy; Parameters holds strategy-specific key-value pairs.
type ComponentConfig struct {
	Type       string            `json:"type"`
	Parameters map[string]string `json:"parameters,omitempty"`
}

// AdapterSelectionConfig extends ComponentConfig with adapter-selection-specific fields.
type AdapterSelectionConfig struct {
	ComponentConfig `json:",inline"`
	// AlwaysActive controls whether adapter selection runs for every request (true)
	// or only when the x-adapter-selection: true header is present (false).
	AlwaysActive bool `json:"alwaysActive,omitempty"`
}

// DefaultConfig returns a SecureInferenceConfig with sensible defaults:
// memory store, OPA policy engine, adapter selection disabled.
func DefaultConfig() *SecureInferenceConfig {
	return &SecureInferenceConfig{
		APIVersion: "secure-inference.llm-d.io/v1alpha1",
		Kind:       "SecureInferenceConfig",
		DataStore: ComponentConfig{
			Type: "memory",
		},
		PolicyEngine: ComponentConfig{
			Type: "opa",
		},
		AdapterSelection: AdapterSelectionConfig{
			ComponentConfig: ComponentConfig{Type: ""},
			AlwaysActive:    false,
		},
		Auth: ComponentConfig{
			Type: "jwt",
			Parameters: map[string]string{
				"publicKeyPath": "/etc/ssl/certs/llm-d-ca.pem",
			},
		},
	}
}

// Validate checks that all component types are recognized and that required
// parameters are present for each enabled component.
func (c *SecureInferenceConfig) Validate() error {
	var errs []string

	switch c.DataStore.Type {
	case "memory", "":
	default:
		errs = append(errs, fmt.Sprintf("unknown dataStore type: %q", c.DataStore.Type))
	}

	switch c.PolicyEngine.Type {
	case "opa", "":
	default:
		errs = append(errs, fmt.Sprintf("unknown policyEngine type: %q", c.PolicyEngine.Type))
	}

	switch c.Auth.Type {
	case "jwt", "":
		if c.Auth.Type == "jwt" && c.Auth.Parameters["publicKeyPath"] == "" {
			errs = append(errs, "auth type \"jwt\" requires parameter \"publicKeyPath\"")
		}
	default:
		errs = append(errs, fmt.Sprintf("unknown auth type: %q", c.Auth.Type))
	}

	switch c.AdapterSelection.Type {
	case "semantic":
		if c.AdapterSelection.Parameters["url"] == "" {
			errs = append(errs, "adapterSelection type \"semantic\" requires parameter \"url\"")
		}
		if v := c.AdapterSelection.Parameters["similarityThreshold"]; v != "" {
			if _, err := strconv.ParseFloat(v, 64); err != nil {
				errs = append(errs, fmt.Sprintf("adapterSelection parameter \"similarityThreshold\" must be a number: %q", v))
			}
		}
	case "":
	default:
		errs = append(errs, fmt.Sprintf("unknown adapterSelection type: %q", c.AdapterSelection.Type))
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation failed:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
}

// LoadConfig reads a YAML config file and returns a SecureInferenceConfig.
// Missing fields are filled with defaults.
func LoadConfig(path string) (*SecureInferenceConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	return cfg, nil
}
