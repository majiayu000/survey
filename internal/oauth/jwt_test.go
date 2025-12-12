package oauth

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignClientAssertion(t *testing.T) {
	t.Run("creates valid client assertion JWT", func(t *testing.T) {
		privateJWK := GenerateSecretJWK()
		clientID := "https://example.com/client_metadata.json"
		audience := "https://bsky.social"

		jwt, err := SignClientAssertion(privateJWK, clientID, audience)
		require.NoError(t, err)
		assert.NotEmpty(t, jwt)

		// Should be a JWT (3 parts separated by dots)
		parts := strings.Split(jwt, ".")
		assert.Equal(t, 3, len(parts), "JWT should have 3 parts")

		// Decode header
		headerBytes, err := decodeJWTPart(parts[0])
		require.NoError(t, err)
		var header map[string]interface{}
		err = json.Unmarshal(headerBytes, &header)
		require.NoError(t, err)

		assert.Equal(t, "JWT", header["typ"])
		assert.Equal(t, "ES256", header["alg"])
		assert.NotEmpty(t, header["kid"])

		// Decode payload
		payloadBytes, err := decodeJWTPart(parts[1])
		require.NoError(t, err)
		var payload map[string]interface{}
		err = json.Unmarshal(payloadBytes, &payload)
		require.NoError(t, err)

		assert.Equal(t, clientID, payload["iss"])
		assert.Equal(t, clientID, payload["sub"])
		assert.Equal(t, audience, payload["aud"])
		assert.NotNil(t, payload["iat"])
		assert.NotNil(t, payload["exp"])
		assert.NotNil(t, payload["jti"])
	})

	t.Run("returns error for invalid JWK", func(t *testing.T) {
		_, err := SignClientAssertion("invalid", "client", "audience")
		assert.Error(t, err)
	})
}

func TestCreateDPoPProof(t *testing.T) {
	t.Run("creates valid DPoP proof", func(t *testing.T) {
		privateJWK := GenerateSecretJWK()
		method := "POST"
		url := "https://bsky.social/oauth/par"

		proof, err := CreateDPoPProof(privateJWK, method, url, "")
		require.NoError(t, err)
		assert.NotEmpty(t, proof)

		// Should be a JWT
		parts := strings.Split(proof, ".")
		assert.Equal(t, 3, len(parts))

		// Decode header
		headerBytes, err := decodeJWTPart(parts[0])
		require.NoError(t, err)
		var header map[string]interface{}
		err = json.Unmarshal(headerBytes, &header)
		require.NoError(t, err)

		assert.Equal(t, "dpop+jwt", header["typ"])
		assert.Equal(t, "ES256", header["alg"])
		assert.NotNil(t, header["jwk"]) // Public key should be embedded

		// Decode payload
		payloadBytes, err := decodeJWTPart(parts[1])
		require.NoError(t, err)
		var payload map[string]interface{}
		err = json.Unmarshal(payloadBytes, &payload)
		require.NoError(t, err)

		assert.Equal(t, method, payload["htm"])
		assert.Equal(t, url, payload["htu"])
		assert.NotNil(t, payload["jti"])
		assert.NotNil(t, payload["iat"])
		assert.NotNil(t, payload["exp"])
	})

	t.Run("includes nonce when provided", func(t *testing.T) {
		privateJWK := GenerateSecretJWK()
		nonce := "test-nonce-12345"

		proof, err := CreateDPoPProof(privateJWK, "POST", "https://example.com", nonce)
		require.NoError(t, err)

		parts := strings.Split(proof, ".")
		payloadBytes, err := decodeJWTPart(parts[1])
		require.NoError(t, err)
		var payload map[string]interface{}
		err = json.Unmarshal(payloadBytes, &payload)
		require.NoError(t, err)

		assert.Equal(t, nonce, payload["nonce"])
	})

	t.Run("returns error for invalid JWK", func(t *testing.T) {
		_, err := CreateDPoPProof("invalid", "POST", "https://example.com", "")
		assert.Error(t, err)
	})
}

func TestGenerateJTI(t *testing.T) {
	t.Run("generates valid JTI", func(t *testing.T) {
		jti := GenerateJTI()
		assert.NotEmpty(t, jti)
		assert.Equal(t, 32, len(jti))
	})

	t.Run("generates unique JTIs", func(t *testing.T) {
		jti1 := GenerateJTI()
		jti2 := GenerateJTI()
		assert.NotEqual(t, jti1, jti2)
	})
}

// Helper function to decode JWT parts (base64url)
func decodeJWTPart(s string) ([]byte, error) {
	// JWT uses base64url encoding without padding
	return base64.RawURLEncoding.DecodeString(s)
}
