-- OAuth request state storage
-- Stores temporary state during the OAuth flow
CREATE TABLE oauth_requests (
    state TEXT PRIMARY KEY,
    issuer TEXT NOT NULL,
    pkce_verifier TEXT NOT NULL,
    dpop_private_key TEXT NOT NULL,
    destination TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);

-- Index for cleanup of expired requests
CREATE INDEX idx_oauth_requests_expires_at ON oauth_requests(expires_at);

-- OAuth sessions
-- Stores authenticated user sessions
CREATE TABLE oauth_sessions (
    id TEXT PRIMARY KEY,
    did TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at TIMESTAMPTZ
);

-- Index for DID lookups
CREATE INDEX idx_oauth_sessions_did ON oauth_sessions(did);

-- Index for session expiration cleanup
CREATE INDEX idx_oauth_sessions_expires_at ON oauth_sessions(expires_at);
