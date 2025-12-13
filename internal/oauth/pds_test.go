package oauth

import (
	"encoding/json"
	"io"
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

	t.Run("includes validate false in payload for custom lexicons", func(t *testing.T) {
		// Mock PDS server that captures and validates the request payload
		pdsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Read the request body
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("Failed to read request body: %v", err)
			}

			// Parse the JSON payload
			var payload map[string]interface{}
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("Failed to unmarshal payload: %v", err)
			}

			// Verify validate field exists and is false
			validateVal, exists := payload["validate"]
			if !exists {
				t.Error("Expected 'validate' field in payload")
			} else if validateVal != false {
				t.Errorf("Expected validate=false, got %v", validateVal)
			}

			// Verify other expected fields
			if payload["repo"] != "did:plc:test123" {
				t.Errorf("Expected repo=did:plc:test123, got %v", payload["repo"])
			}
			if payload["collection"] != "net.openmeet.survey" {
				t.Errorf("Expected collection=net.openmeet.survey, got %v", payload["collection"])
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
		}

		_, _, err := CreateRecord(session, "net.openmeet.survey", "", record)
		if err != nil {
			t.Fatalf("CreateRecord failed: %v", err)
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

// TestListRecords tests fetching records from a collection
func TestListRecords(t *testing.T) {
	t.Run("lists records without auth", func(t *testing.T) {
		// Mock PDS server
		pdsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify URL path
			if r.URL.Path != "/xrpc/com.atproto.repo.listRecords" {
				t.Errorf("Expected path /xrpc/com.atproto.repo.listRecords, got %s", r.URL.Path)
			}

			// Verify query parameters
			repo := r.URL.Query().Get("repo")
			collection := r.URL.Query().Get("collection")
			if repo != "did:plc:test123" {
				t.Errorf("Expected repo=did:plc:test123, got %s", repo)
			}
			if collection != "net.openmeet.survey" {
				t.Errorf("Expected collection=net.openmeet.survey, got %s", collection)
			}

			// Return success response
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"records": [
					{
						"uri": "at://did:plc:test123/net.openmeet.survey/abc123",
						"cid": "bafytest1",
						"value": {"question": "Test?"}
					},
					{
						"uri": "at://did:plc:test123/net.openmeet.survey/def456",
						"cid": "bafytest2",
						"value": {"question": "Another?"}
					}
				],
				"cursor": "next-page"
			}`))
		}))
		defer pdsServer.Close()

		resp, err := ListRecords(pdsServer.URL, "did:plc:test123", "net.openmeet.survey", "", 50)
		if err != nil {
			t.Fatalf("ListRecords failed: %v", err)
		}

		if len(resp.Records) != 2 {
			t.Errorf("Expected 2 records, got %d", len(resp.Records))
		}

		if resp.Cursor != "next-page" {
			t.Errorf("Expected cursor 'next-page', got %s", resp.Cursor)
		}

		// Check first record
		if resp.Records[0].URI != "at://did:plc:test123/net.openmeet.survey/abc123" {
			t.Errorf("Unexpected URI: %s", resp.Records[0].URI)
		}
		if resp.Records[0].RKey != "abc123" {
			t.Errorf("Expected rkey 'abc123', got %s", resp.Records[0].RKey)
		}
	})

	t.Run("returns error for invalid PDS URL", func(t *testing.T) {
		_, err := ListRecords("", "did:plc:test", "net.openmeet.survey", "", 50)
		if err == nil {
			t.Error("Expected error for empty PDS URL")
		}
	})
}

// TestDeleteRecord tests deleting a single record
func TestDeleteRecord(t *testing.T) {
	t.Run("deletes record with valid session", func(t *testing.T) {
		// Mock PDS server
		pdsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request has proper headers
			if r.Header.Get("Authorization") == "" {
				t.Error("Expected Authorization header")
			}
			if r.Header.Get("DPoP") == "" {
				t.Error("Expected DPoP header")
			}

			// Verify URL path
			if r.URL.Path != "/xrpc/com.atproto.repo.deleteRecord" {
				t.Errorf("Expected path /xrpc/com.atproto.repo.deleteRecord, got %s", r.URL.Path)
			}

			// Read and verify payload
			body, _ := io.ReadAll(r.Body)
			var payload map[string]interface{}
			json.Unmarshal(body, &payload)

			if payload["repo"] != "did:plc:test123" {
				t.Errorf("Expected repo=did:plc:test123, got %v", payload["repo"])
			}
			if payload["collection"] != "net.openmeet.survey" {
				t.Errorf("Expected collection=net.openmeet.survey, got %v", payload["collection"])
			}
			if payload["rkey"] != "abc123" {
				t.Errorf("Expected rkey=abc123, got %v", payload["rkey"])
			}

			// Return success response
			w.WriteHeader(http.StatusOK)
		}))
		defer pdsServer.Close()

		tokenExpiresAt := time.Now().Add(1 * time.Hour)
		session := &OAuthSession{
			ID:             "test-session",
			DID:            "did:plc:test123",
			AccessToken:    "test-access-token",
			DPoPKey:        GenerateSecretJWK(),
			PDSUrl:         pdsServer.URL,
			TokenExpiresAt: &tokenExpiresAt,
		}

		err := DeleteRecord(session, "net.openmeet.survey", "abc123")
		if err != nil {
			t.Fatalf("DeleteRecord failed: %v", err)
		}
	})

	t.Run("returns error for nil session", func(t *testing.T) {
		err := DeleteRecord(nil, "net.openmeet.survey", "abc123")
		if err == nil {
			t.Error("Expected error for nil session")
		}
	})
}

// TestUpdateRecord tests updating an existing record
func TestUpdateRecord(t *testing.T) {
	t.Run("updates record with valid session", func(t *testing.T) {
		// Mock PDS server
		pdsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request has proper headers
			if r.Header.Get("Authorization") == "" {
				t.Error("Expected Authorization header")
			}
			if r.Header.Get("DPoP") == "" {
				t.Error("Expected DPoP header")
			}

			// Verify URL path
			if r.URL.Path != "/xrpc/com.atproto.repo.putRecord" {
				t.Errorf("Expected path /xrpc/com.atproto.repo.putRecord, got %s", r.URL.Path)
			}

			// Read and verify payload
			body, _ := io.ReadAll(r.Body)
			var payload map[string]interface{}
			json.Unmarshal(body, &payload)

			if payload["repo"] != "did:plc:test123" {
				t.Errorf("Expected repo=did:plc:test123, got %v", payload["repo"])
			}
			if payload["collection"] != "net.openmeet.survey" {
				t.Errorf("Expected collection=net.openmeet.survey, got %v", payload["collection"])
			}
			if payload["rkey"] != "abc123" {
				t.Errorf("Expected rkey=abc123, got %v", payload["rkey"])
			}

			// Return success response
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"uri":"at://did:plc:test123/net.openmeet.survey/abc123","cid":"bafynewcid"}`))
		}))
		defer pdsServer.Close()

		tokenExpiresAt := time.Now().Add(1 * time.Hour)
		session := &OAuthSession{
			ID:             "test-session",
			DID:            "did:plc:test123",
			AccessToken:    "test-access-token",
			DPoPKey:        GenerateSecretJWK(),
			PDSUrl:         pdsServer.URL,
			TokenExpiresAt: &tokenExpiresAt,
		}

		record := map[string]interface{}{
			"question": "Updated question?",
		}

		uri, cid, err := UpdateRecord(session, "net.openmeet.survey", "abc123", record)
		if err != nil {
			t.Fatalf("UpdateRecord failed: %v", err)
		}

		if uri != "at://did:plc:test123/net.openmeet.survey/abc123" {
			t.Errorf("Unexpected URI: %s", uri)
		}
		if cid != "bafynewcid" {
			t.Errorf("Unexpected CID: %s", cid)
		}
	})

	t.Run("returns error for nil session", func(t *testing.T) {
		record := map[string]interface{}{"test": "data"}
		_, _, err := UpdateRecord(nil, "net.openmeet.survey", "abc123", record)
		if err == nil {
			t.Error("Expected error for nil session")
		}
	})
}
