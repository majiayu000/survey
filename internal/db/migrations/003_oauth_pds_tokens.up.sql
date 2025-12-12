-- Add token storage for PDS writes
-- Extends oauth_sessions to store access tokens, refresh tokens, and DPoP keys

ALTER TABLE oauth_sessions
ADD COLUMN access_token TEXT,
ADD COLUMN refresh_token TEXT,
ADD COLUMN dpop_key TEXT,
ADD COLUMN pds_url TEXT,
ADD COLUMN token_expires_at TIMESTAMPTZ;

-- Index for token expiration (for refresh logic)
CREATE INDEX idx_oauth_sessions_token_expires_at ON oauth_sessions(token_expires_at);
