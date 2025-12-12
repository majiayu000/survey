package oauth

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateSecretJWK(t *testing.T) {
	t.Run("generates valid JWK", func(t *testing.T) {
		jwk := GenerateSecretJWK()

		// Should not be empty
		require.NotEmpty(t, jwk)

		// Should be valid JSON
		var parsed map[string]interface{}
		err := json.Unmarshal([]byte(jwk), &parsed)
		require.NoError(t, err)

		// Should have required fields
		assert.Equal(t, "EC", parsed["kty"])
		assert.Equal(t, "sig", parsed["use"])
		assert.Equal(t, "ES256", parsed["alg"])
		assert.NotEmpty(t, parsed["kid"])
		assert.NotEmpty(t, parsed["crv"])
		assert.NotEmpty(t, parsed["x"])
		assert.NotEmpty(t, parsed["y"])
		assert.NotEmpty(t, parsed["d"]) // Private key component
	})

	t.Run("generates unique keys", func(t *testing.T) {
		jwk1 := GenerateSecretJWK()
		jwk2 := GenerateSecretJWK()

		// Should be different keys
		assert.NotEqual(t, jwk1, jwk2)
	})
}

func TestPrivateJWKToPublicJWK(t *testing.T) {
	t.Run("extracts public key from private JWK", func(t *testing.T) {
		privateJWK := GenerateSecretJWK()

		publicJWK, err := PrivateJWKToPublicJWK(privateJWK)
		require.NoError(t, err)
		require.NotEmpty(t, publicJWK)

		// Should be valid JSON
		var parsed map[string]interface{}
		err = json.Unmarshal([]byte(publicJWK), &parsed)
		require.NoError(t, err)

		// Should have public fields
		assert.Equal(t, "EC", parsed["kty"])
		assert.Equal(t, "sig", parsed["use"])
		assert.Equal(t, "ES256", parsed["alg"])
		assert.NotEmpty(t, parsed["kid"])
		assert.NotEmpty(t, parsed["x"])
		assert.NotEmpty(t, parsed["y"])

		// Should NOT have private key component
		assert.Nil(t, parsed["d"])
	})

	t.Run("returns error for invalid JWK", func(t *testing.T) {
		_, err := PrivateJWKToPublicJWK("not a valid jwk")
		assert.Error(t, err)
	})

	t.Run("returns error for empty string", func(t *testing.T) {
		_, err := PrivateJWKToPublicJWK("")
		assert.Error(t, err)
	})
}
