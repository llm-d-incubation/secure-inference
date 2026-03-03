package admin

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestKeys(t *testing.T) (string, func()) { //nolint:gocritic // unnamedResult is fine for test helpers
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "jwt-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")

	// Create certificate config
	config := &CertificateConfig{
		Name:              "test-cert",
		IsCA:              true,
		CertOutPath:       certPath,
		PrivateKeyOutPath: keyPath,
	}

	// Generate test certificates
	err = CreateCertificate(config)
	if err != nil {
		t.Fatalf("Failed to generate certificates: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return keyPath, cleanup
}

func TestLoadPrivateKey(t *testing.T) {
	keyPath, cleanup := setupTestKeys(t)
	defer cleanup()

	privateKey, err := LoadPrivateKey(keyPath)
	if err != nil {
		t.Fatalf("LoadPrivateKey failed: %v", err)
	}

	if privateKey == nil {
		t.Error("Loaded private key is nil")
	}
}

func TestLoadPrivateKey_InvalidPath(t *testing.T) {
	_, err := LoadPrivateKey("/nonexistent/key.pem")
	if err == nil {
		t.Error("Expected error for nonexistent key path")
	}
}

func TestGenerateJWT(t *testing.T) {
	keyPath, cleanup := setupTestKeys(t)
	defer cleanup()

	token, err := GenerateJWT("alice", "systems_role", "test-org", keyPath)
	if err != nil {
		t.Fatalf("GenerateJWT failed: %v", err)
	}

	if token == "" {
		t.Error("Generated token is empty")
	}

	// JWT should have 3 parts (header.payload.signature)
	dotCount := 0
	for _, c := range token {
		if c == '.' {
			dotCount++
		}
	}
	if dotCount != 2 {
		t.Errorf("Expected JWT with 3 parts, got %d dots", dotCount)
	}
}

func TestGenerateJWT_DifferentUsers(t *testing.T) {
	keyPath, cleanup := setupTestKeys(t)
	defer cleanup()

	token1, err := GenerateJWT("alice", "admin", "org1", keyPath)
	if err != nil {
		t.Fatalf("GenerateJWT failed for alice: %v", err)
	}

	token2, err := GenerateJWT("bob", "user", "org2", keyPath)
	if err != nil {
		t.Fatalf("GenerateJWT failed for bob: %v", err)
	}

	if token1 == token2 {
		t.Error("Tokens for different users should be different")
	}
}

func TestGenerateJWT_InvalidKeyPath(t *testing.T) {
	_, err := GenerateJWT("alice", "admin", "org", "/nonexistent/key.pem")
	if err == nil {
		t.Error("Expected error for nonexistent key path")
	}
}

func TestGenerateJWT_EmptyUsername(t *testing.T) {
	keyPath, cleanup := setupTestKeys(t)
	defer cleanup()

	// Empty username should still generate a token
	token, err := GenerateJWT("", "role", "org", keyPath)
	if err != nil {
		t.Fatalf("GenerateJWT failed with empty username: %v", err)
	}

	if token == "" {
		t.Error("Token should still be generated with empty username")
	}
}
