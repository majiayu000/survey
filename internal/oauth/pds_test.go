package oauth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestCreateRecord tests writing ATProto records to a PDS
func TestCreateRecord(t *testing.T) {
	t.Run("creates record with valid session", func(t *testing.T) {
		// Mock PDS server
		pdsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request has proper headers
			if r.Header.Get("Authorization") == "" {
				t.Error("Expected Authorization header")
			}
			if r.Header.Get("DPoP") == "" {
				t.Error("Expected DPoP header")
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("Expected Content-Type: application/json, got %s", r.Header.Get("Content-Type"))
			}

			// Verify URL path
			if r.URL.Path != "/xrpc/com.atproto.repo.createRecord" {
				t.Errorf("Expected path /xrpc/com.atproto.repo.createRecord, got %s", r.URL.Path)
			}

			// Return success response
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"uri":"at://did:plc:test123/net.openmeet.survey/abc123","cid":"bafytest123"}`))
		}))
		defer pdsServer.Close()

		tokenExpiresAt := time.Now().Add(1 * time.Hour)
		session := &OAuthSession{
			ID:             "test-session",
			DID:            "did:plc:test123",
			AccessToken:    "test-access-token",
			RefreshToken:   "test-refresh-token",
			DPoPKey:        GenerateSecretJWK(),
			PDSUrl:         pdsServer.URL,
			TokenExpiresAt: &tokenExpiresAt,
		}

		record := map[string]interface{}{
			"question": "What's your favorite color?",
			"options":  []string{"Red", "Blue", "Green"},
			"createdAt": time.Now().Format(time.RFC3339),
		}

		uri, cid, err := CreateRecord(session, "net.openmeet.survey", "test123", record)
		if err != nil {
			t.Fatalf("CreateRecord failed: %v", err)
		}

		expectedURI := "at://did:plc:test123/net.openmeet.survey/abc123"
		if uri != expectedURI {
			t.Errorf("URI mismatch: got %s, want %s", uri, expectedURI)
		}

		expectedCID := "bafytest123"
		if cid != expectedCID {
			t.Errorf("CID mismatch: got %s, want %s", cid, expectedCID)
		}
	})

	t.Run("returns error for nil session", func(t *testing.T) {
		record := map[string]interface{}{"test": "data"}
		_, _, err := CreateRecord(nil, "net.openmeet.survey", "test123", record)
		if err == nil {
			t.Error("Expected error for nil session")
		}
	})

	t.Run("returns error for missing access token", func(t *testing.T) {
		session := &OAuthSession{
			DID:    "did:plc:test",
			PDSUrl: "https://pds.example.com",
		}
		record := map[string]interface{}{"test": "data"}
		_, _, err := CreateRecord(session, "net.openmeet.survey", "test123", record)
		if err == nil {
			t.Error("Expected error for missing access token")
		}
	})

	t.Run("returns error for missing PDS URL", func(t *testing.T) {
		session := &OAuthSession{
			DID:         "did:plc:test",
			AccessToken: "test-token",
			DPoPKey:     GenerateSecretJWK(),
		}
		record := map[string]interface{}{"test": "data"}
		_, _, err := CreateRecord(session, "net.openmeet.survey", "test123", record)
		if err == nil {
			t.Error("Expected error for missing PDS URL")
		}
	})

	t.Run("returns error for expired token without refresh", func(t *testing.T) {
		expiredTime := time.Now().Add(-1 * time.Hour)
		session := &OAuthSession{
			DID:            "did:plc:test",
			AccessToken:    "expired-token",
			DPoPKey:        GenerateSecretJWK(),
			PDSUrl:         "https://pds.example.com",
			TokenExpiresAt: &expiredTime,
		}
		record := map[string]interface{}{"test": "data"}
		_, _, err := CreateRecord(session, "net.openmeet.survey", "test123", record)
		if err == nil {
			t.Error("Expected error for expired token without refresh token")
		}
	})
}

// TestRefreshToken tests token refresh functionality
func TestRefreshToken(t *testing.T) {
	t.Run("refreshes expired token", func(t *testing.T) {
		// Mock auth server
		var authServerURL string
		authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/.well-known/oauth-authorization-server" {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"token_endpoint":"` + authServerURL + `/token"}`))
				return
			}
			if r.URL.Path == "/token" {
				// Verify refresh grant
				r.ParseForm()
				if r.Form.Get("grant_type") != "refresh_token" {
					t.Errorf("Expected grant_type=refresh_token, got %s", r.Form.Get("grant_type"))
				}
				if r.Form.Get("refresh_token") == "" {
					t.Error("Expected refresh_token parameter")
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{
					"access_token": "new-access-token",
					"refresh_token": "new-refresh-token",
					"token_type": "DPoP",
					"expires_in": 3600
				}`))
				return
			}
		}))
		defer authServer.Close()
		authServerURL = authServer.URL

		expiredTime := time.Now().Add(-1 * time.Hour)
		session := &OAuthSession{
			ID:             "test-session",
			DID:            "did:plc:test",
			AccessToken:    "old-token",
			RefreshToken:   "test-refresh-token",
			DPoPKey:        GenerateSecretJWK(),
			PDSUrl:         "https://pds.example.com",
			TokenExpiresAt: &expiredTime,
		}

		newToken, newRefresh, expiresIn, err := RefreshAccessToken(session, authServer.URL, "client-id", GenerateSecretJWK())
		if err != nil {
			t.Fatalf("RefreshAccessToken failed: %v", err)
		}

		if newToken != "new-access-token" {
			t.Errorf("Expected new-access-token, got %s", newToken)
		}
		if newRefresh != "new-refresh-token" {
			t.Errorf("Expected new-refresh-token, got %s", newRefresh)
		}
		if expiresIn != 3600 {
			t.Errorf("Expected expires_in=3600, got %d", expiresIn)
		}
	})

	t.Run("returns error for nil session", func(t *testing.T) {
		_, _, _, err := RefreshAccessToken(nil, "https://auth.example.com", "client-id", GenerateSecretJWK())
		if err == nil {
			t.Error("Expected error for nil session")
		}
	})

	t.Run("returns error for missing refresh token", func(t *testing.T) {
		session := &OAuthSession{
			DID:         "did:plc:test",
			AccessToken: "test-token",
		}
		_, _, _, err := RefreshAccessToken(session, "https://auth.example.com", "client-id", GenerateSecretJWK())
		if err == nil {
			t.Error("Expected error for missing refresh token")
		}
	})
}
