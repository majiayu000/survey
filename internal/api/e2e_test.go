//go:build e2e

package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/openmeet-team/survey/internal/db"
	"github.com/openmeet-team/survey/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	testcontainers "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// setupTestServer creates a real Echo server with a PostgreSQL testcontainer
func setupTestServer(t *testing.T) (*echo.Echo, *Handlers, func()) {
	ctx := context.Background()

	// Start Postgres container
	postgresC, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("survey_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second)),
	)
	require.NoError(t, err, "Failed to start PostgreSQL container")

	// Get connection string
	connStr, err := postgresC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err, "Failed to get connection string")

	// Connect to database
	dbConn, err := sql.Open("pgx", connStr)
	require.NoError(t, err, "Failed to open database connection")

	// Wait for connection to be ready
	err = dbConn.PingContext(ctx)
	require.NoError(t, err, "Failed to ping database")

	// Run migrations
	migrationSQL, err := os.ReadFile("../../internal/db/migrations/001_initial.up.sql")
	require.NoError(t, err, "Failed to read migration file")

	_, err = dbConn.ExecContext(ctx, string(migrationSQL))
	require.NoError(t, err, "Failed to run migrations")

	// Create queries and handlers
	queries := db.NewQueries(dbConn)
	handlers := NewHandlers(queries)

	// Setup Echo server
	e := echo.New()
	SetupRoutes(e, handlers, NewHealthHandlers(dbConn), nil, dbConn)

	cleanup := func() {
		dbConn.Close()
		if err := postgresC.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	}

	return e, handlers, cleanup
}

