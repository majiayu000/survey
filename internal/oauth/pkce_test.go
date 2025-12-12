package oauth

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateCodeVerifier(t *testing.T) {
	t.Run("generates valid code verifier", func(t *testing.T) {
		verifier := GenerateCodeVerifier()

		// Should be 64 characters (per spec recommendation)
		assert.Equal(t, 64, len(verifier))

		// Should only contain unreserved characters
		for _, c := range verifier {
			assert.True(t,
				(c >= 'a' && c <= 'z') ||
					(c >= 'A' && c <= 'Z') ||
					(c >= '0' && c <= '9') ||
					c == '-' || c == '.' || c == '_' || c == '~',
				"verifier contains invalid character: %c", c)
		}
	})

	t.Run("generates unique verifiers", func(t *testing.T) {
		verifier1 := GenerateCodeVerifier()
		verifier2 := GenerateCodeVerifier()

		assert.NotEqual(t, verifier1, verifier2)
	})
}

func TestGenerateCodeChallenge(t *testing.T) {
	t.Run("generates correct S256 challenge from verifier", func(t *testing.T) {
		verifier := "test-verifier-12345"

		challenge := GenerateCodeChallenge(verifier)

		// Should be base64url encoded SHA256 hash
		assert.NotEmpty(t, challenge)

		// Verify it's base64url (no padding)
		assert.NotContains(t, challenge, "=")

		// Manually compute expected value
		h := sha256.New()
		h.Write([]byte(verifier))
		hash := h.Sum(nil)
		expected := base64.RawURLEncoding.EncodeToString(hash)

		assert.Equal(t, expected, challenge)
	})

	t.Run("same verifier produces same challenge", func(t *testing.T) {
		verifier := "consistent-verifier"

		challenge1 := GenerateCodeChallenge(verifier)
		challenge2 := GenerateCodeChallenge(verifier)

		assert.Equal(t, challenge1, challenge2)
	})

	t.Run("different verifiers produce different challenges", func(t *testing.T) {
		challenge1 := GenerateCodeChallenge("verifier-1")
		challenge2 := GenerateCodeChallenge("verifier-2")

		assert.NotEqual(t, challenge1, challenge2)
	})
}

func TestGenerateState(t *testing.T) {
	t.Run("generates valid state parameter", func(t *testing.T) {
		state := GenerateState()

		// Should be 32 characters
		assert.Equal(t, 32, len(state))

		// Should be alphanumeric
		for _, c := range state {
			assert.True(t,
				(c >= 'a' && c <= 'z') ||
					(c >= 'A' && c <= 'Z') ||
					(c >= '0' && c <= '9'),
				"state contains invalid character: %c", c)
		}
	})

	t.Run("generates unique states", func(t *testing.T) {
		state1 := GenerateState()
		state2 := GenerateState()

		assert.NotEqual(t, state1, state2)
	})
}
