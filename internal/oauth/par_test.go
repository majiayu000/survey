package oauth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPARRequest(t *testing.T) {
	// Note: This is an integration test that requires network access
	// In a real CI environment, you'd want to mock the HTTP calls
	t.Run("executes complete PAR flow", func(t *testing.T) {
		t.Skip("Skipping integration test - requires actual Bluesky auth server")

		clientJWK := GenerateSecretJWK()
		dpopJWK := GenerateSecretJWK()

		config := PARConfig{
			ClientID:      "https://survey.openmeet.net/oauth/client-metadata.json",
			RedirectURI:   "https://survey.openmeet.net/oauth/callback",
			Scope:         "atproto transition:generic",
			State:         GenerateState(),
			CodeVerifier:  GenerateCodeVerifier(),
			DPoPKey:       dpopJWK,
			ClientKey:     clientJWK,
			PAREndpoint:   "https://bsky.social/oauth/par",
			AuthServerURL: "https://bsky.social",
		}

		requestURI, err := ExecutePAR(config)
		require.NoError(t, err)
		assert.NotEmpty(t, requestURI)
		assert.Contains(t, requestURI, "urn:ietf:params:oauth:request_uri:")
	})
}

func TestPARConfig_Validate(t *testing.T) {
	validConfig := func() PARConfig {
		return PARConfig{
			ClientID:      "https://example.com/client",
			RedirectURI:   "https://example.com/callback",
			Scope:         "atproto",
			State:         "test-state",
			CodeVerifier:  GenerateCodeVerifier(),
			DPoPKey:       GenerateSecretJWK(),
			ClientKey:     GenerateSecretJWK(),
			PAREndpoint:   "https://example.com/par",
			AuthServerURL: "https://example.com",
		}
	}

	t.Run("valid config passes validation", func(t *testing.T) {
		config := validConfig()
		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("missing client ID fails", func(t *testing.T) {
		config := validConfig()
		config.ClientID = ""
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ClientID")
	})

	t.Run("missing redirect URI fails", func(t *testing.T) {
		config := validConfig()
		config.RedirectURI = ""
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "RedirectURI")
	})

	t.Run("missing state fails", func(t *testing.T) {
		config := validConfig()
		config.State = ""
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "State")
	})

	t.Run("missing code verifier fails", func(t *testing.T) {
		config := validConfig()
		config.CodeVerifier = ""
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "CodeVerifier")
	})
}
