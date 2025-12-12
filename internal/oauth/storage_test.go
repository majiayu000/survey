package oauth

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/openmeet-team/survey/internal/db"
)

// TestOAuthRequestStorage tests saving and retrieving OAuth request state
func TestOAuthRequestStorage(t *testing.T) {
	// Set up test database connection
	dbConn := setupTestDB(t)
	defer dbConn.Close()

	storage := NewStorage(dbConn)
	ctx := context.Background()

	t.Run("saves and retrieves OAuth request", func(t *testing.T) {
		req := OAuthRequest{
			State:          "test-state-123",
			Issuer:         "https://bsky.social",
			PKCEVerifier:   "verifier-123",
			DPoPPrivateKey: `{"kty":"EC","crv":"P-256","x":"..."}`,
			Destination:    "/surveys",
			ExpiresAt:      time.Now().Add(10 * time.Minute),
		}

		// Save request
		err := storage.SaveOAuthRequest(ctx, req)
		if err != nil {
			t.Fatalf("SaveOAuthRequest failed: %v", err)
		}

		// Retrieve request
		retrieved, err := storage.GetOAuthRequest(ctx, "test-state-123")
		if err != nil {
			t.Fatalf("GetOAuthRequest failed: %v", err)
		}

		if retrieved.State != req.State {
			t.Errorf("State mismatch: got %s, want %s", retrieved.State, req.State)
		}
		if retrieved.Issuer != req.Issuer {
			t.Errorf("Issuer mismatch: got %s, want %s", retrieved.Issuer, req.Issuer)
		}
		if retrieved.PKCEVerifier != req.PKCEVerifier {
			t.Errorf("PKCEVerifier mismatch: got %s, want %s", retrieved.PKCEVerifier, req.PKCEVerifier)
		}
		if retrieved.DPoPPrivateKey != req.DPoPPrivateKey {
			t.Errorf("DPoPPrivateKey mismatch")
		}
		if retrieved.Destination != req.Destination {
			t.Errorf("Destination mismatch: got %s, want %s", retrieved.Destination, req.Destination)
		}
	})

	t.Run("returns error for non-existent state", func(t *testing.T) {
		_, err := storage.GetOAuthRequest(ctx, "non-existent-state")
		if err == nil {
			t.Error("Expected error for non-existent state, got nil")
		}
		if err != sql.ErrNoRows {
			t.Errorf("Expected sql.ErrNoRows, got %v", err)
		}
	})

	t.Run("deletes OAuth request", func(t *testing.T) {
		req := OAuthRequest{
			State:          "delete-test-state",
			Issuer:         "https://bsky.social",
			PKCEVerifier:   "verifier-456",
			DPoPPrivateKey: `{"kty":"EC"}`,
			ExpiresAt:      time.Now().Add(10 * time.Minute),
		}

		err := storage.SaveOAuthRequest(ctx, req)
		if err != nil {
			t.Fatalf("SaveOAuthRequest failed: %v", err)
		}

		// Delete request
		err = storage.DeleteOAuthRequest(ctx, "delete-test-state")
		if err != nil {
			t.Fatalf("DeleteOAuthRequest failed: %v", err)
		}

		// Verify it's deleted
		_, err = storage.GetOAuthRequest(ctx, "delete-test-state")
		if err == nil {
			t.Error("Expected error after deletion, got nil")
		}
	})
}

