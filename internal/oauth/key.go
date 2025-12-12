package oauth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-jose/go-jose/v4"
)

// GenerateSecretJWK generates a new ES256 private key in JWK format
func GenerateSecretJWK() string {
	now := time.Now().Unix()

	// Generate ECDSA P-256 private key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}

	// Create JWK object
	jwk := jose.JSONWebKey{
		Key:       privateKey,
		KeyID:     fmt.Sprintf("key-%d", now),
		Algorithm: string(jose.ES256),
		Use:       "sig",
	}

	jwkJSON, err := json.Marshal(jwk)
	if err != nil {
		panic(err)
	}

	return string(jwkJSON)
}

// PrivateJWKToPublicJWK extracts the public key from a private JWK
func PrivateJWKToPublicJWK(privateJWK string) (string, error) {
	if privateJWK == "" {
		return "", fmt.Errorf("private JWK cannot be empty")
	}

	var jwk jose.JSONWebKey
	err := json.Unmarshal([]byte(privateJWK), &jwk)
	if err != nil {
		return "", fmt.Errorf("failed to parse private key JWK: %v", err)
	}

	publicKey := jwk.Public()
	publicJWK, err := json.Marshal(publicKey)
	if err != nil {
		return "", fmt.Errorf("failed to marshal public key JWK: %v", err)
	}

	return string(publicJWK), nil
}
