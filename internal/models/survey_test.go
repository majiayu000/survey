package models

import (
	"fmt"
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

// YAML Bomb Protection Tests

func TestParseSurveyDefinition_RejectsOversizedInput(t *testing.T) {
	// Create input larger than MaxSurveyDefinitionSize (100KB)
	largeData := make([]byte, 101*1024)
	for i := range largeData {
		largeData[i] = 'a'
	}

	_, err := ParseSurveyDefinition(largeData)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "survey definition too large")
	assert.Contains(t, err.Error(), "100KB")
}

func TestParseSurveyDefinition_RejectsUnknownFields(t *testing.T) {
	yamlData := []byte(`
questions:
  - id: q1
    text: What is your favorite color?
    type: single
    options:
      - id: red
        text: Red
      - id: blue
        text: Blue
anonymous: false
maliciousField: "this should be rejected"
`)

	_, err := ParseSurveyDefinition(yamlData)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "field maliciousField not found")
}

func TestParseSurveyDefinition_RejectsUnknownQuestionFields(t *testing.T) {
	yamlData := []byte(`
questions:
  - id: q1
    text: What is your favorite color?
    type: single
    maliciousQuestionField: "bad"
    options:
      - id: red
        text: Red
      - id: blue
        text: Blue
anonymous: false
`)

	_, err := ParseSurveyDefinition(yamlData)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "field maliciousQuestionField not found")
}

func TestParseSurveyDefinition_AcceptsValidYAML(t *testing.T) {
	// Ensure strict YAML parsing doesn't break valid inputs
	yamlData := []byte(`
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
anonymous: false
`)

	def, err := ParseSurveyDefinition(yamlData)
	require.NoError(t, err)
	assert.NotNil(t, def)
	assert.Equal(t, 1, len(def.Questions))
}

func TestValidateDefinition_RejectsTooManyQuestions(t *testing.T) {
	// Create survey with 51 questions (max is 50)
	questions := make([]Question, 51)
	for i := 0; i < 51; i++ {
		questions[i] = Question{
			ID:   fmt.Sprintf("q%d", i),
			Text: "Question text",
			Type: QuestionTypeText,
		}
	}

	def := &SurveyDefinition{
		Questions: questions,
		Anonymous: false,
	}

	err := def.ValidateDefinition()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too many questions")
	assert.Contains(t, err.Error(), "50")
}

func TestValidateDefinition_RejectsTooManyOptions(t *testing.T) {
	// Create question with 21 options (max is 20)
	options := make([]Option, 21)
	for i := 0; i < 21; i++ {
		options[i] = Option{
			ID:   fmt.Sprintf("opt%d", i),
			Text: "Option text",
		}
	}

	def := &SurveyDefinition{
		Questions: []Question{
			{
				ID:      "q1",
				Text:    "Choose one",
				Type:    QuestionTypeSingle,
				Options: options,
			},
		},
		Anonymous: false,
	}

	err := def.ValidateDefinition()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too many options")
	assert.Contains(t, err.Error(), "20")
}

func TestValidateDefinition_RejectsQuestionTextTooLong(t *testing.T) {
	// Create question with text longer than 1000 chars
	longText := make([]byte, 1001)
	for i := range longText {
		longText[i] = 'a'
	}

	def := &SurveyDefinition{
		Questions: []Question{
			{
				ID:   "q1",
				Text: string(longText),
				Type: QuestionTypeText,
			},
		},
		Anonymous: false,
	}

	err := def.ValidateDefinition()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "question text too long")
	assert.Contains(t, err.Error(), "1000")
}

func TestValidateDefinition_RejectsOptionTextTooLong(t *testing.T) {
	// Create option with text longer than 500 chars
	longText := make([]byte, 501)
	for i := range longText {
		longText[i] = 'a'
	}

	def := &SurveyDefinition{
		Questions: []Question{
			{
				ID:   "q1",
				Text: "Choose one",
				Type: QuestionTypeSingle,
				Options: []Option{
					{ID: "a", Text: "Option A"},
					{ID: "b", Text: string(longText)},
				},
			},
		},
		Anonymous: false,
	}

	err := def.ValidateDefinition()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "option text too long")
	assert.Contains(t, err.Error(), "500")
}

func TestValidateDefinition_AcceptsMaxValidLimits(t *testing.T) {
	// Test that exactly at the limits is valid
	questions := make([]Question, 50)
	for i := 0; i < 50; i++ {
		questions[i] = Question{
			ID:   fmt.Sprintf("q%d", i),
			Text: "Question text",
			Type: QuestionTypeText,
		}
	}

	def := &SurveyDefinition{
		Questions: questions,
		Anonymous: false,
	}

	err := def.ValidateDefinition()
	assert.NoError(t, err)
}

func TestValidateDefinition_AcceptsMaxQuestionTextLength(t *testing.T) {
	// Exactly 1000 chars should be valid
	text := make([]byte, 1000)
	for i := range text {
		text[i] = 'a'
	}

	def := &SurveyDefinition{
		Questions: []Question{
			{
				ID:   "q1",
				Text: string(text),
				Type: QuestionTypeText,
			},
		},
		Anonymous: false,
	}

	err := def.ValidateDefinition()
	assert.NoError(t, err)
}

func TestValidateDefinition_AcceptsMaxOptionTextLength(t *testing.T) {
	// Exactly 500 chars should be valid
	text := make([]byte, 500)
	for i := range text {
		text[i] = 'a'
	}

	def := &SurveyDefinition{
		Questions: []Question{
			{
				ID:   "q1",
				Text: "Choose one",
				Type: QuestionTypeSingle,
				Options: []Option{
					{ID: "a", Text: "Option A"},
					{ID: "b", Text: string(text)},
				},
			},
		},
		Anonymous: false,
	}

	err := def.ValidateDefinition()
	assert.NoError(t, err)
}

