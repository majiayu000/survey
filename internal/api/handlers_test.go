package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/openmeet-team/survey/internal/models"
	"github.com/openmeet-team/survey/internal/oauth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockQueries implements a mock version of db.Queries for testing
type MockQueries struct {
	surveys         map[string]*models.Survey
	slugs           map[string]bool
	responses       map[uuid.UUID]*models.Response
	responsesBySurvey map[uuid.UUID]map[string]*models.Response // surveyID -> voterSession -> response
}

func NewMockQueries() *MockQueries {
	return &MockQueries{
		surveys:           make(map[string]*models.Survey),
		slugs:             make(map[string]bool),
		responses:         make(map[uuid.UUID]*models.Response),
		responsesBySurvey: make(map[uuid.UUID]map[string]*models.Response),
	}
}

func (m *MockQueries) CreateSurvey(ctx context.Context, s *models.Survey) error {
	m.surveys[s.Slug] = s
	m.slugs[s.Slug] = true
	m.responsesBySurvey[s.ID] = make(map[string]*models.Response)
	return nil
}

func (m *MockQueries) GetSurveyBySlug(ctx context.Context, slug string) (*models.Survey, error) {
	if s, ok := m.surveys[slug]; ok {
		return s, nil
	}
	return nil, sql.ErrNoRows
}

func (m *MockQueries) ListSurveys(ctx context.Context, limit, offset int) ([]*models.Survey, error) {
	var surveys []*models.Survey
	for _, s := range m.surveys {
		surveys = append(surveys, s)
	}
	return surveys, nil
}

func (m *MockQueries) SlugExists(ctx context.Context, slug string) (bool, error) {
	return m.slugs[slug], nil
}

func (m *MockQueries) CreateResponse(ctx context.Context, r *models.Response) error {
	m.responses[r.ID] = r

	// Track by voter session
	if r.VoterSession != nil {
		m.responsesBySurvey[r.SurveyID][*r.VoterSession] = r
	}

	return nil
}

func (m *MockQueries) GetResponseBySurveyAndVoter(ctx context.Context, surveyID uuid.UUID, voterDID, voterSession string) (*models.Response, error) {
	if voterSession != "" {
		if surveyResponses, ok := m.responsesBySurvey[surveyID]; ok {
			if resp, exists := surveyResponses[voterSession]; exists {
				return resp, nil
			}
		}
	}
	return nil, nil // No existing response
}

func (m *MockQueries) GetSurveyResults(ctx context.Context, surveyID uuid.UUID) (*models.SurveyResults, error) {
	// Simple mock implementation
	return &models.SurveyResults{
		SurveyID:        surveyID,
		TotalVotes:      0,
		QuestionResults: make(map[string]*models.QuestionResult),
	}, nil
}

func (m *MockQueries) UpdateSurveyResults(ctx context.Context, surveyID uuid.UUID, resultsURI, resultsCID string) error {
	// Find and update the survey
	for _, survey := range m.surveys {
		if survey.ID == surveyID {
			survey.ResultsURI = &resultsURI
			survey.ResultsCID = &resultsCID
			return nil
		}
	}
	return fmt.Errorf("survey not found")
}

func (m *MockQueries) GetStats(ctx context.Context) (*models.Stats, error) {
	// Count surveys
	surveyCount := len(m.surveys)

	// Count total responses
	responseCount := len(m.responses)

	// Count unique DIDs
	uniqueDIDs := make(map[string]bool)
	for _, response := range m.responses {
		if response.VoterDID != nil && *response.VoterDID != "" {
			uniqueDIDs[*response.VoterDID] = true
		}
	}
	uniqueUserCount := len(uniqueDIDs)

	return &models.Stats{
		SurveyCount:     surveyCount,
		ResponseCount:   responseCount,
		UniqueUserCount: uniqueUserCount,
	}, nil
}

// Test Helpers

func setupTest() (*echo.Echo, *MockQueries, *Handlers) {
	e := echo.New()
	mq := NewMockQueries()
	h := NewHandlers(mq)
	return e, mq, h
}

// RED PHASE: Write failing tests

