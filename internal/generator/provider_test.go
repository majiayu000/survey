package generator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tmc/langchaingo/llms/fake"
)

func TestSurveyGenerator_Generate(t *testing.T) {
	t.Run("generates valid survey from prompt", func(t *testing.T) {
		// Use fake LLM that returns valid JSON matching the lexicon schema
		validJSON := `{
			"questions": [
				{
					"id": "q1",
					"text": "Do you like pizza?",
					"type": "single",
					"required": false,
					"options": [
						{"id": "opt1", "text": "Yes"},
						{"id": "opt2", "text": "No"}
					]
				}
			],
			"anonymous": false
		}`
		fakeLLM := fake.NewFakeLLM([]string{validJSON})

		generator := NewSurveyGenerator(fakeLLM, "gpt-4o-mini")
		ctx := context.Background()

		result, err := generator.Generate(ctx, "Create a simple yes/no poll about pizza")

		require.NoError(t, err)
		assert.NotNil(t, result.Definition)
		assert.Equal(t, 1, len(result.Definition.Questions))
		assert.Equal(t, "Do you like pizza?", result.Definition.Questions[0].Text)
		assert.Equal(t, "single", string(result.Definition.Questions[0].Type))
		assert.Greater(t, result.InputTokens, 0)
		assert.Greater(t, result.OutputTokens, 0)
		assert.Greater(t, result.EstimatedCost, 0.0)
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		fakeLLM := fake.NewFakeLLM([]string{"This is not valid JSON"})
		generator := NewSurveyGenerator(fakeLLM, "gpt-4o-mini")
		ctx := context.Background()

		_, err := generator.Generate(ctx, "Create a survey")

		assert.Error(t, err)
	})

	t.Run("returns error for empty LLM response", func(t *testing.T) {
		fakeLLM := fake.NewFakeLLM([]string{""})
		generator := NewSurveyGenerator(fakeLLM, "gpt-4o-mini")
		ctx := context.Background()

		_, err := generator.Generate(ctx, "Create a survey")

		assert.Error(t, err)
	})

	t.Run("sanitizes malicious output", func(t *testing.T) {
		// LLM tries to inject HTML
		maliciousJSON := `{
			"questions": [
				{
					"id": "q1",
					"text": "<script>alert('xss')</script>What is your name?",
					"type": "text",
					"required": false,
					"options": []
				}
			],
			"anonymous": false
		}`
		fakeLLM := fake.NewFakeLLM([]string{maliciousJSON})
		generator := NewSurveyGenerator(fakeLLM, "gpt-4o-mini")
		ctx := context.Background()

		result, err := generator.Generate(ctx, "Create a survey")

		// Should succeed but sanitize the output
		require.NoError(t, err)
		// The HTML should be removed by sanitization
		assert.NotContains(t, result.Definition.Questions[0].Text, "<script>")
		assert.Contains(t, result.Definition.Questions[0].Text, "What is your name?")
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		validJSON := `{"questions":[{"id":"q1","text":"Test?","type":"single","required":false,"options":[{"id":"opt1","text":"Yes"}]}],"anonymous":false}`
		fakeLLM := fake.NewFakeLLM([]string{validJSON})
		generator := NewSurveyGenerator(fakeLLM, "gpt-4o-mini")
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := generator.Generate(ctx, "Create a survey")

		assert.Error(t, err)
	})
}

func TestSurveyGenerator_TokenCounting(t *testing.T) {
	t.Run("estimates tokens correctly", func(t *testing.T) {
		validJSON := `{"questions":[{"id":"q1","text":"Test?","type":"single","required":false,"options":[{"id":"opt1","text":"Yes"}]}],"anonymous":false}`
		fakeLLM := fake.NewFakeLLM([]string{validJSON})
		generator := NewSurveyGenerator(fakeLLM, "gpt-4o-mini")

		// Rough estimate: ~1 token per 4 characters
		input := "This is a test prompt with about twenty words in it for testing purposes and validation."
		tokens := generator.estimateTokens(input)

		// Should be roughly 20-30 tokens (words + punctuation)
		assert.Greater(t, tokens, 15)
		assert.Less(t, tokens, 40)
	})
}

func TestSurveyGenerator_SystemPrompt(t *testing.T) {
	t.Run("system prompt includes JSON schema and security instructions", func(t *testing.T) {
		validJSON := `{"questions":[{"id":"q1","text":"Test?","type":"single","required":false,"options":[{"id":"opt1","text":"Yes"}]}],"anonymous":false}`
		fakeLLM := fake.NewFakeLLM([]string{validJSON})
		generator := NewSurveyGenerator(fakeLLM, "gpt-4o-mini")
		prompt := generator.buildSystemPrompt()

		// Should contain JSON format instructions
		assert.Contains(t, prompt, "JSON")
		assert.Contains(t, prompt, "questions")
		assert.Contains(t, prompt, "type")
		assert.Contains(t, prompt, "options")

		// Should contain the correct question types from lexicon
		assert.Contains(t, prompt, "single")
		assert.Contains(t, prompt, "multi")
		assert.Contains(t, prompt, "text")

		// Should contain security/safety instructions
		assert.Contains(t, prompt, "safe")
	})
}