// Input Sanitization Tests (XSS Protection)

func TestSanitizeText_RemovesScriptTags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple script tag",
			input:    "<script>alert('xss')</script>",
			expected: "",
		},
		{
			name:     "script tag with text",
			input:    "Hello <script>alert('xss')</script> world",
			expected: "Hello  world",
		},
		{
			name:     "uppercase script tag",
			input:    "<SCRIPT>alert('xss')</SCRIPT>",
			expected: "",
		},
		{
			name:     "mixed case script tag",
			input:    "<ScRiPt>alert('xss')</ScRiPt>",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeText(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeText_RemovesImgWithOnerror(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "img with onerror",
			input:    `<img src="x" onerror="alert('xss')">`,
			expected: "",
		},
		{
			name:     "img with onerror and text",
			input:    `Click here <img src="x" onerror="alert('xss')"> to vote`,
			expected: "Click here  to vote",
		},
		{
			name:     "uppercase IMG",
			input:    `<IMG SRC="x" ONERROR="alert('xss')">`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeText(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeText_RemovesOtherDangerousTags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "iframe tag",
			input:    `<iframe src="evil.com"></iframe>`,
			expected: "",
		},
		{
			name:     "object tag",
			input:    `<object data="evil.swf"></object>`,
			expected: "",
		},
		{
			name:     "embed tag",
			input:    `<embed src="evil.swf">`,
			expected: "",
		},
		{
			name:     "link tag",
			input:    `<link rel="stylesheet" href="evil.css">`,
			expected: "",
		},
		{
			name:     "style tag",
			input:    `<style>body{background:url('evil.com')}</style>`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeText(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeText_PreservesLegitimateText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "ampersand in text",
			input:    "Q&A session",
			expected: "Q&A session",
		},
		{
			name:     "math comparison",
			input:    "x > 5 and y < 10",
			expected: "x > 5 and y < 10",
		},
		{
			name:     "quotes in text",
			input:    `He said "hello" to me`,
			expected: `He said "hello" to me`,
		},
		{
			name:     "single quotes",
			input:    "It's a great day",
			expected: "It's a great day",
		},
		{
			name:     "parentheses",
			input:    "This (is important) to know",
			expected: "This (is important) to know",
		},
		{
			name:     "brackets",
			input:    "Array[0] contains value",
			expected: "Array[0] contains value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeText(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeText_StripsNullBytes(t *testing.T) {
	input := "Hello\x00World"
	expected := "HelloWorld"
	result := SanitizeText(input)
	assert.Equal(t, expected, result)
}

func TestSanitizeText_StripsControlCharacters(t *testing.T) {
	// Test various control characters
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "null byte",
			input:    "Hello\x00World",
			expected: "HelloWorld",
		},
		{
			name:     "vertical tab",
			input:    "Hello\x0BWorld",
			expected: "HelloWorld",
		},
		{
			name:     "form feed",
			input:    "Hello\x0CWorld",
			expected: "HelloWorld",
		},
		{
			name:     "preserves newline",
			input:    "Hello\nWorld",
			expected: "Hello\nWorld",
		},
		{
			name:     "preserves tab",
			input:    "Hello\tWorld",
			expected: "Hello\tWorld",
		},
		{
			name:     "preserves carriage return",
			input:    "Hello\rWorld",
			expected: "Hello\rWorld",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeText(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeText_TrimsWhitespace(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "leading whitespace",
			input:    "   Hello",
			expected: "Hello",
		},
		{
			name:     "trailing whitespace",
			input:    "Hello   ",
			expected: "Hello",
		},
		{
			name:     "both sides",
			input:    "   Hello   ",
			expected: "Hello",
		},
		{
			name:     "newlines around text",
			input:    "\n\nHello\n\n",
			expected: "Hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeText(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateDefinition_SanitizesQuestionText(t *testing.T) {
	def := &SurveyDefinition{
		Questions: []Question{
			{
				ID:   "q1",
				Text: "<script>alert('xss')</script>What is your name?",
				Type: QuestionTypeText,
			},
		},
		Anonymous: false,
	}

	err := def.ValidateDefinition()
	require.NoError(t, err)

	// Question text should be sanitized
	assert.Equal(t, "What is your name?", def.Questions[0].Text)
}

func TestValidateDefinition_SanitizesOptionText(t *testing.T) {
	def := &SurveyDefinition{
		Questions: []Question{
			{
				ID:   "q1",
				Text: "Choose one",
				Type: QuestionTypeSingle,
				Options: []Option{
					{ID: "a", Text: "Normal Option"},
					{ID: "b", Text: `<img src="x" onerror="alert('xss')">Malicious`},
				},
			},
		},
		Anonymous: false,
	}

	err := def.ValidateDefinition()
	require.NoError(t, err)

	// Option text should be sanitized
	assert.Equal(t, "Normal Option", def.Questions[0].Options[0].Text)
	assert.Equal(t, "Malicious", def.Questions[0].Options[1].Text)
}

func TestValidateDefinition_SanitizationPreservesValidation(t *testing.T) {
	// After sanitization, if text is empty, validation should still fail
	def := &SurveyDefinition{
		Questions: []Question{
			{
				ID:   "q1",
				Text: "<script>alert('xss')</script>", // Will be sanitized to empty
				Type: QuestionTypeText,
			},
		},
		Anonymous: false,
	}

	err := def.ValidateDefinition()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "question text is required")
}
