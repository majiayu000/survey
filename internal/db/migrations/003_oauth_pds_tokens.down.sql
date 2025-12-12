-- Remove PDS token columns from oauth_sessions

DROP INDEX IF EXISTS idx_oauth_sessions_token_expires_at;

ALTER TABLE oauth_sessions
DROP COLUMN IF EXISTS access_token,
DROP COLUMN IF EXISTS refresh_token,
DROP COLUMN IF EXISTS dpop_key,
DROP COLUMN IF EXISTS pds_url,
DROP COLUMN IF EXISTS token_expires_at;
