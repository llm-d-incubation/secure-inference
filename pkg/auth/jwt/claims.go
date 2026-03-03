package jwt

import (
	gojwt "github.com/golang-jwt/jwt/v5"
)

// UserClaims defines the structure for JWT claims.
type UserClaims struct {
	Username     string `json:"username"`
	Role         string `json:"role"`
	Organization string `json:"organization"`
	gojwt.RegisteredClaims
}
