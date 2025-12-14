package models

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// QuestionType represents the type of question
type QuestionType string

const (
	QuestionTypeSingle QuestionType = "single"
	QuestionTypeMulti  QuestionType = "multi"
	QuestionTypeText   QuestionType = "text"
)

// Survey represents a survey definition stored in the database
type Survey struct {
	ID          uuid.UUID         `db:"id" json:"id"`
	URI         *string           `db:"uri" json:"uri,omitempty"`
	CID         *string           `db:"cid" json:"cid,omitempty"`
	AuthorDID   *string           `db:"author_did" json:"authorDid,omitempty"`
	Slug        string            `db:"slug" json:"slug"`
	Title       string            `db:"title" json:"title"`
	Description *string           `db:"description" json:"description,omitempty"`
	Definition  SurveyDefinition  `db:"definition" json:"definition"`
	StartsAt    *time.Time        `db:"starts_at" json:"startsAt,omitempty"`
	EndsAt      *time.Time        `db:"ends_at" json:"endsAt,omitempty"`
	ResultsURI  *string           `db:"results_uri" json:"resultsUri,omitempty"`
	ResultsCID  *string           `db:"results_cid" json:"resultsCid,omitempty"`
	CreatedAt   time.Time         `db:"created_at" json:"createdAt"`
	UpdatedAt   time.Time         `db:"updated_at" json:"updatedAt"`
}

// SurveyDefinition represents the survey structure stored as JSONB
type SurveyDefinition struct {
	Questions []Question `json:"questions"`
	Anonymous bool       `json:"anonymous"`
}

// Question represents a survey question
type Question struct {
	ID       string       `json:"id"`
	Text     string       `json:"text"`
	Type     QuestionType `json:"type"`
	Required bool         `json:"required"`
	Options  []Option     `json:"options,omitempty"`
}

// Option represents a choice option for a question
type Option struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// Security limits for YAML bomb protection
const (
	MaxSurveyDefinitionSize = 100 * 1024 // 100KB
	MaxQuestions            = 50
	MaxOptionsPerQuestion   = 20
	MaxQuestionTextLength   = 1000
	MaxOptionTextLength     = 500
	MaxTextAnswerLength     = 5000 // Maximum length for free-form text answers
)

// Regex patterns for sanitization (compiled once for performance)
var (
	// Matches dangerous HTML tags (script, iframe, object, embed, link, style, img)
	// Case-insensitive, matches both self-closing and paired tags with any content
	dangerousTagsRegex = regexp.MustCompile(`(?i)<\s*(script|iframe|object|embed|link|style|img)(\s+[^>]*)?>(.*?)</\s*(script|iframe|object|embed|link|style|img)\s*>|<\s*(script|iframe|object|embed|link|style|img)(\s+[^>]*)?>`)
)

// SanitizeText removes dangerous HTML tags and control characters from user input
// This provides defense in depth even though templ auto-escapes output.
// It strips:
// - Dangerous HTML tags (script, iframe, img, object, embed, link, style)
// - Null bytes and harmful control characters (except \n, \t, \r)
// - Leading/trailing whitespace
// It preserves:
// - Normal text with special chars (ampersands, quotes, <, >, etc.)
// - Legitimate whitespace (newlines, tabs, spaces within text)
func SanitizeText(input string) string {
	// Remove dangerous HTML tags
	sanitized := dangerousTagsRegex.ReplaceAllString(input, "")

	// Remove null bytes and dangerous control characters
	// Keep only printable characters plus newline, tab, carriage return
	sanitized = strings.Map(func(r rune) rune {
		// Allow printable characters
		if unicode.IsPrint(r) {
			return r
		}
		// Allow newline, tab, carriage return
		if r == '\n' || r == '\t' || r == '\r' {
			return r
		}
		// Remove all other control characters
		return -1
	}, sanitized)

	// Trim leading/trailing whitespace
	sanitized = strings.TrimSpace(sanitized)

	return sanitized
}

