# Survey Service

ATProto-native survey application. See [README.md](README.md) for features, endpoints, and setup.

## Quick Reference

```bash
# Build
make build              # Runs templ generate + go build

# Test
go test ./...           # All tests
go test -tags=e2e ./... # E2E tests (requires Docker)

# Run locally
./bin/survey-api        # Requires .env with DB connection
```

## Code Patterns

### HTML Handler Pattern

```go
func (h *Handlers) SomePageHTML(c echo.Context) error {
    user, profile := getUserAndProfile(c)
    data, err := h.queries.GetSomething(c.Request().Context())

    c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
    return templates.SomePage(data, user, profile).Render(c.Request().Context(), c.Response().Writer)
}
```

### Auth-Required Handler

```go
user := oauth.GetUser(c)
if user == nil {
    return c.String(http.StatusUnauthorized, "Authentication required")
}
session, err := oauth.GetSession(c, h.oauthStorage)
if err != nil || session == nil {
    return c.String(http.StatusUnauthorized, "Session not found")
}
```

### Test Setup Pattern

```go
func setupTest() (*echo.Echo, *MockQueries, *Handlers) {
    e := echo.New()
    mq := NewMockQueries()
    h := &Handlers{queries: mq}
    return e, mq, h
}
```

## PDS Operations

```go
// Create (requires DPoP auth)
oauth.CreateRecord(session, "net.openmeet.survey", rkey, record)

// List (public, no auth)
oauth.ListRecords(pdsURL, did, "net.openmeet.survey", cursor, limit)

// Delete (requires DPoP auth)
oauth.DeleteRecord(session, "net.openmeet.survey", rkey)
```

### DPoP Authentication

PDS writes require DPoP (Demonstration of Proof-of-Possession):
1. Generate proof: `CreateDPoPProof(key, method, url, nonce, accessToken)`
2. Headers: `Authorization: DPoP {token}`, `DPoP: {proof}`
3. Handle `use_dpop_nonce` challenges

## Anonymous vs Authenticated Voting

| Type | Storage | Location |
|------|---------|----------|
| Authenticated (DID) | `voter_did` | User's PDS + local DB |
| Anonymous | `voter_session` (SHA256 hash) | Local DB only |

## AI Generation Patterns

### Generator Usage

The `generator` package wraps langchaingo's LLM interface with built-in validation, sanitization, and cost limiting. Initialize with any langchaingo-compatible LLM (OpenAI, Anthropic, Ollama, etc.) and call `Generate(ctx, prompt)`. The generator automatically validates input, calls the LLM, sanitizes output, validates against schema, and checks cost limits.

### Handler Pattern

AI generation handlers should: check consent checkbox, apply rate limits (DID-based for authenticated, IP-based for anonymous), time the generation call, record Prometheus metrics (duration, tokens, cost, status), and return specific error responses for rate limiting, budget exceeded, and validation failures.

### Testing

Use langchaingo's `fake.NewFakeLLM()` to return canned JSON responses without making real API calls. This works for both unit tests and integration tests.

### Metrics

Always instrument AI endpoints with five metric types: `survey_ai_generations_total` (with status labels), `survey_ai_generation_duration_seconds`, `survey_ai_tokens_total` (input/output labels), `survey_ai_daily_cost_usd`, and `survey_ai_rate_limit_hits_total` (user_type labels). Record metrics immediately after generation attempt, before returning response.

## Gotchas

**Template changes not showing:** Run `make templ` after editing `.templ` files.

**PDS write failures:**
- Token expired (check `session.TokenExpiresAt`)
- Missing DPoP key
- Invalid collection name (must be `a.b.c` format)
- Need `validate: false` for custom lexicons

**JSON in templates:** Use `record.ValueJSON` not `fmt.Sprintf("%v", record.Value)`.

**List endpoints removed:** `GET /api/v1/surveys` returns 404 intentionally. Access surveys via `/surveys/:slug` only.

**AI generation disabled:** If `OPENAI_API_KEY` is not set, `/api/v1/surveys/generate` returns 503. This is expected - AI is optional.