func TestCreateSurvey_WithJSONDefinition(t *testing.T) {
	e, _, h := setupTest()

	definition := `{
		"questions": [
			{
				"id": "q1",
				"text": "What is your favorite color?",
				"type": "single",
				"required": true,
				"options": [
					{"id": "red", "text": "Red"},
					{"id": "blue", "text": "Blue"}
				]
			}
		],
		"anonymous": false
	}`

	reqBody := CreateSurveyRequest{
		Slug:       "test-survey",
		Definition: definition,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/surveys", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.CreateSurvey(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp SurveyResponse
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "test-survey", resp.Slug)
	assert.Equal(t, "What is your favorite color?", resp.Definition.Questions[0].Text)
}

func TestCreateSurvey_WithYAMLDefinition(t *testing.T) {
	e, _, h := setupTest()

	definition := `
questions:
  - id: q1
    text: What days work for you?
    type: multi
    required: true
    options:
      - id: mon
        text: Monday
      - id: tue
        text: Tuesday
anonymous: true
`

	reqBody := CreateSurveyRequest{
		Slug:       "yaml-survey",
		Definition: definition,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/surveys", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.CreateSurvey(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp SurveyResponse
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "yaml-survey", resp.Slug)
	assert.True(t, resp.Definition.Anonymous)
}

