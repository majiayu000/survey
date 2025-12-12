package oauth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchProfile(t *testing.T) {
	t.Run("successfully fetches profile", func(t *testing.T) {
		// Create mock Bluesky API server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify the request
			assert.Equal(t, "/xrpc/app.bsky.actor.getProfile", r.URL.Path)
			assert.Equal(t, "did:plc:test123", r.URL.Query().Get("actor"))

			// Return mock profile
			profile := map[string]interface{}{
				"did":         "did:plc:test123",
				"handle":      "alice.bsky.social",
				"displayName": "Alice Smith",
				"avatar":      "https://cdn.bsky.app/avatar123.jpg",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(profile)
		}))
		defer server.Close()

		// Fetch profile
		profile, err := fetchProfileFromAPI("did:plc:test123", server.URL)
		require.NoError(t, err)
		require.NotNil(t, profile)

		assert.Equal(t, "did:plc:test123", profile.DID)
		assert.Equal(t, "alice.bsky.social", profile.Handle)
		assert.Equal(t, "Alice Smith", profile.DisplayName)
		assert.Equal(t, "https://cdn.bsky.app/avatar123.jpg", profile.Avatar)
	})

	t.Run("handles missing optional fields", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Return minimal profile (only required fields)
			profile := map[string]interface{}{
				"did":    "did:plc:test123",
				"handle": "alice.bsky.social",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(profile)
		}))
		defer server.Close()

		profile, err := fetchProfileFromAPI("did:plc:test123", server.URL)
		require.NoError(t, err)
		require.NotNil(t, profile)

		assert.Equal(t, "did:plc:test123", profile.DID)
		assert.Equal(t, "alice.bsky.social", profile.Handle)
		assert.Equal(t, "", profile.DisplayName)
		assert.Equal(t, "", profile.Avatar)
	})

	t.Run("returns error on HTTP failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		profile, err := fetchProfileFromAPI("did:plc:test123", server.URL)
		assert.Error(t, err)
		assert.Nil(t, profile)
		assert.Contains(t, err.Error(), "unexpected status code")
	})

	t.Run("returns error on invalid JSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("invalid json"))
		}))
		defer server.Close()

		profile, err := fetchProfileFromAPI("did:plc:test123", server.URL)
		assert.Error(t, err)
		assert.Nil(t, profile)
	})
}

func TestGetProfile(t *testing.T) {
	t.Run("fetches and caches profile", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			profile := map[string]interface{}{
				"did":         "did:plc:test123",
				"handle":      "alice.bsky.social",
				"displayName": "Alice Smith",
				"avatar":      "https://cdn.bsky.app/avatar123.jpg",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(profile)
		}))
		defer server.Close()

		// Override default API URL for testing
		oldURL := defaultBlueskyAPIURL
		defaultBlueskyAPIURL = server.URL
		defer func() { defaultBlueskyAPIURL = oldURL }()

		// Clear cache
		profileCache.mu.Lock()
		profileCache.profiles = make(map[string]*cachedProfile)
		profileCache.mu.Unlock()

		// First call should fetch from API
		profile1, err := GetProfile("did:plc:test123")
		require.NoError(t, err)
		require.NotNil(t, profile1)
		assert.Equal(t, "alice.bsky.social", profile1.Handle)

		// Second call should use cache (server won't be called again)
		profile2, err := GetProfile("did:plc:test123")
		require.NoError(t, err)
		require.NotNil(t, profile2)
		assert.Equal(t, profile1.Handle, profile2.Handle)
	})

	t.Run("returns error on fetch failure", func(t *testing.T) {
		// Use invalid URL to trigger error
		oldURL := defaultBlueskyAPIURL
		defaultBlueskyAPIURL = "http://invalid-url-that-does-not-exist.local"
		defer func() { defaultBlueskyAPIURL = oldURL }()

		// Clear cache
		profileCache.mu.Lock()
		profileCache.profiles = make(map[string]*cachedProfile)
		profileCache.mu.Unlock()

		profile, err := GetProfile("did:plc:test123")
		assert.Error(t, err)
		assert.Nil(t, profile)
	})
}
