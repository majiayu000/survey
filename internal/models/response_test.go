package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestGenerateVoterSession(t *testing.T) {
	surveyID := uuid.New()
	ip := "192.168.1.1"
	userAgent := "Mozilla/5.0"

	session1 := GenerateVoterSession(surveyID, ip, userAgent)
	assert.NotEmpty(t, session1)
	assert.Len(t, session1, 64) // SHA256 hex string

	// Same inputs should produce same session
	session2 := GenerateVoterSession(surveyID, ip, userAgent)
	assert.Equal(t, session1, session2)

	// Different survey ID should produce different session
	differentSurveyID := uuid.New()
	session3 := GenerateVoterSession(differentSurveyID, ip, userAgent)
	assert.NotEqual(t, session1, session3)

	// Different IP should produce different session
	session4 := GenerateVoterSession(surveyID, "192.168.1.2", userAgent)
	assert.NotEqual(t, session1, session4)

	// Different user agent should produce different session
	session5 := GenerateVoterSession(surveyID, ip, "Different Agent")
	assert.NotEqual(t, session1, session5)
}

func TestValidateAnswers_Valid(t *testing.T) {
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
			{
				ID:       "q2",
				Text:     "Choose multiple",
				Type:     QuestionTypeMulti,
				Required: false,
				Options: []Option{
					{ID: "x", Text: "Option X"},
					{ID: "y", Text: "Option Y"},
					{ID: "z", Text: "Option Z"},
				},
			},
			{
				ID:       "q3",
				Text:     "Write something",
				Type:     QuestionTypeText,
				Required: false,
			},
		},
	}

	answers := map[string]Answer{
		"q1": {SelectedOptions: []string{"a"}},
		"q2": {SelectedOptions: []string{"x", "z"}},
		"q3": {Text: "My answer"},
	}

	err := ValidateAnswers(def, answers)
	assert.NoError(t, err)
}

func TestValidateAnswers_MissingRequiredQuestion(t *testing.T) {
	def := &SurveyDefinition{
		Questions: []Question{
			{
				ID:       "q1",
				Text:     "Required question",
				Type:     QuestionTypeSingle,
				Required: true,
				Options: []Option{
					{ID: "a", Text: "Option A"},
					{ID: "b", Text: "Option B"},
				},
			},
		},
	}

	answers := map[string]Answer{}

	err := ValidateAnswers(def, answers)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required question")
}

func TestValidateAnswers_InvalidOptionID(t *testing.T) {
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
	}

	answers := map[string]Answer{
		"q1": {SelectedOptions: []string{"invalid"}},
	}

	err := ValidateAnswers(def, answers)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid option")
}

func TestValidateAnswers_SingleChoiceMultipleSelections(t *testing.T) {
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
	}

	answers := map[string]Answer{
		"q1": {SelectedOptions: []string{"a", "b"}},
	}

	err := ValidateAnswers(def, answers)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exactly one option")
}

func TestValidateAnswers_SingleChoiceNoSelection(t *testing.T) {
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
	}

	answers := map[string]Answer{
		"q1": {SelectedOptions: []string{}},
	}

	err := ValidateAnswers(def, answers)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exactly one option")
}

func TestValidateAnswers_MultiChoiceValid(t *testing.T) {
	def := &SurveyDefinition{
		Questions: []Question{
			{
				ID:       "q1",
				Text:     "Choose multiple",
				Type:     QuestionTypeMulti,
				Required: true,
				Options: []Option{
					{ID: "a", Text: "Option A"},
					{ID: "b", Text: "Option B"},
					{ID: "c", Text: "Option C"},
				},
			},
		},
	}

	// Multiple selections is valid
	answers1 := map[string]Answer{
		"q1": {SelectedOptions: []string{"a", "b"}},
	}
	err := ValidateAnswers(def, answers1)
	assert.NoError(t, err)

	// Single selection is also valid for multi-choice
	answers2 := map[string]Answer{
		"q1": {SelectedOptions: []string{"a"}},
	}
	err = ValidateAnswers(def, answers2)
	assert.NoError(t, err)
}

func TestValidateAnswers_TextQuestionEmpty(t *testing.T) {
	def := &SurveyDefinition{
		Questions: []Question{
			{
				ID:       "q1",
				Text:     "Write something",
				Type:     QuestionTypeText,
				Required: true,
			},
		},
	}

	answers := map[string]Answer{
		"q1": {Text: ""},
	}

	err := ValidateAnswers(def, answers)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "text answer is required")
}

func TestValidateAnswers_UnknownQuestion(t *testing.T) {
	def := &SurveyDefinition{
		Questions: []Question{
			{
				ID:       "q1",
				Text:     "Question 1",
				Type:     QuestionTypeText,
				Required: false,
			},
		},
	}

	answers := map[string]Answer{
		"q1":      {Text: "Answer"},
		"unknown": {Text: "Invalid"},
	}

	err := ValidateAnswers(def, answers)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown question")
}

func TestValidateAnswers_OptionalQuestionsCanBeSkipped(t *testing.T) {
	def := &SurveyDefinition{
		Questions: []Question{
			{
				ID:       "q1",
				Text:     "Optional question",
				Type:     QuestionTypeText,
				Required: false,
			},
			{
				ID:       "q2",
				Text:     "Another optional",
				Type:     QuestionTypeSingle,
				Required: false,
				Options: []Option{
					{ID: "a", Text: "Option A"},
				},
			},
		},
	}

	// Empty answers should be valid for optional questions
	answers := map[string]Answer{}

	err := ValidateAnswers(def, answers)
	assert.NoError(t, err)
}

func TestResponseStruct(t *testing.T) {
	// Test that Response struct can be created with all fields
	id := uuid.New()
	surveyID := uuid.New()
	now := time.Now()

	response := Response{
		ID:            id,
		SurveyID:      surveyID,
		VoterDID:      stringPtr("did:plc:abc"),
		VoterSession:  stringPtr("session-hash"),
		RecordURI:     stringPtr("at://did:plc:abc/net.openmeet.survey.response/xyz"),
		RecordCID:     stringPtr("bafyreiabc"),
		Answers: map[string]Answer{
			"q1": {SelectedOptions: []string{"a"}},
			"q2": {Text: "My answer"},
		},
		CreatedAt: now,
	}

	assert.Equal(t, id, response.ID)
	assert.Equal(t, surveyID, response.SurveyID)
	assert.NotNil(t, response.VoterDID)
	assert.NotNil(t, response.VoterSession)
	assert.NotNil(t, response.RecordURI)
	assert.NotNil(t, response.RecordCID)
	assert.Len(t, response.Answers, 2)
}

func TestAnswerStruct(t *testing.T) {
	// Test choice answer
	choiceAnswer := Answer{
		SelectedOptions: []string{"a", "b"},
		Text:            "",
	}
	assert.Len(t, choiceAnswer.SelectedOptions, 2)

	// Test text answer
	textAnswer := Answer{
		SelectedOptions: nil,
		Text:            "My text answer",
	}
	assert.Equal(t, "My text answer", textAnswer.Text)
}
