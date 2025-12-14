//go:build e2e

package oauth

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// TestCleanupExpiredRequests verifies expired requests are deleted
func TestCleanupExpiredRequests(t *testing.T) {
	dbConn := setupTestDB(t)
	defer dbConn.Close()

	storage := NewStorage(dbConn)
	ctx := context.Background()

	t.Run("removes expired requests", func(t *testing.T) {
		// Create an expired request
		expiredReq := OAuthRequest{
			State:          "cleanup-test-expired",
			Issuer:         "https://bsky.social",
			PKCEVerifier:   "verifier-expired",
			DPoPPrivateKey: `{"kty":"EC"}`,
			Destination:    "/surveys",
			ExpiresAt:      time.Now().Add(-1 * time.Hour), // Expired 1 hour ago
		}

		err := storage.SaveOAuthRequest(ctx, expiredReq)
		if err != nil {
			t.Fatalf("SaveOAuthRequest failed: %v", err)
		}

		// Create a valid request
		validReq := OAuthRequest{
			State:          "cleanup-test-valid",
			Issuer:         "https://bsky.social",
			PKCEVerifier:   "verifier-valid",
			DPoPPrivateKey: `{"kty":"EC"}`,
			Destination:    "/surveys",
			ExpiresAt:      time.Now().Add(1 * time.Hour), // Valid for 1 hour
		}

		err = storage.SaveOAuthRequest(ctx, validReq)
		if err != nil {
			t.Fatalf("SaveOAuthRequest failed: %v", err)
		}

		// Run cleanup
		count, err := storage.CleanupExpiredRequests(ctx)
		if err != nil {
			t.Fatalf("CleanupExpiredRequests failed: %v", err)
		}

		if count < 1 {
			t.Errorf("Expected at least 1 expired request cleaned, got %d", count)
		}

		// Verify expired request is deleted
		_, err = storage.GetOAuthRequest(ctx, "cleanup-test-expired")
		if err != sql.ErrNoRows {
			t.Errorf("Expected expired request to be deleted, got error: %v", err)
		}

		// Verify valid request still exists
		retrieved, err := storage.GetOAuthRequest(ctx, "cleanup-test-valid")
		if err != nil {
			t.Errorf("Expected valid request to still exist, got error: %v", err)
		}
		if retrieved.State != "cleanup-test-valid" {
			t.Errorf("Valid request state mismatch: got %s", retrieved.State)
		}

		// Cleanup test data
		storage.DeleteOAuthRequest(ctx, "cleanup-test-valid")
	})
}

// TestCleanupExpiredSessions verifies expired sessions are deleted
func TestCleanupExpiredSessions(t *testing.T) {
	dbConn := setupTestDB(t)
	defer dbConn.Close()

	storage := NewStorage(dbConn)
	ctx := context.Background()

	t.Run("removes expired sessions", func(t *testing.T) {
		// Create an expired session
		expiredSession := OAuthSession{
			ID:        "cleanup-session-expired",
			DID:       "did:plc:expired",
			ExpiresAt: time.Now().Add(-1 * time.Hour), // Expired 1 hour ago
		}

		err := storage.CreateSession(ctx, expiredSession)
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}

		// Create a valid session
		validSession := OAuthSession{
			ID:        "cleanup-session-valid",
			DID:       "did:plc:valid",
			ExpiresAt: time.Now().Add(24 * time.Hour), // Valid for 24 hours
		}

		err = storage.CreateSession(ctx, validSession)
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}

		// Run cleanup
		count, err := storage.CleanupExpiredSessions(ctx)
		if err != nil {
			t.Fatalf("CleanupExpiredSessions failed: %v", err)
		}

		if count < 1 {
			t.Errorf("Expected at least 1 expired session cleaned, got %d", count)
		}

		// Verify expired session is deleted
		_, err = storage.GetSessionByID(ctx, "cleanup-session-expired")
		if err != sql.ErrNoRows {
			t.Errorf("Expected expired session to be deleted, got error: %v", err)
		}

		// Verify valid session still exists
		retrieved, err := storage.GetSessionByID(ctx, "cleanup-session-valid")
		if err != nil {
			t.Errorf("Expected valid session to still exist, got error: %v", err)
		}
		if retrieved.ID != "cleanup-session-valid" {
			t.Errorf("Valid session ID mismatch: got %s", retrieved.ID)
		}

		// Cleanup test data
		storage.DeleteSession(ctx, "cleanup-session-valid")
	})
}

// TestCleanupWorkerCancellation verifies worker stops on context cancellation
func TestCleanupWorkerCancellation(t *testing.T) {
	dbConn := setupTestDB(t)
	defer dbConn.Close()

	storage := NewStorage(dbConn)

	t.Run("stops when context is cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		// Start worker with very short interval
		done := make(chan bool)
		go func() {
			StartCleanupWorker(ctx, storage, 10*time.Millisecond)
			done <- true
		}()

		// Let it run for a bit
		time.Sleep(50 * time.Millisecond)

		// Cancel context
		cancel()

		// Wait for worker to stop (with timeout)
		select {
		case <-done:
			// Success - worker stopped
		case <-time.After(500 * time.Millisecond):
			t.Error("Worker did not stop within timeout after context cancellation")
		}
	})
}
