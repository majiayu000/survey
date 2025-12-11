package api

import (
	"time"

	"github.com/google/uuid"
	"github.com/openmeet-team/survey/internal/models"
)

// CreateSurveyRequest represents the request body for creating a survey
type CreateSurveyRequest struct {
	Slug       string `json:"slug"`       // optional, auto-generate if missing
	Definition string `json:"definition"` // YAML or JSON string
}

// SurveyResponse represents a survey in API responses
type SurveyResponse struct {
	ID          uuid.UUID                `json:"id"`
	URI         *string                  `json:"uri,omitempty"`
	CID         *string                  `json:"cid,omitempty"`
	AuthorDID   *string                  `json:"authorDid,omitempty"`
	Slug        string                   `json:"slug"`
	Title       string                   `json:"title"`
	Description *string                  `json:"description,omitempty"`
	Definition  *models.SurveyDefinition `json:"definition,omitempty"` // omitted in list view
	StartsAt    *time.Time               `json:"startsAt,omitempty"`
	EndsAt      *time.Time               `json:"endsAt,omitempty"`
	CreatedAt   time.Time                `json:"createdAt"`
	UpdatedAt   time.Time                `json:"updatedAt"`
}

// SurveyListResponse represents a survey in list responses (without full definition)
type SurveyListResponse struct {
	ID          uuid.UUID  `json:"id"`
	Slug        string     `json:"slug"`
	Title       string     `json:"title"`
	Description *string    `json:"description,omitempty"`
	StartsAt    *time.Time `json:"startsAt,omitempty"`
	EndsAt      *time.Time `json:"endsAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// SubmitResponseRequest represents the request body for submitting a survey response
type SubmitResponseRequest struct {
	Answers map[string]models.Answer `json:"answers"`
}

// ResponseSubmittedResponse represents the response after submitting a survey response
type ResponseSubmittedResponse struct {
	ID        uuid.UUID `json:"id"`
	SurveyID  uuid.UUID `json:"surveyId"`
	CreatedAt time.Time `json:"createdAt"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

// SurveyResultsResponse wraps the models.SurveyResults for API response
type SurveyResultsResponse struct {
	*models.SurveyResults
}

// ToSurveyResponse converts a models.Survey to a SurveyResponse
func ToSurveyResponse(s *models.Survey, includeDefinition bool) *SurveyResponse {
	resp := &SurveyResponse{
		ID:          s.ID,
		URI:         s.URI,
		CID:         s.CID,
		AuthorDID:   s.AuthorDID,
		Slug:        s.Slug,
		Title:       s.Title,
		Description: s.Description,
		StartsAt:    s.StartsAt,
		EndsAt:      s.EndsAt,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}

	if includeDefinition {
		resp.Definition = &s.Definition
	}

	return resp
}

// ToSurveyListResponse converts a models.Survey to a SurveyListResponse
func ToSurveyListResponse(s *models.Survey) *SurveyListResponse {
	return &SurveyListResponse{
		ID:          s.ID,
		Slug:        s.Slug,
		Title:       s.Title,
		Description: s.Description,
		StartsAt:    s.StartsAt,
		EndsAt:      s.EndsAt,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}