// TestOAuthSessionStorage tests session creation and retrieval
func TestOAuthSessionStorage(t *testing.T) {
	dbConn := setupTestDB(t)
	defer dbConn.Close()

	storage := NewStorage(dbConn)
	ctx := context.Background()

	t.Run("creates and retrieves session", func(t *testing.T) {
		session := OAuthSession{
			ID:        "session-123",
			DID:       "did:plc:abc123xyz",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}

		err := storage.CreateSession(ctx, session)
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}

		// Retrieve session
		retrieved, err := storage.GetSessionByID(ctx, "session-123")
		if err != nil {
			t.Fatalf("GetSessionByID failed: %v", err)
		}

		if retrieved.ID != session.ID {
			t.Errorf("ID mismatch: got %s, want %s", retrieved.ID, session.ID)
		}
		if retrieved.DID != session.DID {
			t.Errorf("DID mismatch: got %s, want %s", retrieved.DID, session.DID)
		}
	})

	t.Run("creates and retrieves session with tokens", func(t *testing.T) {
		tokenExpiresAt := time.Now().Add(1 * time.Hour)
		session := OAuthSession{
			ID:             "session-with-tokens",
			DID:            "did:plc:abc123xyz",
			AccessToken:    "access-token-123",
			RefreshToken:   "refresh-token-456",
			DPoPKey:        `{"kty":"EC","crv":"P-256","x":"test"}`,
			PDSUrl:         "https://pds.example.com",
			TokenExpiresAt: &tokenExpiresAt,
			ExpiresAt:      time.Now().Add(24 * time.Hour),
		}

		err := storage.CreateSession(ctx, session)
		if err != nil {
			t.Fatalf("CreateSession with tokens failed: %v", err)
		}

		// Retrieve session
		retrieved, err := storage.GetSessionByID(ctx, "session-with-tokens")
		if err != nil {
			t.Fatalf("GetSessionByID failed: %v", err)
		}

		if retrieved.ID != session.ID {
			t.Errorf("ID mismatch: got %s, want %s", retrieved.ID, session.ID)
		}
		if retrieved.DID != session.DID {
			t.Errorf("DID mismatch: got %s, want %s", retrieved.DID, session.DID)
		}
		if retrieved.AccessToken != session.AccessToken {
			t.Errorf("AccessToken mismatch: got %s, want %s", retrieved.AccessToken, session.AccessToken)
		}
		if retrieved.RefreshToken != session.RefreshToken {
			t.Errorf("RefreshToken mismatch: got %s, want %s", retrieved.RefreshToken, session.RefreshToken)
		}
		if retrieved.DPoPKey != session.DPoPKey {
			t.Errorf("DPoPKey mismatch: got %s, want %s", retrieved.DPoPKey, session.DPoPKey)
		}
		if retrieved.PDSUrl != session.PDSUrl {
			t.Errorf("PDSUrl mismatch: got %s, want %s", retrieved.PDSUrl, session.PDSUrl)
		}
		if retrieved.TokenExpiresAt == nil {
			t.Error("TokenExpiresAt is nil")
		} else {
			// Database truncates to microseconds, so check within 1ms
			diff := retrieved.TokenExpiresAt.Sub(*session.TokenExpiresAt)
			if diff < -time.Millisecond || diff > time.Millisecond {
				t.Errorf("TokenExpiresAt mismatch (diff: %v): got %v, want %v", diff, retrieved.TokenExpiresAt, session.TokenExpiresAt)
			}
		}
	})

	t.Run("returns error for non-existent session", func(t *testing.T) {
		_, err := storage.GetSessionByID(ctx, "non-existent-session")
		if err == nil {
			t.Error("Expected error for non-existent session, got nil")
		}
		if err != sql.ErrNoRows {
			t.Errorf("Expected sql.ErrNoRows, got %v", err)
		}
	})

	t.Run("updates session tokens", func(t *testing.T) {
		session := OAuthSession{
			ID:          "update-session-123",
			DID:         "did:plc:update",
			AccessToken: "old-token",
			ExpiresAt:   time.Now().Add(24 * time.Hour),
		}

		err := storage.CreateSession(ctx, session)
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}

		// Update tokens
		newTokenExpiresAt := time.Now().Add(2 * time.Hour)
		err = storage.UpdateSessionTokens(ctx, "update-session-123", "new-access-token", "new-refresh-token", &newTokenExpiresAt)
		if err != nil {
			t.Fatalf("UpdateSessionTokens failed: %v", err)
		}

		// Retrieve and verify
		retrieved, err := storage.GetSessionByID(ctx, "update-session-123")
		if err != nil {
			t.Fatalf("GetSessionByID failed: %v", err)
		}

		if retrieved.AccessToken != "new-access-token" {
			t.Errorf("AccessToken not updated: got %s, want new-access-token", retrieved.AccessToken)
		}
		if retrieved.RefreshToken != "new-refresh-token" {
			t.Errorf("RefreshToken not updated: got %s, want new-refresh-token", retrieved.RefreshToken)
		}
	})

	t.Run("deletes session", func(t *testing.T) {
		session := OAuthSession{
			ID:        "delete-session-123",
			DID:       "did:plc:delete",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}

		err := storage.CreateSession(ctx, session)
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}

		// Delete session
		err = storage.DeleteSession(ctx, "delete-session-123")
		if err != nil {
			t.Fatalf("DeleteSession failed: %v", err)
		}

		// Verify it's deleted
		_, err = storage.GetSessionByID(ctx, "delete-session-123")
		if err == nil {
			t.Error("Expected error after deletion, got nil")
		}
	})
}

// setupTestDB creates a test database connection
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	// Get DB config from environment
	cfg, err := db.ConfigFromEnv()
	if err != nil {
		t.Fatalf("Failed to load database config: %v", err)
	}

	// Use test database
	ctx := context.Background()
	dbConn, err := db.Connect(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	// Clean up test data before each test
	_, err = dbConn.Exec("DELETE FROM oauth_requests WHERE state LIKE '%test%'")
	if err != nil {
		t.Logf("Warning: failed to clean oauth_requests: %v", err)
	}

	_, err = dbConn.Exec("DELETE FROM oauth_sessions WHERE id LIKE '%test%' OR id LIKE '%session%' OR id LIKE '%delete%'")
	if err != nil {
		t.Logf("Warning: failed to clean oauth_sessions: %v", err)
	}

	return dbConn
}
