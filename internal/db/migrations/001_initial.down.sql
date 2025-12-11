-- Rollback initial schema
-- Drop tables in reverse dependency order

DROP TABLE IF EXISTS responses;
DROP TABLE IF EXISTS surveys;
DROP TABLE IF EXISTS jetstream_cursor;

-- Note: We don't drop pgcrypto extension as other databases might use it