// TestE2E_CreateAndListSurveys tests the full flow of creating and listing surveys
func TestE2E_CreateAndListSurveys(t *testing.T) {
	e, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Step 1: POST /api/v1/surveys with YAML definition
	yamlDef := `
questions:
  - id: q1
    text: What is your favorite color?
    type: single
    required: true
    options:
      - id: red
        text: Red
      - id: blue
        text: Blue
      - id: green
        text: Green
anonymous: false
`

	createReq := CreateSurveyRequest{
		Slug:       "favorite-color",
		Definition: yamlDef,
	}
	body, err := json.Marshal(createReq)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/surveys", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var createResp SurveyResponse
	err = json.Unmarshal(rec.Body.Bytes(), &createResp)
	require.NoError(t, err)
	assert.Equal(t, "favorite-color", createResp.Slug)
	assert.Equal(t, "What is your favorite color?", createResp.Title)
	assert.Len(t, createResp.Definition.Questions, 1)

	// Step 2: GET /api/v1/surveys - verify survey appears in list
	req = httptest.NewRequest(http.MethodGet, "/api/v1/surveys", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var listResp []SurveyListResponse
	err = json.Unmarshal(rec.Body.Bytes(), &listResp)
	require.NoError(t, err)
	assert.Len(t, listResp, 1)
	assert.Equal(t, "favorite-color", listResp[0].Slug)

	// Step 3: GET /api/v1/surveys/:slug - verify full survey returned
	req = httptest.NewRequest(http.MethodGet, "/api/v1/surveys/favorite-color", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var getResp SurveyResponse
	err = json.Unmarshal(rec.Body.Bytes(), &getResp)
	require.NoError(t, err)
	assert.Equal(t, "favorite-color", getResp.Slug)
	assert.Equal(t, "What is your favorite color?", getResp.Definition.Questions[0].Text)
	assert.Len(t, getResp.Definition.Questions[0].Options, 3)
}

// TestE2E_SubmitResponse tests submitting a valid response to a survey
func TestE2E_SubmitResponse(t *testing.T) {
	e, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Step 1: Create a survey
	jsonDef := `{
		"questions": [
			{
				"id": "q1",
				"text": "Which days work for you?",
				"type": "multi",
				"required": true,
				"options": [
					{"id": "mon", "text": "Monday"},
					{"id": "tue", "text": "Tuesday"},
					{"id": "wed", "text": "Wednesday"}
				]
			}
		],
		"anonymous": true
	}`

	createReq := CreateSurveyRequest{
		Slug:       "meeting-days",
		Definition: jsonDef,
	}
	body, err := json.Marshal(createReq)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/surveys", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var createResp SurveyResponse
	err = json.Unmarshal(rec.Body.Bytes(), &createResp)
	require.NoError(t, err)

	// Step 2: Submit a valid response
	submitReq := SubmitResponseRequest{
		Answers: map[string]models.Answer{
			"q1": {
				SelectedOptions: []string{"mon", "wed"},
			},
		},
	}
	body, err = json.Marshal(submitReq)
	require.NoError(t, err)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/surveys/meeting-days/responses", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.RemoteAddr = "192.168.1.100:54321"
	req.Header.Set("User-Agent", "TestBrowser/1.0")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var submitResp ResponseSubmittedResponse
	err = json.Unmarshal(rec.Body.Bytes(), &submitResp)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, submitResp.ID)
	assert.Equal(t, createResp.ID, submitResp.SurveyID)

	// Step 3: GET /api/v1/surveys/:slug/results - verify vote counted
	req = httptest.NewRequest(http.MethodGet, "/api/v1/surveys/meeting-days/results", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var results models.SurveyResults
	err = json.Unmarshal(rec.Body.Bytes(), &results)
	require.NoError(t, err)
	assert.Equal(t, 1, results.TotalVotes)
	assert.NotNil(t, results.QuestionResults["q1"])
	assert.Equal(t, 1, results.QuestionResults["q1"].OptionCounts["mon"])
	assert.Equal(t, 1, results.QuestionResults["q1"].OptionCounts["wed"])
	assert.Equal(t, 0, results.QuestionResults["q1"].OptionCounts["tue"])
}

// TestE2E_DuplicateVoteRejected tests that duplicate votes are rejected
func TestE2E_DuplicateVoteRejected(t *testing.T) {
	e, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Step 1: Create a survey
	jsonDef := `{
		"questions": [
			{
				"id": "q1",
				"text": "Do you agree?",
				"type": "single",
				"required": true,
				"options": [
					{"id": "yes", "text": "Yes"},
					{"id": "no", "text": "No"}
				]
			}
		],
		"anonymous": false
	}`

	createReq := CreateSurveyRequest{
		Slug:       "agreement-poll",
		Definition: jsonDef,
	}
	body, err := json.Marshal(createReq)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/surveys", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	// Step 2: Submit first response (should succeed)
	submitReq := SubmitResponseRequest{
		Answers: map[string]models.Answer{
			"q1": {SelectedOptions: []string{"yes"}},
		},
	}
	body, err = json.Marshal(submitReq)
	require.NoError(t, err)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/surveys/agreement-poll/responses", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.RemoteAddr = "10.0.0.50:12345"
	req.Header.Set("User-Agent", "Mozilla/5.0")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	// Step 3: Submit same response again from same "client" (should get 409)
	submitReq2 := SubmitResponseRequest{
		Answers: map[string]models.Answer{
			"q1": {SelectedOptions: []string{"no"}}, // Different answer, same client
		},
	}
	body, err = json.Marshal(submitReq2)
	require.NoError(t, err)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/surveys/agreement-poll/responses", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.RemoteAddr = "10.0.0.50:12345" // Same IP
	req.Header.Set("User-Agent", "Mozilla/5.0") // Same user agent
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)

	var errResp ErrorResponse
	err = json.Unmarshal(rec.Body.Bytes(), &errResp)
	require.NoError(t, err)
	assert.Contains(t, errResp.Error, "Already voted")
}

// TestE2E_InvalidAnswersRejected tests validation of survey responses
func TestE2E_InvalidAnswersRejected(t *testing.T) {
	e, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a survey with required questions
	jsonDef := `{
		"questions": [
			{
				"id": "q1",
				"text": "Required question",
				"type": "single",
				"required": true,
				"options": [
					{"id": "opt1", "text": "Option 1"},
					{"id": "opt2", "text": "Option 2"}
				]
			},
			{
				"id": "q2",
				"text": "Optional question",
				"type": "text",
				"required": false
			}
		],
		"anonymous": false
	}`

	createReq := CreateSurveyRequest{
		Slug:       "validation-test",
		Definition: jsonDef,
	}
	body, err := json.Marshal(createReq)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/surveys", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	// Test 1: Submit response missing required answer
	submitReq := SubmitResponseRequest{
		Answers: map[string]models.Answer{
			"q2": {Text: "This is optional"}, // Missing required q1
		},
	}
	body, err = json.Marshal(submitReq)
	require.NoError(t, err)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/surveys/validation-test/responses", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.RemoteAddr = "172.16.0.1:9999"
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var errResp ErrorResponse
	err = json.Unmarshal(rec.Body.Bytes(), &errResp)
	require.NoError(t, err)
	assert.Contains(t, errResp.Error, "Invalid answers")
	assert.Contains(t, errResp.Details, "required")

	// Test 2: Submit response with invalid option ID
	submitReq2 := SubmitResponseRequest{
		Answers: map[string]models.Answer{
			"q1": {SelectedOptions: []string{"invalid_option"}},
		},
	}
	body, err = json.Marshal(submitReq2)
	require.NoError(t, err)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/surveys/validation-test/responses", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.RemoteAddr = "172.16.0.2:9999"
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	err = json.Unmarshal(rec.Body.Bytes(), &errResp)
	require.NoError(t, err)
	assert.Contains(t, errResp.Error, "Invalid answers")
}

// TestE2E_SlugValidation tests slug validation and auto-generation
func TestE2E_SlugValidation(t *testing.T) {
	e, _, cleanup := setupTestServer(t)
	defer cleanup()

	baseDef := `{
		"questions": [
			{
				"id": "q1",
				"text": "Test question",
				"type": "single",
				"required": true,
				"options": [
					{"id": "a", "text": "A"},
					{"id": "b", "text": "B"}
				]
			}
		],
		"anonymous": false
	}`

	// Test 1: Try to create survey with invalid slug (spaces)
	createReq := CreateSurveyRequest{
		Slug:       "invalid slug with spaces",
		Definition: baseDef,
	}
	body, err := json.Marshal(createReq)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/surveys", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var errResp ErrorResponse
	err = json.Unmarshal(rec.Body.Bytes(), &errResp)
	require.NoError(t, err)
	assert.Contains(t, errResp.Error, "Invalid slug")

	// Test 2: Try to create survey with invalid slug (special chars)
	createReq.Slug = "invalid@slug#123"
	body, err = json.Marshal(createReq)
	require.NoError(t, err)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/surveys", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// Test 3: Create survey with valid slug - verify success
	createReq.Slug = "valid-slug-123"
	body, err = json.Marshal(createReq)
	require.NoError(t, err)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/surveys", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var createResp SurveyResponse
	err = json.Unmarshal(rec.Body.Bytes(), &createResp)
	require.NoError(t, err)
	assert.Equal(t, "valid-slug-123", createResp.Slug)

	// Test 4: Auto-generate slug when not provided
	createReq2 := CreateSurveyRequest{
		Slug:       "", // Empty slug should auto-generate
		Definition: baseDef,
	}
	body, err = json.Marshal(createReq2)
	require.NoError(t, err)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/surveys", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var createResp2 SurveyResponse
	err = json.Unmarshal(rec.Body.Bytes(), &createResp2)
	require.NoError(t, err)
	assert.NotEmpty(t, createResp2.Slug)
	assert.NoError(t, models.ValidateSlug(createResp2.Slug))
}

// TestE2E_HealthChecks tests the health and readiness endpoints
func TestE2E_HealthChecks(t *testing.T) {
	e, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Test health endpoint (liveness)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var healthResp HealthResponse
	err := json.Unmarshal(rec.Body.Bytes(), &healthResp)
	require.NoError(t, err)
	assert.Equal(t, "healthy", healthResp.Status)
	assert.Equal(t, "survey-api", healthResp.Service)

	// Test readiness endpoint (with DB check)
	req = httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var readyResp ReadinessResponse
	err = json.Unmarshal(rec.Body.Bytes(), &readyResp)
	require.NoError(t, err)
	assert.Equal(t, "ready", readyResp.Status)
	assert.Equal(t, "survey-api", readyResp.Service)
	assert.Equal(t, "healthy", readyResp.Checks["database"])
}

// TestE2E_MultipleResponses tests aggregation of multiple responses
func TestE2E_MultipleResponses(t *testing.T) {
	e, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a survey
	jsonDef := `{
		"questions": [
			{
				"id": "q1",
				"text": "Favorite programming language?",
				"type": "single",
				"required": true,
				"options": [
					{"id": "go", "text": "Go"},
					{"id": "python", "text": "Python"},
					{"id": "rust", "text": "Rust"}
				]
			}
		],
		"anonymous": true
	}`

	createReq := CreateSurveyRequest{
		Slug:       "prog-lang-poll",
		Definition: jsonDef,
	}
	body, err := json.Marshal(createReq)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/surveys", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	// Submit multiple responses from different "clients"
	votes := []struct {
		ip     string
		choice string
	}{
		{"192.168.1.1:1111", "go"},
		{"192.168.1.2:2222", "go"},
		{"192.168.1.3:3333", "python"},
		{"192.168.1.4:4444", "go"},
		{"192.168.1.5:5555", "rust"},
	}

	for i, vote := range votes {
		submitReq := SubmitResponseRequest{
			Answers: map[string]models.Answer{
				"q1": {SelectedOptions: []string{vote.choice}},
			},
		}
		body, err = json.Marshal(submitReq)
		require.NoError(t, err)

		req = httptest.NewRequest(http.MethodPost, "/api/v1/surveys/prog-lang-poll/responses", bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		req.RemoteAddr = vote.ip
		req.Header.Set("User-Agent", "TestClient/"+string(rune('A'+i)))
		rec = httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code, "Vote %d should succeed", i+1)
	}

	// Check results
	req = httptest.NewRequest(http.MethodGet, "/api/v1/surveys/prog-lang-poll/results", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var results models.SurveyResults
	err = json.Unmarshal(rec.Body.Bytes(), &results)
	require.NoError(t, err)
	assert.Equal(t, 5, results.TotalVotes)
	assert.Equal(t, 3, results.QuestionResults["q1"].OptionCounts["go"])
	assert.Equal(t, 1, results.QuestionResults["q1"].OptionCounts["python"])
	assert.Equal(t, 1, results.QuestionResults["q1"].OptionCounts["rust"])
}
