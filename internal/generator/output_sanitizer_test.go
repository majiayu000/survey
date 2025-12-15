package generator

import (
	"encoding/json"
	"testing"

	"github.com/openmeet-team/survey/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutputSanitizer_Sanitize(t *testing.T) {
	sanitizer := NewOutputSanitizer()

	t.Run("valid JSON survey definition", func(t *testing.T) {
		validJSON := `{
			"questions": [
				{
					"id": "q1",
					"text": "What's your favorite color?",
					"type": "single",
					"required": true,
					"options": [
						{"id": "opt1", "text": "Red"},
						{"id": "opt2", "text": "Blue"}
					]
				}
			],
			"anonymous": false
		}`

		def, err := sanitizer.Sanitize(validJSON)
		require.NoError(t, err)
		assert.NotNil(t, def)
		assert.Len(t, def.Questions, 1)
		assert.Equal(t, "q1", def.Questions[0].ID)
		assert.Equal(t, "What's your favorite color?", def.Questions[0].Text)
		assert.Equal(t, models.QuestionTypeSingle, def.Questions[0].Type)
		assert.Len(t, def.Questions[0].Options, 2)
	})

	t.Run("sanitizes HTML in question text", func(t *testing.T) {
		jsonWithHTML := `{
			"questions": [
				{
					"id": "q1",
					"text": "<script>alert('xss')</script>What is your name?",
					"type": "text",
					"required": false
				}
			],
			"anonymous": true
		}`

		def, err := sanitizer.Sanitize(jsonWithHTML)
		require.NoError(t, err)
		assert.NotNil(t, def)
		// SanitizeText should strip the script tag
		assert.Equal(t, "What is your name?", def.Questions[0].Text)
	})

	t.Run("sanitizes HTML in option text", func(t *testing.T) {
		jsonWithHTML := `{
			"questions": [
				{
					"id": "q1",
					"text": "Pick one",
					"type": "single",
					"required": false,
					"options": [
						{"id": "opt1", "text": "<img src=x onerror=alert(1)>Option 1"},
						{"id": "opt2", "text": "Option 2"}
					]
				}
			],
			"anonymous": false
		}`

		def, err := sanitizer.Sanitize(jsonWithHTML)
		require.NoError(t, err)
		// SanitizeText should strip the img tag
		assert.Equal(t, "Option 1", def.Questions[0].Options[0].Text)
	})

	t.Run("rejects invalid JSON", func(t *testing.T) {
		invalidJSON := `{invalid json}`
		def, err := sanitizer.Sanitize(invalidJSON)
		assert.Error(t, err)
		assert.Nil(t, def)
	})

	t.Run("rejects JSON missing required fields", func(t *testing.T) {
		missingFields := `{"questions": []}`
		def, err := sanitizer.Sanitize(missingFields)
		assert.Error(t, err)
		assert.Nil(t, def)
	})

	t.Run("rejects too many questions", func(t *testing.T) {
		// Create 51 questions (exceeds max of 50)
		questions := make([]map[string]interface{}, 51)
		for i := 0; i < 51; i++ {
			questions[i] = map[string]interface{}{
				"id":       "q" + string(rune(i)),
				"text":     "Question " + string(rune(i)),
				"type":     "text",
				"required": false,
			}
		}
		data := map[string]interface{}{
			"questions": questions,
			"anonymous": false,
		}
		jsonBytes, _ := json.Marshal(data)

		def, err := sanitizer.Sanitize(string(jsonBytes))
		assert.Error(t, err)
		assert.Nil(t, def)
	})

	t.Run("rejects too many options per question", func(t *testing.T) {
		// Create 21 options (exceeds max of 20)
		options := make([]map[string]string, 21)
		for i := 0; i < 21; i++ {
			options[i] = map[string]string{
				"id":   "opt" + string(rune(i)),
				"text": "Option " + string(rune(i)),
			}
		}
		data := map[string]interface{}{
			"questions": []map[string]interface{}{
				{
					"id":       "q1",
					"text":     "Pick one",
					"type":     "single",
					"required": false,
					"options":  options,
				},
			},
			"anonymous": false,
		}
		jsonBytes, _ := json.Marshal(data)

		def, err := sanitizer.Sanitize(string(jsonBytes))
		assert.Error(t, err)
		assert.Nil(t, def)
	})

	t.Run("accepts multi-question survey", func(t *testing.T) {
		validJSON := `{
			"questions": [
				{
					"id": "q1",
					"text": "What's your name?",
					"type": "text",
					"required": true
				},
				{
					"id": "q2",
					"text": "What's your favorite color?",
					"type": "single",
					"required": false,
					"options": [
						{"id": "red", "text": "Red"},
						{"id": "blue", "text": "Blue"},
						{"id": "green", "text": "Green"}
					]
				},
				{
					"id": "q3",
					"text": "Which languages do you speak?",
					"type": "multi",
					"required": false,
					"options": [
						{"id": "en", "text": "English"},
						{"id": "es", "text": "Spanish"},
						{"id": "fr", "text": "French"}
					]
				}
			],
			"anonymous": false
		}`

		def, err := sanitizer.Sanitize(validJSON)
		require.NoError(t, err)
		assert.NotNil(t, def)
		assert.Len(t, def.Questions, 3)
		assert.Equal(t, models.QuestionTypeText, def.Questions[0].Type)
		assert.Equal(t, models.QuestionTypeSingle, def.Questions[1].Type)
		assert.Equal(t, models.QuestionTypeMulti, def.Questions[2].Type)
	})
}