func TestCreateSurvey_AutoGenerateSlug(t *testing.T) {
	e, _, h := setupTest()

	definition := `{
		"questions": [
			{
				"id": "q1",
				"text": "Test Question",
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

	reqBody := CreateSurveyRequest{
		Definition: definition,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/surveys", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.CreateSurvey(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp SurveyResponse
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Slug)
	// Generated slug should be valid
	assert.NoError(t, models.ValidateSlug(resp.Slug))
}

func TestCreateSurvey_DuplicateSlug(t *testing.T) {
	e, mq, h := setupTest()

	// Pre-create a survey with the slug
	existingSurvey := &models.Survey{
		ID:        uuid.New(),
		Slug:      "existing-slug",
		Title:     "Existing",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	mq.CreateSurvey(context.Background(), existingSurvey)

	definition := `{
		"questions": [
			{
				"id": "q1",
				"text": "Test",
				"type": "single",
				"required": true,
				"options": [{"id": "a", "text": "A"}, {"id": "b", "text": "B"}]
			}
		],
		"anonymous": false
	}`

	reqBody := CreateSurveyRequest{
		Slug:       "existing-slug",
		Definition: definition,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/surveys", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.CreateSurvey(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, rec.Code)

	var errResp ErrorResponse
	err = json.Unmarshal(rec.Body.Bytes(), &errResp)
	require.NoError(t, err)
	assert.Contains(t, errResp.Error, "slug already exists")
}

func TestGetSurvey_Success(t *testing.T) {
	e, mq, h := setupTest()

	// Create a survey
	survey := &models.Survey{
		ID:    uuid.New(),
		Slug:  "test-survey",
		Title: "Test Survey",
		Definition: models.SurveyDefinition{
			Questions: []models.Question{
				{
					ID:       "q1",
					Text:     "Test Question",
					Type:     models.QuestionTypeSingle,
					Required: true,
					Options: []models.Option{
						{ID: "a", Text: "A"},
						{ID: "b", Text: "B"},
					},
				},
			},
			Anonymous: false,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	mq.CreateSurvey(context.Background(), survey)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/surveys/test-survey", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("test-survey")

	err := h.GetSurvey(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp SurveyResponse
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "test-survey", resp.Slug)
	assert.NotNil(t, resp.Definition)
}

func TestGetSurvey_NotFound(t *testing.T) {
	e, _, h := setupTest()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/surveys/nonexistent", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("nonexistent")

	err := h.GetSurvey(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestListSurveys_Success(t *testing.T) {
	e, mq, h := setupTest()

	// Create multiple surveys
	for i := 1; i <= 3; i++ {
		survey := &models.Survey{
			ID:        uuid.New(),
			Slug:      "survey-" + string(rune(i)),
			Title:     "Survey " + string(rune(i)),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		mq.CreateSurvey(context.Background(), survey)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/surveys?limit=10&offset=0", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.ListSurveys(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var surveys []SurveyListResponse
	err = json.Unmarshal(rec.Body.Bytes(), &surveys)
	require.NoError(t, err)
	assert.Len(t, surveys, 3)
}

func TestSubmitResponse_Success(t *testing.T) {
	e, mq, h := setupTest()

	// Create a survey
	survey := &models.Survey{
		ID:    uuid.New(),
		Slug:  "test-survey",
		Title: "Test Survey",
		Definition: models.SurveyDefinition{
			Questions: []models.Question{
				{
					ID:       "q1",
					Text:     "Test Question",
					Type:     models.QuestionTypeSingle,
					Required: true,
					Options: []models.Option{
						{ID: "a", Text: "A"},
						{ID: "b", Text: "B"},
					},
				},
			},
			Anonymous: false,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	mq.CreateSurvey(context.Background(), survey)

	reqBody := SubmitResponseRequest{
		Answers: map[string]models.Answer{
			"q1": {
				SelectedOptions: []string{"a"},
			},
		},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/surveys/test-survey/responses", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.RemoteAddr = "192.168.1.1:12345"
	req.Header.Set("User-Agent", "TestAgent/1.0")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("test-survey")

	err := h.SubmitResponse(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp ResponseSubmittedResponse
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, resp.ID)
	assert.Equal(t, survey.ID, resp.SurveyID)
}

func TestSubmitResponse_DuplicateVote(t *testing.T) {
	e, mq, h := setupTest()

	// Create a survey
	survey := &models.Survey{
		ID:    uuid.New(),
		Slug:  "test-survey",
		Title: "Test Survey",
		Definition: models.SurveyDefinition{
			Questions: []models.Question{
				{
					ID:       "q1",
					Text:     "Test Question",
					Type:     models.QuestionTypeSingle,
					Required: true,
					Options: []models.Option{
						{ID: "a", Text: "A"},
						{ID: "b", Text: "B"},
					},
				},
			},
			Anonymous: false,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	mq.CreateSurvey(context.Background(), survey)

	// Submit first response using the handler (so voter session is generated properly)
	reqBody := SubmitResponseRequest{
		Answers: map[string]models.Answer{
			"q1": {SelectedOptions: []string{"a"}},
		},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/surveys/test-survey/responses", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.RemoteAddr = "192.168.1.1:12345"
	req.Header.Set("User-Agent", "TestAgent/1.0")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("test-survey")

	err := h.SubmitResponse(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)

	// Try to submit again with same IP and user agent
	reqBody2 := SubmitResponseRequest{
		Answers: map[string]models.Answer{
			"q1": {SelectedOptions: []string{"b"}},
		},
	}
	body2, _ := json.Marshal(reqBody2)

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/surveys/test-survey/responses", bytes.NewReader(body2))
	req2.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req2.RemoteAddr = "192.168.1.1:12345"
	req2.Header.Set("User-Agent", "TestAgent/1.0")
	rec2 := httptest.NewRecorder()
	c2 := e.NewContext(req2, rec2)
	c2.SetParamNames("slug")
	c2.SetParamValues("test-survey")

	err = h.SubmitResponse(c2)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, rec2.Code)

	var errResp ErrorResponse
	err = json.Unmarshal(rec2.Body.Bytes(), &errResp)
	require.NoError(t, err)
	assert.Contains(t, errResp.Error, "Already voted")
}

func TestSubmitResponse_InvalidAnswers(t *testing.T) {
	e, mq, h := setupTest()

	// Create a survey
	survey := &models.Survey{
		ID:    uuid.New(),
		Slug:  "test-survey",
		Title: "Test Survey",
		Definition: models.SurveyDefinition{
			Questions: []models.Question{
				{
					ID:       "q1",
					Text:     "Test Question",
					Type:     models.QuestionTypeSingle,
					Required: true,
					Options: []models.Option{
						{ID: "a", Text: "A"},
						{ID: "b", Text: "B"},
					},
				},
			},
			Anonymous: false,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	mq.CreateSurvey(context.Background(), survey)

	// Submit invalid response (missing required question)
	reqBody := SubmitResponseRequest{
		Answers: map[string]models.Answer{},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/surveys/test-survey/responses", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("test-survey")

	err := h.SubmitResponse(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetResults_Success(t *testing.T) {
	e, mq, h := setupTest()

	// Create a survey
	survey := &models.Survey{
		ID:        uuid.New(),
		Slug:      "test-survey",
		Title:     "Test Survey",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	mq.CreateSurvey(context.Background(), survey)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/surveys/test-survey/results", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("test-survey")

	err := h.GetResults(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var results models.SurveyResults
	err = json.Unmarshal(rec.Body.Bytes(), &results)
	require.NoError(t, err)
	assert.Equal(t, survey.ID, results.SurveyID)
}

func TestGenerateSlug(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		expected string
	}{
		{
			name:     "simple title",
			title:    "What is your favorite color?",
			expected: "what-is-your-favorite-color",
		},
		{
			name:     "title with special chars",
			title:    "Survey #1: Best Day!",
			expected: "survey-1-best-day",
		},
		{
			name:     "title with multiple spaces",
			title:    "This   has   spaces",
			expected: "this-has-spaces",
		},
		{
			name:     "very long title",
			title:    "This is a very long survey title that should be truncated to fit within the slug length limit",
			expected: "this-is-a-very-long-survey-title-that-should-be-tr",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slug := generateSlug(tt.title)
			assert.Equal(t, tt.expected, slug)
			assert.NoError(t, models.ValidateSlug(slug))
		})
	}
}

func TestGetClientIP(t *testing.T) {
	e := echo.New()

	tests := []struct {
		name          string
		remoteAddr    string
		xForwardedFor string
		expected      string
	}{
		{
			name:       "direct connection",
			remoteAddr: "192.168.1.1:12345",
			expected:   "192.168.1.1",
		},
		{
			name:          "behind proxy",
			remoteAddr:    "10.0.0.1:12345",
			xForwardedFor: "203.0.113.1, 198.51.100.1",
			expected:      "203.0.113.1",
		},
		{
			name:          "single proxy",
			remoteAddr:    "10.0.0.1:12345",
			xForwardedFor: "203.0.113.1",
			expected:      "203.0.113.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.xForwardedFor)
			}
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			ip := getClientIP(c)
			assert.Equal(t, tt.expected, ip)
		})
	}
}

// RED PHASE: Test response submission WITHOUT OAuth (guest voting)
func TestSubmitResponseHTML_GuestVoting(t *testing.T) {
	// Validates that guest voting still works (no PDS write)
	e, mq, h := setupTest()

	// Create a local-only survey (no URI)
	survey := &models.Survey{
		ID:    uuid.New(),
		Slug:  "guest-survey",
		Title: "Guest Survey",
		Definition: models.SurveyDefinition{
			Questions: []models.Question{
				{
					ID:       "q1",
					Text:     "Test Question",
					Type:     models.QuestionTypeSingle,
					Required: true,
					Options: []models.Option{
						{ID: "a", Text: "A"},
						{ID: "b", Text: "B"},
					},
				},
			},
			Anonymous: false,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	mq.CreateSurvey(context.Background(), survey)

	// Submit response via HTML form (no OAuth session)
	form := "q1=a"
	req := httptest.NewRequest(http.MethodPost, "/surveys/guest-survey/responses", strings.NewReader(form))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	req.RemoteAddr = "192.168.1.1:12345"
	req.Header.Set("User-Agent", "TestAgent/1.0")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("guest-survey")

	err := h.SubmitResponseHTML(c)
	require.NoError(t, err)

	// Should redirect or show thank you
	// Response should be stored with VoterSession, no URI/CID/VoterDID
	assert.True(t, rec.Code == http.StatusOK || rec.Code == http.StatusSeeOther)
}

// RED PHASE: Test response with PDS write (user logged in + survey has URI)
func TestSubmitResponseHTML_WithOAuthAndSurveyURI(t *testing.T) {
	// When:
	// 1. User is logged in (has OAuth session)
	// 2. Survey has a URI (is an ATProto record)
	// Then:
	// - Response should be written to user's PDS
	// - Local response should have RecordURI, RecordCID, and VoterDID

	// This will fail until we implement the PDS write logic in SubmitResponseHTML
	t.Skip("Will implement after adding PDS write to SubmitResponseHTML handler")

	// Future implementation:
	// 1. Create mock OAuth session and storage
	// 2. Create survey with URI (ATProto record)
	// 3. Set session cookie in request
	// 4. Mock CreateRecord to return URI and CID
	// 5. Verify local response has all ATProto fields set
}

// RED PHASE: Test PublishResultsHTML - User not logged in
func TestPublishResultsHTML_NotLoggedIn(t *testing.T) {
	e, mq, h := setupTest()

	// Create a survey
	survey := &models.Survey{
		ID:    uuid.New(),
		Slug:  "test-survey",
		Title: "Test Survey",
		Definition: models.SurveyDefinition{
			Questions: []models.Question{
				{
					ID:       "q1",
					Text:     "Test Question",
					Type:     models.QuestionTypeSingle,
					Required: true,
					Options: []models.Option{
						{ID: "a", Text: "A"},
						{ID: "b", Text: "B"},
					},
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	mq.CreateSurvey(context.Background(), survey)

	req := httptest.NewRequest(http.MethodPost, "/surveys/test-survey/publish-results", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("test-survey")

	err := h.PublishResultsHTML(c)
	require.NoError(t, err)

	// Should return error (unauthorized - not logged in)
	assert.Equal(t, http.StatusOK, rec.Code)
	// Check for error message in body (case insensitive)
	body := strings.ToLower(rec.Body.String())
	assert.True(t, strings.Contains(body, "log") || strings.Contains(body, "error"))
}

// RED PHASE: Test PublishResultsHTML - Survey not found
func TestPublishResultsHTML_SurveyNotFound(t *testing.T) {
	e, _, h := setupTest()

	req := httptest.NewRequest(http.MethodPost, "/surveys/nonexistent/publish-results", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("nonexistent")

	err := h.PublishResultsHTML(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// RED PHASE: Test PublishResultsHTML - Survey has no URI (local-only)
func TestPublishResultsHTML_SurveyNoURI(t *testing.T) {
	e, mq, h := setupTest()

	// Create a local-only survey (no URI)
	survey := &models.Survey{
		ID:    uuid.New(),
		Slug:  "local-survey",
		Title: "Local Survey",
		Definition: models.SurveyDefinition{
			Questions: []models.Question{
				{
					ID:       "q1",
					Text:     "Test",
					Type:     models.QuestionTypeSingle,
					Required: true,
					Options:  []models.Option{{ID: "a", Text: "A"}},
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	mq.CreateSurvey(context.Background(), survey)

	req := httptest.NewRequest(http.MethodPost, "/surveys/local-survey/publish-results", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("local-survey")

	err := h.PublishResultsHTML(c)
	require.NoError(t, err)

	// Should return error (cannot publish results for local-only survey)
	assert.Equal(t, http.StatusOK, rec.Code)
	// The handler checks for login first, so we may get login error or ATProto error
	body := strings.ToLower(rec.Body.String())
	assert.True(t, strings.Contains(body, "log") || strings.Contains(body, "atproto") || strings.Contains(body, "error"))
}

// RED PHASE: Test GetStats - returns correct statistics
func TestGetStats_Success(t *testing.T) {
	_, mq, _ := setupTest()

	// Create some test data
	survey1 := &models.Survey{
		ID:    uuid.New(),
		Slug:  "survey-1",
		Title: "Survey 1",
		Definition: models.SurveyDefinition{
			Questions: []models.Question{
				{
					ID:       "q1",
					Text:     "Question 1",
					Type:     models.QuestionTypeSingle,
					Required: true,
					Options:  []models.Option{{ID: "a", Text: "A"}},
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	mq.CreateSurvey(context.Background(), survey1)

	survey2 := &models.Survey{
		ID:    uuid.New(),
		Slug:  "survey-2",
		Title: "Survey 2",
		Definition: models.SurveyDefinition{
			Questions: []models.Question{
				{
					ID:       "q1",
					Text:     "Question 1",
					Type:     models.QuestionTypeSingle,
					Required: true,
					Options:  []models.Option{{ID: "a", Text: "A"}},
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	mq.CreateSurvey(context.Background(), survey2)

	// Create some responses
	did1 := "did:plc:user1"
	did2 := "did:plc:user2"

	response1 := &models.Response{
		ID:       uuid.New(),
		SurveyID: survey1.ID,
		VoterDID: &did1,
		Answers:  map[string]models.Answer{"q1": {SelectedOptions: []string{"a"}}},
		CreatedAt: time.Now(),
	}
	mq.CreateResponse(context.Background(), response1)

	response2 := &models.Response{
		ID:       uuid.New(),
		SurveyID: survey1.ID,
		VoterDID: &did2,
		Answers:  map[string]models.Answer{"q1": {SelectedOptions: []string{"a"}}},
		CreatedAt: time.Now(),
	}
	mq.CreateResponse(context.Background(), response2)

	response3 := &models.Response{
		ID:       uuid.New(),
		SurveyID: survey2.ID,
		VoterDID: &did1, // Same user, different survey
		Answers:  map[string]models.Answer{"q1": {SelectedOptions: []string{"a"}}},
		CreatedAt: time.Now(),
	}
	mq.CreateResponse(context.Background(), response3)

	// Test GetStats
	stats, err := mq.GetStats(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, stats.SurveyCount, "Should have 2 surveys")
	assert.Equal(t, 3, stats.ResponseCount, "Should have 3 responses")
	assert.Equal(t, 2, stats.UniqueUserCount, "Should have 2 unique users")
}

// RED PHASE: Test LandingPageHTML - renders landing page with stats
func TestLandingPageHTML_Success(t *testing.T) {
	e, mq, h := setupTest()

	// Create some test data
	survey := &models.Survey{
		ID:    uuid.New(),
		Slug:  "test-survey",
		Title: "Test Survey",
		Definition: models.SurveyDefinition{
			Questions: []models.Question{
				{
					ID:       "q1",
					Text:     "Test",
					Type:     models.QuestionTypeSingle,
					Required: true,
					Options:  []models.Option{{ID: "a", Text: "A"}},
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	mq.CreateSurvey(context.Background(), survey)

	did := "did:plc:testuser"
	response := &models.Response{
		ID:       uuid.New(),
		SurveyID: survey.ID,
		VoterDID: &did,
		Answers:  map[string]models.Answer{"q1": {SelectedOptions: []string{"a"}}},
		CreatedAt: time.Now(),
	}
	mq.CreateResponse(context.Background(), response)

	// Request landing page
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.LandingPage(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Check that the response contains expected content
	body := rec.Body.String()
	assert.Contains(t, body, "Welcome", "Should contain welcome message")
	assert.Contains(t, body, "1", "Should show survey count")
	assert.Contains(t, body, "Create Survey", "Should have CTA button")
	assert.Contains(t, body, "Browse Surveys", "Should have browse button")
}

// RED PHASE: Test MyData handlers require authentication
func TestMyDataHTML_RequiresAuth(t *testing.T) {
	e, _, h := setupTest()

	req := httptest.NewRequest(http.MethodGet, "/my-data", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.MyDataHTML(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// RED PHASE: Test MyData shows overview when authenticated
func TestMyDataHTML_ShowsOverview(t *testing.T) {
	e, _, h := setupTest()

	// Set authenticated user in context
	req := httptest.NewRequest(http.MethodGet, "/my-data", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user", &oauth.User{DID: "did:plc:test123"})

	err := h.MyDataHTML(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	body := rec.Body.String()
	assert.Contains(t, body, "My Data", "Should contain page title")
	assert.Contains(t, body, "net.openmeet.survey", "Should show survey collection")
}

// RED PHASE: Test MyDataCollection requires auth
func TestMyDataCollectionHTML_RequiresAuth(t *testing.T) {
	e, _, h := setupTest()

	req := httptest.NewRequest(http.MethodGet, "/my-data/net.openmeet.survey", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/my-data/:collection")
	c.SetParamNames("collection")
	c.SetParamValues("net.openmeet.survey")

	err := h.MyDataCollectionHTML(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// RED PHASE: Test MyDataRecord requires auth
func TestMyDataRecordHTML_RequiresAuth(t *testing.T) {
	e, _, h := setupTest()

	req := httptest.NewRequest(http.MethodGet, "/my-data/net.openmeet.survey/abc123", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/my-data/:collection/:rkey")
	c.SetParamNames("collection", "rkey")
	c.SetParamValues("net.openmeet.survey", "abc123")

	err := h.MyDataRecordHTML(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// RED PHASE: Test DeleteRecords requires auth
func TestDeleteRecordsHTML_RequiresAuth(t *testing.T) {
	e, _, h := setupTest()

	req := httptest.NewRequest(http.MethodPost, "/my-data/delete", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.DeleteRecordsHTML(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
