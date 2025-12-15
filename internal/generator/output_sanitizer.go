package generator

import (
	"github.com/openmeet-team/survey/internal/models"
)

// OutputSanitizer sanitizes and validates LLM output
type OutputSanitizer struct{}

// NewOutputSanitizer creates a new output sanitizer
func NewOutputSanitizer() *OutputSanitizer {
	return &OutputSanitizer{}
}

// Sanitize parses JSON output from LLM and sanitizes it using existing validation
func (s *OutputSanitizer) Sanitize(llmOutput string) (*models.SurveyDefinition, error) {
	// Parse the JSON using existing parser
	def, err := models.ParseSurveyDefinition([]byte(llmOutput))
	if err != nil {
		return nil, err
	}

	// Validate and sanitize using existing validation (which calls SanitizeText)
	if err := def.ValidateDefinition(); err != nil {
		return nil, err
	}

	return def, nil
}
