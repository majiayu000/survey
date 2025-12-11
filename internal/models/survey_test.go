package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSurveyDefinition_JSON(t *testing.T) {
	jsonData := []byte(`{
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
	}`)

	def, err := ParseSurveyDefinition(jsonData)
	require.NoError(t, err)
	assert.NotNil(t, def)
	assert.Equal(t, 1, len(def.Questions))
	assert.Equal(t, "q1", def.Questions[0].ID)
	assert.Equal(t, "What is your favorite color?", def.Questions[0].Text)
	assert.Equal(t, QuestionTypeSingle, def.Questions[0].Type)
	assert.True(t, def.Questions[0].Required)
	assert.Equal(t, 2, len(def.Questions[0].Options))
	assert.False(t, def.Anonymous)
}

func TestParseSurveyDefinition_YAML(t *testing.T) {
	yamlData := []byte(`
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
      - id: wed
        text: Wednesday
anonymous: true
`)

	def, err := ParseSurveyDefinition(yamlData)
	require.NoError(t, err)
	assert.NotNil(t, def)
	assert.Equal(t, 1, len(def.Questions))
	assert.Equal(t, "q1", def.Questions[0].ID)
	assert.Equal(t, "What days work for you?", def.Questions[0].Text)
	assert.Equal(t, QuestionTypeMulti, def.Questions[0].Type)
	assert.True(t, def.Questions[0].Required)
	assert.Equal(t, 3, len(def.Questions[0].Options))
	assert.True(t, def.Anonymous)
}

func TestParseSurveyDefinition_TextQuestion(t *testing.T) {
	jsonData := []byte(`{
		"questions": [
			{
				"id": "q1",
				"text": "What are your thoughts?",
				"type": "text",
				"required": false
			}
		],
		"anonymous": false
	}`)

	def, err := ParseSurveyDefinition(jsonData)
	require.NoError(t, err)
	assert.NotNil(t, def)
	assert.Equal(t, 1, len(def.Questions))
	assert.Equal(t, QuestionTypeText, def.Questions[0].Type)
	assert.False(t, def.Questions[0].Required)
	assert.Nil(t, def.Questions[0].Options)
}

func TestParseSurveyDefinition_InvalidJSON(t *testing.T) {
	invalidData := []byte(`{invalid json`)

	_, err := ParseSurveyDefinition(invalidData)
	assert.Error(t, err)
}

func TestValidateDefinition_Valid(t *testing.T) {
	def := &SurveyDefinition{
		Questions: []Question{
			{
				ID:       "q1",
				Text:     "Choose one",
				Type:     QuestionTypeSingle,
				Required: true,
				Options: []Option{
					{ID: "a", Text: "Option A"},
					{ID: "b", Text: "Option B"},
				},
			},
		},
		Anonymous: false,
	}

	err := def.ValidateDefinition()
	assert.NoError(t, err)
}

func TestValidateDefinition_MissingQuestionID(t *testing.T) {
	def := &SurveyDefinition{
		Questions: []Question{
			{
				ID:   "", // Missing ID
				Text: "Choose one",
				Type: QuestionTypeSingle,
			},
		},
	}

	err := def.ValidateDefinition()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "question ID is required")
}

func TestValidateDefinition_MissingQuestionText(t *testing.T) {
	def := &SurveyDefinition{
		Questions: []Question{
			{
				ID:   "q1",
				Text: "", // Missing text
				Type: QuestionTypeSingle,
			},
		},
	}

	err := def.ValidateDefinition()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "question text is required")
}

func TestValidateDefinition_InvalidQuestionType(t *testing.T) {
	def := &SurveyDefinition{
		Questions: []Question{
			{
				ID:   "q1",
				Text: "Choose one",
				Type: "invalid",
			},
		},
	}

	err := def.ValidateDefinition()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid question type")
}

