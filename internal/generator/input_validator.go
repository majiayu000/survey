package generator

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const (
	// MaxInputLength is the maximum allowed length for AI generation input
	MaxInputLength = 2000
)

var (
	// ErrEmptyInput is returned when input is empty or whitespace only
	ErrEmptyInput = errors.New("input cannot be empty")

	// ErrInputTooLong is returned when input exceeds max length
	ErrInputTooLong = fmt.Errorf("input too long: maximum %d characters allowed", MaxInputLength)

	// ErrBlockedPattern is returned when input contains dangerous patterns
	ErrBlockedPattern = errors.New("input contains blocked pattern")
)

// InputValidator validates user input for AI survey generation
type InputValidator struct {
	blockedPatterns []*regexp.Regexp
}

// NewInputValidator creates a new input validator with predefined blocked patterns
func NewInputValidator() *InputValidator {
	return &InputValidator{
		blockedPatterns: compileBlockedPatterns(),
	}
}

// Validate checks if the input is safe for AI processing
func (v *InputValidator) Validate(input string) error {
	// Trim whitespace
	trimmed := strings.TrimSpace(input)

	// Check if empty
	if trimmed == "" {
		return ErrEmptyInput
	}

	// Check length
	if len(trimmed) > MaxInputLength {
		return ErrInputTooLong
	}

	// Check for blocked patterns
	lowerInput := strings.ToLower(trimmed)
	for _, pattern := range v.blockedPatterns {
		if pattern.MatchString(lowerInput) {
			return ErrBlockedPattern
		}
	}

	return nil
}

// compileBlockedPatterns returns a list of regex patterns to block
// These protect against SQL injection, XSS, and prompt injection
func compileBlockedPatterns() []*regexp.Regexp {
	patterns := []string{
		// SQL injection patterns
		`\b(drop|delete|truncate|alter|create)\s+(table|database|schema)`,
		`\bunion\s+select\b`,
		`\bselect\s+.*\s+from\s+`,
		`\bwhere\s+1\s*=\s*1`,
		`--`,        // SQL comment
		`/\*.*\*/`,  // Multi-line comment

		// XSS patterns
		`<script[^>]*>`,
		`</script>`,
		`javascript:`,
		`<iframe`,
		`<img[^>]+onerror`,
		`<img[^>]+src\s*=`,

		// Prompt injection patterns
		`\bignore\s+(all\s+)?(previous|above|prior)\s+instructions`,
		`\bsystem\s*:\s*you\s+are\s+(now\s+)?`,
		`\bassistant\s*:\s*i\s+will`,
		`\breplace\s+your\s+instructions`,
		`\bforget\s+(everything|all|your\s+rules)`,
		`\bact\s+as\s+(if\s+)?you\s+(are|were)\s+`,
	}

	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		compiled = append(compiled, regexp.MustCompile(p))
	}

	return compiled
}