// ParseSurveyDefinition parses a survey definition from JSON or YAML
func ParseSurveyDefinition(data []byte) (*SurveyDefinition, error) {
	// Check input size limit
	if len(data) > MaxSurveyDefinitionSize {
		return nil, fmt.Errorf("survey definition too large: %d bytes exceeds maximum of 100KB", len(data))
	}

	var def SurveyDefinition

	// Try JSON first
	if err := json.Unmarshal(data, &def); err == nil {
		return &def, nil
	}

	// Try YAML with strict unmarshaling
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true) // Reject unknown fields

	if err := decoder.Decode(&def); err != nil {
		return nil, fmt.Errorf("failed to parse as JSON or YAML: %w", err)
	}

	return &def, nil
}

// ValidateDefinition validates the survey definition
func (d *SurveyDefinition) ValidateDefinition() error {
	if len(d.Questions) == 0 {
		return errors.New("survey must have at least one question")
	}

	// Check total question count
	if len(d.Questions) > MaxQuestions {
		return fmt.Errorf("too many questions: %d exceeds maximum of 50", len(d.Questions))
	}

	questionIDs := make(map[string]bool)

	for i, q := range d.Questions {
		// Validate question ID
		if q.ID == "" {
			return fmt.Errorf("question %d: question ID is required", i)
		}

		// Check for duplicate question IDs
		if questionIDs[q.ID] {
			return fmt.Errorf("question %d: duplicate question ID '%s'", i, q.ID)
		}
		questionIDs[q.ID] = true

		// Sanitize question text
		d.Questions[i].Text = SanitizeText(q.Text)

		// Validate question text (after sanitization)
		if d.Questions[i].Text == "" {
			return fmt.Errorf("question %d: question text is required", i)
		}

		// Check question text length
		if len(d.Questions[i].Text) > MaxQuestionTextLength {
			return fmt.Errorf("question %d: question text too long: %d characters exceeds maximum of 1000", i, len(d.Questions[i].Text))
		}

		// Validate question type
		if q.Type != QuestionTypeSingle && q.Type != QuestionTypeMulti && q.Type != QuestionTypeText {
			return fmt.Errorf("question %d: invalid question type '%s'", i, q.Type)
		}

		// Validate options for choice questions
		if q.Type == QuestionTypeSingle || q.Type == QuestionTypeMulti {
			if len(q.Options) < 2 {
				return fmt.Errorf("question %d: choice questions must have at least 2 options", i)
			}

			// Check option count
			if len(q.Options) > MaxOptionsPerQuestion {
				return fmt.Errorf("question %d: too many options: %d exceeds maximum of 20", i, len(q.Options))
			}

			optionIDs := make(map[string]bool)
			for j, opt := range q.Options {
				if opt.ID == "" {
					return fmt.Errorf("question %d, option %d: option ID is required", i, j)
				}

				// Sanitize option text
				d.Questions[i].Options[j].Text = SanitizeText(opt.Text)

				// Validate option text (after sanitization)
				if d.Questions[i].Options[j].Text == "" {
					return fmt.Errorf("question %d, option %d: option text is required", i, j)
				}

				// Check option text length
				if len(d.Questions[i].Options[j].Text) > MaxOptionTextLength {
					return fmt.Errorf("question %d, option %d: option text too long: %d characters exceeds maximum of 500", i, j, len(d.Questions[i].Options[j].Text))
				}

				if optionIDs[opt.ID] {
					return fmt.Errorf("question %d: duplicate option ID '%s'", i, opt.ID)
				}
				optionIDs[opt.ID] = true
			}
		}
	}

	return nil
}

var slugRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$|^[a-z0-9]{3}$`)

// ValidateSlug validates a survey slug
func ValidateSlug(slug string) error {
	if len(slug) < 3 || len(slug) > 50 {
		return errors.New("slug must be between 3 and 50 characters")
	}

	if !slugRegex.MatchString(slug) {
		return errors.New("slug must contain only lowercase letters, numbers, and hyphens (cannot start or end with hyphen)")
	}

	return nil
}

// SurveyResults represents aggregated results for a survey
type SurveyResults struct {
	SurveyID        uuid.UUID                  `json:"surveyId"`
	TotalVotes      int                        `json:"totalVotes"`
	QuestionResults map[string]*QuestionResult `json:"questionResults"` // keyed by question ID
}

// QuestionResult represents aggregated results for a single question
type QuestionResult struct {
	QuestionID   string         `json:"questionId"`
	OptionCounts map[string]int `json:"optionCounts"` // keyed by option ID, value is count
	TextAnswers  []string       `json:"textAnswers"`  // for text questions
}
