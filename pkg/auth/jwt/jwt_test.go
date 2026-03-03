package jwt

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"

	"github.com/llm-d-incubation/secure-inference/pkg/config"
)

// testKeyPair holds RSA keys generated for tests.
type testKeyPair struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
}

func generateTestKeyPair(t *testing.T) *testKeyPair {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}
	return &testKeyPair{privateKey: key, publicKey: &key.PublicKey}
}

func signToken(t *testing.T, kp *testKeyPair, claims *UserClaims) string {
	t.Helper()
	token := gojwt.NewWithClaims(gojwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(kp.privateKey)
	if err != nil {
		t.Fatalf("Failed to sign token: %v", err)
	}
	return signed
}

func newTestClaims(username, role, org string) *UserClaims {
	return &UserClaims{
		Username:     username,
		Role:         role,
		Organization: org,
		RegisteredClaims: gojwt.RegisteredClaims{
			Issuer:    config.ServerName,
			ExpiresAt: gojwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  gojwt.NewNumericDate(time.Now()),
		},
	}
}

// --- ValidateJWTWithKey Tests ---

func TestValidateJWTWithKey_ValidToken(t *testing.T) {
	kp := generateTestKeyPair(t)
	claims := newTestClaims("alice", "admin", "acme")
	tokenStr := signToken(t, kp, claims)

	got, err := ValidateJWTWithKey(tokenStr, kp.publicKey)
	if err != nil {
		t.Fatalf("Expected valid token, got error: %v", err)
	}
	if got.Username != "alice" {
		t.Errorf("Expected username alice, got %s", got.Username)
	}
	if got.Role != "admin" {
		t.Errorf("Expected role admin, got %s", got.Role)
	}
	if got.Organization != "acme" {
		t.Errorf("Expected organization acme, got %s", got.Organization)
	}
}

func TestValidateJWTWithKey_ExpiredToken(t *testing.T) {
	kp := generateTestKeyPair(t)
	claims := &UserClaims{
		Username: "alice",
		RegisteredClaims: gojwt.RegisteredClaims{
			Issuer:    config.ServerName,
			ExpiresAt: gojwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  gojwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}
	tokenStr := signToken(t, kp, claims)

	_, err := ValidateJWTWithKey(tokenStr, kp.publicKey)
	if err == nil {
		t.Fatal("Expected error for expired token")
	}
}

func TestValidateJWTWithKey_WrongKey(t *testing.T) {
	kp1 := generateTestKeyPair(t)
	kp2 := generateTestKeyPair(t)
	claims := newTestClaims("alice", "admin", "acme")
	tokenStr := signToken(t, kp1, claims)

	_, err := ValidateJWTWithKey(tokenStr, kp2.publicKey)
	if err == nil {
		t.Fatal("Expected error when validating with wrong public key")
	}
}

func TestValidateJWTWithKey_InvalidIssuer(t *testing.T) {
	kp := generateTestKeyPair(t)
	claims := &UserClaims{
		Username: "alice",
		RegisteredClaims: gojwt.RegisteredClaims{
			Issuer:    "wrong-issuer",
			ExpiresAt: gojwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	}
	tokenStr := signToken(t, kp, claims)

	_, err := ValidateJWTWithKey(tokenStr, kp.publicKey)
	if err == nil {
		t.Fatal("Expected error for invalid issuer")
	}
}

func TestValidateJWTWithKey_MalformedToken(t *testing.T) {
	kp := generateTestKeyPair(t)

	_, err := ValidateJWTWithKey("not.a.valid.token", kp.publicKey)
	if err == nil {
		t.Fatal("Expected error for malformed token")
	}
}

// --- LoadPublicKey Tests ---

func TestLoadPublicKey_Certificate(t *testing.T) {
	kp := generateTestKeyPair(t)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(1 * time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, kp.publicKey, kp.privateKey)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "cert.pem")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		t.Fatalf("Failed to write cert file: %v", err)
	}

	pubKey, err := LoadPublicKey(certPath)
	if err != nil {
		t.Fatalf("LoadPublicKey failed: %v", err)
	}
	if pubKey.N.Cmp(kp.publicKey.N) != 0 {
		t.Error("Loaded public key does not match original")
	}
}

func TestLoadPublicKey_PublicKeyPEM(t *testing.T) {
	kp := generateTestKeyPair(t)

	pubDER, err := x509.MarshalPKIXPublicKey(kp.publicKey)
	if err != nil {
		t.Fatalf("Failed to marshal public key: %v", err)
	}

	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "pubkey.pem")
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})
	if err := os.WriteFile(keyPath, keyPEM, 0o644); err != nil {
		t.Fatalf("Failed to write key file: %v", err)
	}

	pubKey, err := LoadPublicKey(keyPath)
	if err != nil {
		t.Fatalf("LoadPublicKey failed: %v", err)
	}
	if pubKey.N.Cmp(kp.publicKey.N) != 0 {
		t.Error("Loaded public key does not match original")
	}
}

func TestLoadPublicKey_FileNotFound(t *testing.T) {
	_, err := LoadPublicKey("/nonexistent/path.pem")
	if err == nil {
		t.Fatal("Expected error for missing file")
	}
}

func TestLoadPublicKey_InvalidPEM(t *testing.T) {
	tmpDir := t.TempDir()
	badPath := filepath.Join(tmpDir, "bad.pem")
	if err := os.WriteFile(badPath, []byte("not pem data"), 0o644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	_, err := LoadPublicKey(badPath)
	if err == nil {
		t.Fatal("Expected error for invalid PEM")
	}
}
