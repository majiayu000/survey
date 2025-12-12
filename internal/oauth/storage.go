package oauth

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// OAuthRequest represents a pending OAuth request
type OAuthRequest struct {
	State          string
	Issuer         string
	PKCEVerifier   string
	DPoPPrivateKey string
	Destination    string
	CreatedAt      time.Time
	ExpiresAt      time.Time
}

// OAuthSession represents an authenticated user session
type OAuthSession struct {
	ID        string
	DID       string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Storage provides database operations for OAuth
type Storage struct {
	db *sql.DB
}

// NewStorage creates a new Storage instance
func NewStorage(db *sql.DB) *Storage {
	return &Storage{db: db}
}

// SaveOAuthRequest stores an OAuth request state
func (s *Storage) SaveOAuthRequest(ctx context.Context, req OAuthRequest) error {
	query := `
		INSERT INTO oauth_requests (state, issuer, pkce_verifier, dpop_private_key, destination, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := s.db.ExecContext(
		ctx,
		query,
		req.State,
		req.Issuer,
		req.PKCEVerifier,
		req.DPoPPrivateKey,
		req.Destination,
		req.ExpiresAt,
	)

	if err != nil {
		return fmt.Errorf("failed to save OAuth request: %w", err)
	}

	return nil
}

// GetOAuthRequest retrieves an OAuth request by state
func (s *Storage) GetOAuthRequest(ctx context.Context, state string) (*OAuthRequest, error) {
	query := `
		SELECT state, issuer, pkce_verifier, dpop_private_key, destination, created_at, expires_at
		FROM oauth_requests
		WHERE state = $1
	`

	req := &OAuthRequest{}
	err := s.db.QueryRowContext(ctx, query, state).Scan(
		&req.State,
		&req.Issuer,
		&req.PKCEVerifier,
		&req.DPoPPrivateKey,
		&req.Destination,
		&req.CreatedAt,
		&req.ExpiresAt,
	)

	if err != nil {
		return nil, err
	}

	return req, nil
}

// DeleteOAuthRequest removes an OAuth request by state
func (s *Storage) DeleteOAuthRequest(ctx context.Context, state string) error {
	query := `DELETE FROM oauth_requests WHERE state = $1`

	_, err := s.db.ExecContext(ctx, query, state)
	if err != nil {
		return fmt.Errorf("failed to delete OAuth request: %w", err)
	}

	return nil
}

// CreateSession creates a new OAuth session
func (s *Storage) CreateSession(ctx context.Context, session OAuthSession) error {
	query := `
		INSERT INTO oauth_sessions (id, did, expires_at)
		VALUES ($1, $2, $3)
	`

	_, err := s.db.ExecContext(
		ctx,
		query,
		session.ID,
		session.DID,
		session.ExpiresAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	return nil
}

// GetSessionByID retrieves a session by its ID
func (s *Storage) GetSessionByID(ctx context.Context, id string) (*OAuthSession, error) {
	query := `
		SELECT id, did, created_at, expires_at
		FROM oauth_sessions
		WHERE id = $1
	`

	session := &OAuthSession{}
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&session.ID,
		&session.DID,
		&session.CreatedAt,
		&session.ExpiresAt,
	)

	if err != nil {
		return nil, err
	}

	return session, nil
}

// DeleteSession removes a session by ID
func (s *Storage) DeleteSession(ctx context.Context, id string) error {
	query := `DELETE FROM oauth_sessions WHERE id = $1`

	_, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}

// CleanupExpiredRequests removes expired OAuth requests
func (s *Storage) CleanupExpiredRequests(ctx context.Context) (int64, error) {
	query := `DELETE FROM oauth_requests WHERE expires_at < NOW()`

	result, err := s.db.ExecContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup expired requests: %w", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return count, nil
}

// CleanupExpiredSessions removes expired sessions
func (s *Storage) CleanupExpiredSessions(ctx context.Context) (int64, error) {
	query := `DELETE FROM oauth_sessions WHERE expires_at < NOW()`

	result, err := s.db.ExecContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup expired sessions: %w", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return count, nil
}
