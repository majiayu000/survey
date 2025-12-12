package oauth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleToDID(t *testing.T) {
	t.Run("resolves handle to DID via DNS", func(t *testing.T) {
		// Using a known Bluesky handle - bsky.app
		did, err := HandleToDID("bsky.app")
		require.NoError(t, err)
		assert.True(t, len(did) > 0)
		assert.True(t, did == "did:plc:z72i7hdynmk6r22z27h6tvur" || did[:4] == "did:")
	})

	t.Run("returns error for non-existent handle", func(t *testing.T) {
		_, err := HandleToDID("this-handle-definitely-does-not-exist-12345.com")
		assert.Error(t, err)
	})

	t.Run("returns error for empty handle", func(t *testing.T) {
		_, err := HandleToDID("")
		assert.Error(t, err)
	})
}

func TestDIDToPDS(t *testing.T) {
	t.Run("resolves DID to PDS endpoint", func(t *testing.T) {
		// Using bsky.app's DID
		pds, err := DIDToPDS("did:plc:z72i7hdynmk6r22z27h6tvur")
		require.NoError(t, err)
		assert.NotEmpty(t, pds)
		assert.Contains(t, pds, "https://")
	})

	t.Run("returns error for invalid DID", func(t *testing.T) {
		_, err := DIDToPDS("not-a-valid-did")
		assert.Error(t, err)
	})

	t.Run("returns error for empty DID", func(t *testing.T) {
		_, err := DIDToPDS("")
		assert.Error(t, err)
	})
}

func TestPDSToAuthServer(t *testing.T) {
	t.Run("resolves PDS to authorization server", func(t *testing.T) {
		// First resolve a DID to get actual PDS
		pds, err := DIDToPDS("did:plc:z72i7hdynmk6r22z27h6tvur")
		require.NoError(t, err)

		// Then resolve PDS to auth server
		authServer, err := PDSToAuthServer(pds)
		require.NoError(t, err)
		assert.NotEmpty(t, authServer)
		assert.Contains(t, authServer, "https://")
	})

	t.Run("returns error for invalid PDS URL", func(t *testing.T) {
		_, err := PDSToAuthServer("not-a-url")
		assert.Error(t, err)
	})

	t.Run("returns error for empty URL", func(t *testing.T) {
		_, err := PDSToAuthServer("")
		assert.Error(t, err)
	})
}

func TestAuthServerToPAREndpoint(t *testing.T) {
	t.Run("resolves auth server to PAR endpoint", func(t *testing.T) {
		// Bluesky's auth server
		parEndpoint, err := AuthServerToPAREndpoint("https://bsky.social")
		require.NoError(t, err)
		assert.NotEmpty(t, parEndpoint)
		assert.Contains(t, parEndpoint, "https://")
	})

	t.Run("returns error for invalid auth server", func(t *testing.T) {
		_, err := AuthServerToPAREndpoint("not-a-url")
		assert.Error(t, err)
	})
}
