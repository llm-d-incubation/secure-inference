package jwt

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/llm-d-incubation/secure-inference/pkg/config"
)

// LoadPublicKey loads an RSA public key from a certificate or public key PEM file.
func LoadPublicKey(filePath string) (*rsa.PublicKey, error) {
	certData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate/public key file: %w", err)
	}

	block, _ := pem.Decode(certData)
	if block == nil {
		return nil, fmt.Errorf("invalid PEM block")
	}

	if block.Type == "CERTIFICATE" {
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse certificate: %w", err)
		}
		publicKey, ok := cert.PublicKey.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("certificate does not contain an RSA public key")
		}
		return publicKey, nil
	}

	if block.Type == "PUBLIC KEY" || block.Type == "RSA PUBLIC KEY" {
		publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse public key: %w", err)
		}
		rsaPubKey, ok := publicKey.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("not an RSA public key")
		}
		return rsaPubKey, nil
	}

	return nil, fmt.Errorf("unsupported PEM block type: %s", block.Type)
}

// ValidateJWT validates a JWT token by loading the public key from file.
// Prefer ValidateJWTWithKey for repeated calls to avoid re-reading the file.
func ValidateJWT(tokenString, publicKeyFile string) (*UserClaims, error) {
	publicKey, err := LoadPublicKey(publicKeyFile)
	if err != nil {
		return nil, err
	}
	return ValidateJWTWithKey(tokenString, publicKey)
}

// ValidateJWTWithKey validates a JWT token using a pre-loaded RSA public key.
func ValidateJWTWithKey(tokenString string, publicKey *rsa.PublicKey) (*UserClaims, error) {
	token, err := gojwt.ParseWithClaims(tokenString, &UserClaims{}, func(token *gojwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*gojwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return publicKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(*UserClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims format")
	}

	if claims.Issuer != config.ServerName {
		return nil, fmt.Errorf("invalid issuer: %s", claims.Issuer)
	}

	return claims, nil
}
