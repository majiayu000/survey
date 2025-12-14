package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDefaultBodyLimitConfig verifies the body limit configuration
func TestDefaultBodyLimitConfig(t *testing.T) {
	config := DefaultBodyLimitConfig()

	assert.Equal(t, "100KB", config.SurveyCreation, "Survey creation limit should be 100KB")
	assert.Equal(t, "10KB", config.ResponseSubmission, "Response submission limit should be 10KB")
	assert.Equal(t, "1MB", config.GeneralAPI, "General API limit should be 1MB")
}

// TestBodyLimitConfigDocumentation documents the rationale for each limit
func TestBodyLimitConfigDocumentation(t *testing.T) {
	t.Log("Body size limits protect against large payload attacks")
	t.Log("")
	t.Log("Survey creation (100KB):")
	t.Log("  - Survey definitions are YAML text files")
	t.Log("  - Typical survey: 1-5KB")
	t.Log("  - 100KB allows for complex surveys with many questions")
	t.Log("  - Already limited by YAML parser but HTTP layer adds defense in depth")
	t.Log("")
	t.Log("Response submission (10KB):")
	t.Log("  - Responses are JSON with question IDs and selected options")
	t.Log("  - Typical response: <1KB")
	t.Log("  - 10KB allows for surveys with many questions and text responses")
	t.Log("")
	t.Log("General API (1MB):")
	t.Log("  - Default fallback for other endpoints")
	t.Log("  - Provides reasonable protection without being too restrictive")
}
