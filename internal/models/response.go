package models

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Response represents a user's response to a survey
type Response struct {
	ID           uuid.UUID         `db:"id" json:"id"`
	SurveyID     uuid.UUID         `db:"survey_id" json:"surveyId"`
	VoterDID     *string           `db:"voter_did" json:"voterDid,omitempty"`
	VoterSession *string           `db:"voter_session" json:"voterSession,omitempty"`
	RecordURI    *string           `db:"record_uri" json:"recordUri,omitempty"`
	RecordCID    *string           `db:"record_cid" json:"recordCid,omitempty"`
	Answers      map[string]Answer `db:"answers" json:"answers"`
	CreatedAt    time.Time         `db:"created_at" json:"createdAt"`
}

// Answer represents a response to a single question
type Answer struct {
	SelectedOptions []string `json:"selectedOptions,omitempty"`
	Text            string   `json:"text,omitempty"`
}

// GenerateVoterSession creates a SHA256 hash for anonymous voter identification
// The hash is per-survey salted using surveyID + ip + userAgent
func GenerateVoterSession(surveyID uuid.UUID, ip string, userAgent string) string {
	data := fmt.Sprintf("%s:%s:%s", surveyID.String(), ip, userAgent)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// ValidateAnswers validates that the answers are valid for the survey definition
func ValidateAnswers(def *SurveyDefinition, answers map[string]Answer) error {
	// Create a map of question IDs for quick lookup
	questionMap := make(map[string]*Question)
	for i := range def.Questions {
		questionMap[def.Questions[i].ID] = &def.Questions[i]
	}

	// Check for unknown questions in answers
	for answerID := range answers {
		if _, exists := questionMap[answerID]; !exists {
			return fmt.Errorf("unknown question ID '%s'", answerID)
		}
	}

	// Validate each question
	for _, question := range def.Questions {
		answer, hasAnswer := answers[question.ID]

		// Check if required question is answered
		if question.Required && !hasAnswer {
			return fmt.Errorf("required question '%s' is not answered", question.ID)
		}

		// If not answered and not required, skip validation
		if !hasAnswer {
			continue
		}

		// Validate based on question type
		switch question.Type {
		case QuestionTypeSingle:
			if err := validateSingleChoice(&question, &answer); err != nil {
				return fmt.Errorf("question '%s': %w", question.ID, err)
			}
		case QuestionTypeMulti:
			if err := validateMultiChoice(&question, &answer); err != nil {
				return fmt.Errorf("question '%s': %w", question.ID, err)
			}
		case QuestionTypeText:
			if err := validateTextAnswer(&question, &answer); err != nil {
				return fmt.Errorf("question '%s': %w", question.ID, err)
			}
			// Write back the sanitized answer
			answers[question.ID] = answer
		}
	}

	return nil
}

func validateSingleChoice(question *Question, answer *Answer) error {
	if len(answer.SelectedOptions) != 1 {
		return errors.New("single-choice question must have exactly one option selected")
	}

	selectedOption := answer.SelectedOptions[0]
	validOptions := make(map[string]bool)
	for _, opt := range question.Options {
		validOptions[opt.ID] = true
	}

	if !validOptions[selectedOption] {
		return fmt.Errorf("invalid option '%s'", selectedOption)
	}

	return nil
}

func validateMultiChoice(question *Question, answer *Answer) error {
	if len(answer.SelectedOptions) == 0 {
		return errors.New("multi-choice question must have at least one option selected")
	}

	validOptions := make(map[string]bool)
	for _, opt := range question.Options {
		validOptions[opt.ID] = true
	}

	for _, selectedOption := range answer.SelectedOptions {
		if !validOptions[selectedOption] {
			return fmt.Errorf("invalid option '%s'", selectedOption)
		}
	}

	return nil
}

func validateTextAnswer(question *Question, answer *Answer) error {
	// Sanitize text answer
	answer.Text = SanitizeText(answer.Text)

	// Validate after sanitization
	if question.Required && answer.Text == "" {
		return errors.New("text answer is required")
	}

	// Check length limit
	if len(answer.Text) > MaxTextAnswerLength {
		return fmt.Errorf("text answer exceeds maximum length of %d characters", MaxTextAnswerLength)
	}

	return nil
}

// Stats represents statistics about the survey service
type Stats struct {
	SurveyCount     int `json:"surveyCount"`
	ResponseCount   int `json:"responseCount"`
	UniqueUserCount int `json:"uniqueUserCount"`
}
