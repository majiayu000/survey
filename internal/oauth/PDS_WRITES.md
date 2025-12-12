# PDS Writes Implementation

This document describes how to write ATProto records to users' Personal Data Servers (PDS).

## Overview

After OAuth authentication, the survey service can write records directly to a user's PDS. This enables:

1. **Survey Creation** - User creates a survey, it's written to their PDS as `net.openmeet.survey`
2. **Response Submission** - User votes, response written as `net.openmeet.survey.response`
3. **Results Publishing** - When survey completes, results written as `net.openmeet.survey.results`

## Architecture

### Token Storage

The `oauth_sessions` table stores everything needed for PDS writes:

```sql
CREATE TABLE oauth_sessions (
    id TEXT PRIMARY KEY,
    did TEXT NOT NULL,
    access_token TEXT,           -- DPoP-bound access token
    refresh_token TEXT,          -- For token renewal
    dpop_key TEXT,               -- DPoP private key (JWK)
    pds_url TEXT,                -- User's PDS endpoint
    token_expires_at TIMESTAMPTZ, -- Token expiration
    created_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at TIMESTAMPTZ       -- Session cookie expiry
);
```

### OAuth Flow

1. User authenticates via Bluesky OAuth
2. Callback handler exchanges code for tokens
3. Handler resolves user's PDS URL from their DID
4. Session created with tokens, DPoP key, and PDS URL
5. Application can now write to user's PDS

## Usage

### Writing a Record

```go
import "github.com/openmeet-team/survey/internal/oauth"

// Get user's session (from cookie, database, etc.)
session, err := storage.GetSessionByID(ctx, sessionID)
if err != nil {
    return err
}

// Create survey record
surveyRecord := map[string]interface{}{
    "question": "What's your favorite color?",
    "options":  []string{"Red", "Blue", "Green"},
    "createdAt": time.Now().Format(time.RFC3339),
}

// Write to user's PDS
uri, cid, err := oauth.CreateRecord(session, "net.openmeet.survey", surveyRecord)
if err != nil {
    return fmt.Errorf("failed to write survey: %w", err)
}

// uri: "at://did:plc:abc123/net.openmeet.survey/xyz789"
// cid: "bafyreiabc123..." (content identifier)
```

### Handling Token Expiration

```go
// Check if token is expired
if session.TokenExpiresAt != nil && time.Now().After(*session.TokenExpiresAt) {
    // Refresh the token
    newAccessToken, newRefreshToken, expiresIn, err := oauth.RefreshAccessToken(
        session,
        authServerURL,  // e.g., "https://bsky.social"
        clientID,       // Your OAuth client ID
        clientKey,      // Your client signing key
    )
    if err != nil {
        return fmt.Errorf("token refresh failed: %w", err)
    }

    // Update session in database
    newExpiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)
    err = storage.UpdateSessionTokens(ctx, session.ID, newAccessToken, newRefreshToken, &newExpiresAt)
    if err != nil {
        return fmt.Errorf("failed to update session: %w", err)
    }

    // Update local session object
    session.AccessToken = newAccessToken
    session.RefreshToken = newRefreshToken
    session.TokenExpiresAt = &newExpiresAt
}

// Now proceed with CreateRecord
uri, cid, err := oauth.CreateRecord(session, "net.openmeet.survey", record)
```

### Complete Example

```go
func CreateSurvey(ctx context.Context, sessionID string, question string, options []string) (string, error) {
    storage := oauth.NewStorage(db)

    // Get user session
    session, err := storage.GetSessionByID(ctx, sessionID)
    if err != nil {
        return "", fmt.Errorf("session not found: %w", err)
    }

    // Check token expiration and refresh if needed
    if session.TokenExpiresAt != nil && time.Now().After(*session.TokenExpiresAt) {
        if session.RefreshToken == "" {
            return "", fmt.Errorf("token expired and no refresh token available")
        }

        newAccessToken, newRefreshToken, expiresIn, err := oauth.RefreshAccessToken(
            session,
            "https://bsky.social", // Get from config
            config.ClientID,
            config.ClientKey,
        )
        if err != nil {
            return "", err
        }

        newExpiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)
        err = storage.UpdateSessionTokens(ctx, session.ID, newAccessToken, newRefreshToken, &newExpiresAt)
        if err != nil {
            return "", err
        }

        session.AccessToken = newAccessToken
        session.RefreshToken = newRefreshToken
        session.TokenExpiresAt = &newExpiresAt
    }

    // Create survey record
    surveyRecord := map[string]interface{}{
        "$type":    "net.openmeet.survey",
        "question": question,
        "options":  options,
        "createdAt": time.Now().Format(time.RFC3339),
    }

    // Write to PDS
    uri, cid, err := oauth.CreateRecord(session, "net.openmeet.survey", surveyRecord)
    if err != nil {
        return "", fmt.Errorf("failed to create survey: %w", err)
    }

    log.Printf("Survey created: %s (CID: %s)", uri, cid)
    return uri, nil
}
```