func TestValidateDefinition_ChoiceQuestionMissingOptions(t *testing.T) {
	def := &SurveyDefinition{
		Questions: []Question{
			{
				ID:      "q1",
				Text:    "Choose one",
				Type:    QuestionTypeSingle,
				Options: nil, // Missing options for choice question
			},
		},
	}

	err := def.ValidateDefinition()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must have at least 2 options")
}

func TestValidateDefinition_DuplicateQuestionIDs(t *testing.T) {
	def := &SurveyDefinition{
		Questions: []Question{
			{
				ID:   "q1",
				Text: "Question 1",
				Type: QuestionTypeText,
			},
			{
				ID:   "q1", // Duplicate ID
				Text: "Question 2",
				Type: QuestionTypeText,
			},
		},
	}

	err := def.ValidateDefinition()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate question ID")
}

func TestValidateDefinition_DuplicateOptionIDs(t *testing.T) {
	def := &SurveyDefinition{
		Questions: []Question{
			{
				ID:   "q1",
				Text: "Choose one",
				Type: QuestionTypeSingle,
				Options: []Option{
					{ID: "a", Text: "Option A"},
					{ID: "a", Text: "Option B"}, // Duplicate ID
				},
			},
		},
	}

	err := def.ValidateDefinition()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate option ID")
}

func TestValidateDefinition_NoQuestions(t *testing.T) {
	def := &SurveyDefinition{
		Questions: []Question{},
		Anonymous: false,
	}

	err := def.ValidateDefinition()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one question")
}

func TestValidateSlug_Valid(t *testing.T) {
	validSlugs := []string{
		"my-survey",
		"survey123",
		"test-survey-2024",
		"abc",
	}

	for _, slug := range validSlugs {
		err := ValidateSlug(slug)
		assert.NoError(t, err, "slug %s should be valid", slug)
	}
}

func TestValidateSlug_TooShort(t *testing.T) {
	err := ValidateSlug("ab")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "3 and 50 characters")
}

func TestValidateSlug_TooLong(t *testing.T) {
	err := ValidateSlug("this-is-a-very-long-slug-that-exceeds-the-maximum-length-allowed")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "3 and 50 characters")
}

func TestValidateSlug_InvalidCharacters(t *testing.T) {
	invalidSlugs := []string{
		"my survey",      // space
		"survey_test",    // underscore
		"survey@test",    // special char
		"Survey-Test",    // uppercase
		"survey.test",    // period
		"-survey",        // starts with hyphen
		"survey-",        // ends with hyphen
	}

	for _, slug := range invalidSlugs {
		err := ValidateSlug(slug)
		assert.Error(t, err, "slug %s should be invalid", slug)
	}
}

func TestSurveyStruct(t *testing.T) {
	// Test that Survey struct can be created with all fields
	id := uuid.New()
	now := time.Now()

	survey := Survey{
		ID:          id,
		URI:         stringPtr("at://did:plc:abc/net.openmeet.survey/xyz"),
		CID:         stringPtr("bafyreiabc"),
		AuthorDID:   stringPtr("did:plc:abc"),
		Slug:        "test-survey",
		Title:       "Test Survey",
		Description: stringPtr("A test survey"),
		Definition: SurveyDefinition{
			Questions: []Question{
				{
					ID:   "q1",
					Text: "Test question",
					Type: QuestionTypeText,
				},
			},
			Anonymous: false,
		},
		StartsAt:  &now,
		EndsAt:    &now,
		CreatedAt: now,
		UpdatedAt: now,
	}

	assert.Equal(t, id, survey.ID)
	assert.Equal(t, "test-survey", survey.Slug)
	assert.Equal(t, "Test Survey", survey.Title)
	assert.NotNil(t, survey.URI)
	assert.NotNil(t, survey.CID)
	assert.NotNil(t, survey.AuthorDID)
	assert.NotNil(t, survey.Description)
	assert.NotNil(t, survey.StartsAt)
	assert.NotNil(t, survey.EndsAt)
}

func stringPtr(s string) *string {
	return &s
}
