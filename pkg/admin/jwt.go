package admin

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	authjwt "github.com/llm-d-incubation/secure-inference/pkg/auth/jwt"
	"github.com/llm-d-incubation/secure-inference/pkg/config"
)

// LoadPrivateKey loads an RSA private key from a PEM file.
func LoadPrivateKey(filePath string) (*rsa.PrivateKey, error) {
	keyData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		return nil, fmt.Errorf("invalid PEM block or not an RSA private key")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse RSA private key: %w", err)
	}

	return privateKey, nil
}

// GenerateJWT creates a JWT token signed with an RSA private key.
func GenerateJWT(username, role, organization, privateKeyFile string) (string, error) {
	privateKey, err := LoadPrivateKey(privateKeyFile)
	if err != nil {
		return "", err
	}

	claims := authjwt.UserClaims{
		Username:     username,
		Role:         role,
		Organization: organization,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    config.ServerName,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)), // Token expires in 24 hours
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   username,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}
