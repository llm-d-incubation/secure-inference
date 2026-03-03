package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/llm-d-incubation/secure-inference/pkg/config"
)

func TestNewAuthenticatorFromConfig_JWT(t *testing.T) {
	keyPath := writeTempPublicKey(t)
	cfg := config.ComponentConfig{
		Type: "jwt",
		Parameters: map[string]string{
			"publicKeyPath": keyPath,
		},
	}

	auth, err := NewAuthenticatorFromConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth == nil {
		t.Fatal("expected non-nil authenticator")
	}
}

func TestNewAuthenticatorFromConfig_EmptyTypeDefaultsToJWT(t *testing.T) {
	keyPath := writeTempPublicKey(t)
	cfg := config.ComponentConfig{
		Type: "",
		Parameters: map[string]string{
			"publicKeyPath": keyPath,
		},
	}

	auth, err := NewAuthenticatorFromConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth == nil {
		t.Fatal("expected non-nil authenticator")
	}
}

func TestNewAuthenticatorFromConfig_UnknownType(t *testing.T) {
	cfg := config.ComponentConfig{Type: "oauth2"}
	_, err := NewAuthenticatorFromConfig(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
	if !strings.Contains(err.Error(), "unknown authenticator type") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNewAuthenticatorFromConfig_MissingKeyPath(t *testing.T) {
	cfg := config.ComponentConfig{
		Type:       "jwt",
		Parameters: map[string]string{},
	}
	_, err := NewAuthenticatorFromConfig(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error for missing publicKeyPath")
	}
	if !strings.Contains(err.Error(), "publicKeyPath") {
		t.Errorf("unexpected error: %v", err)
	}
}

// writeTempPublicKey generates an RSA key pair and writes the public key as PEM to a temp file.
func writeTempPublicKey(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	pubBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("failed to marshal public key: %v", err)
	}
	pemBlock := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	})

	dir := t.TempDir()
	path := filepath.Join(dir, "pub.pem")
	if err := os.WriteFile(path, pemBlock, 0o644); err != nil {
		t.Fatalf("failed to write temp key: %v", err)
	}
	return path
}