## Record Collections

### net.openmeet.survey

Survey definition record:

```json
{
  "$type": "net.openmeet.survey",
  "question": "What's your favorite color?",
  "options": ["Red", "Blue", "Green"],
  "createdAt": "2025-12-12T10:00:00Z",
  "endsAt": "2025-12-19T10:00:00Z"
}
```

### net.openmeet.survey.response

User's vote on a survey:

```json
{
  "$type": "net.openmeet.survey.response",
  "survey": "at://did:plc:creator/net.openmeet.survey/abc123",
  "option": "Blue",
  "createdAt": "2025-12-12T11:30:00Z"
}
```

### net.openmeet.survey.results

Survey results (published when survey ends):

```json
{
  "$type": "net.openmeet.survey.results",
  "survey": "at://did:plc:creator/net.openmeet.survey/abc123",
  "results": {
    "Red": 5,
    "Blue": 12,
    "Green": 8
  },
  "totalVotes": 25,
  "publishedAt": "2025-12-19T10:00:00Z"
}
```

## Security Considerations

1. **DPoP Binding** - Access tokens are bound to the DPoP key pair. Each request includes a DPoP proof JWT.

2. **Token Storage** - Access tokens and DPoP keys are stored in the database. Consider encrypting at rest.

3. **Token Refresh** - Refresh tokens allow obtaining new access tokens without re-authentication.

4. **Session Expiry** - Session cookies expire after 24 hours. Token expiry is separate (typically 1 hour).

5. **PDS URL Resolution** - PDS URL is resolved from the DID document and stored. If user migrates PDS, need to re-resolve.

## Error Handling

The `CreateRecord` function handles common errors:

- **Missing credentials** - Returns error if access token, DPoP key, or PDS URL missing
- **Expired token** - Returns error suggesting token refresh
- **DPoP nonce** - Automatically retries with nonce if required by PDS
- **Network errors** - Returns wrapped error with context
- **PDS errors** - Returns status code and response body

Example error handling:

```go
uri, cid, err := oauth.CreateRecord(session, "net.openmeet.survey", record)
if err != nil {
    if strings.Contains(err.Error(), "expired") {
        // Try refreshing token
        return handleTokenRefresh(session)
    }
    if strings.Contains(err.Error(), "status 401") {
        // Unauthorized - may need to re-authenticate
        return promptReLogin()
    }
    // Other error
    return fmt.Errorf("PDS write failed: %w", err)
}
```

## Testing

Comprehensive tests cover:

- ✅ Successful record creation
- ✅ Error cases (nil session, missing tokens, missing PDS URL)
- ✅ Token expiration handling
- ✅ Token refresh flow
- ✅ DPoP nonce retry
- ✅ Session token updates

Run tests:

```bash
cd /home/tscanlan/projects/openmeet/survey
go test ./internal/oauth -v -run "TestCreateRecord|TestRefreshToken"
```

## Next Steps

1. **Implement Survey Creation** - Wire up CreateRecord in survey creation handler
2. **Response Submission** - Write responses to PDS when users vote
3. **Results Publishing** - Aggregate and publish results when survey completes
4. **Token Refresh Middleware** - Auto-refresh tokens before requests
5. **Encryption** - Encrypt tokens at rest in database
6. **PDS Migration** - Handle user PDS changes
7. **Lexicon Validation** - Validate records match lexicon schemas before writing
