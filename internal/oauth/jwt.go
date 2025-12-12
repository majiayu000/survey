package oauth

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-jose/go-jose/v4"
)

// SignClientAssertion creates a signed JWT for client authentication
// Used in the client_assertion parameter for PAR requests
func SignClientAssertion(privateJWK, clientID, audience string) (string, error) {
	var jwk jose.JSONWebKey
	err := json.Unmarshal([]byte(privateJWK), &jwk)
	if err != nil {
		return "", fmt.Errorf("failed to parse private key JWK: %v", err)
	}

	// Create signer with kid in header
	signingKey := jose.SigningKey{Algorithm: jose.ES256, Key: jwk.Key}
	signer, err := jose.NewSigner(signingKey, (&jose.SignerOptions{
		ExtraHeaders: map[jose.HeaderKey]interface{}{
			"kid": jwk.KeyID,
		},
	}).WithType("JWT"))
	if err != nil {
		return "", fmt.Errorf("failed to create signer: %v", err)
	}

	// Create claims
	now := time.Now()
	claims := map[string]interface{}{
		"iss": clientID,
		"sub": clientID,
		"aud": audience,
		"iat": now.Unix(),
		"exp": now.Add(5 * time.Minute).Unix(),
		"jti": GenerateJTI(),
	}

	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("failed to marshal claims: %v", err)
	}

	// Sign the claims
	object, err := signer.Sign(claimsJSON)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %v", err)
	}

	// Serialize to compact JWT format
	token, err := object.CompactSerialize()
	if err != nil {
		return "", fmt.Errorf("failed to serialize token: %v", err)
	}

	return token, nil
}

// CreateDPoPProof creates a DPoP proof JWT
// DPoP (Demonstrating Proof-of-Possession) binds tokens to specific keys
func CreateDPoPProof(privateJWK, method, url, nonce string) (string, error) {
	var jwk jose.JSONWebKey
	err := json.Unmarshal([]byte(privateJWK), &jwk)
	if err != nil {
		return "", fmt.Errorf("failed to parse private key JWK: %v", err)
	}

	// Get public key for embedding in header
	publicKey := jwk.Public()
	publicJWKMap := make(map[string]interface{})
	publicJWKBytes, err := publicKey.MarshalJSON()
	if err != nil {
		return "", fmt.Errorf("failed to marshal public key: %v", err)
	}
	err = json.Unmarshal(publicJWKBytes, &publicJWKMap)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal public key: %v", err)
	}

	// Create signer with DPoP-specific headers
	opts := &jose.SignerOptions{}
	opts.WithHeader("typ", "dpop+jwt")
	opts.WithHeader("jwk", publicJWKMap)

	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.ES256, Key: jwk.Key}, opts)
	if err != nil {
		return "", fmt.Errorf("failed to create signer: %v", err)
	}

	// Create DPoP claims
	now := time.Now()
	claims := map[string]interface{}{
		"jti": GenerateJTI(),
		"htm": method,
		"htu": url,
		"iat": now.Unix(),
		"exp": now.Add(30 * time.Second).Unix(),
	}

	// Add nonce if provided
	if nonce != "" {
		claims["nonce"] = nonce
	}

	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("failed to marshal claims: %v", err)
	}

	// Sign the claims
	object, err := signer.Sign(claimsJSON)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %v", err)
	}

	// Serialize to compact JWT format
	token, err := object.CompactSerialize()
	if err != nil {
		return "", fmt.Errorf("failed to serialize token: %v", err)
	}

	return token, nil
}

// GenerateJTI generates a unique JWT ID
func GenerateJTI() string {
	return GenerateState() // Reuse state generation logic
}
