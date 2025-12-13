//go:build e2e

package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestATProto_PutRecord_ValidateFalse tests the Statusphere approach:
// Using putRecord with validate: false for custom lexicons
//
// Key insight from Statusphere:
// - They use putRecord (not createRecord) with validate: false
// - This might bypass lexicon validation differently
func TestATProto_PutRecord_ValidateFalse(t *testing.T) {
	ctx := context.Background()

	containers, err := NewATProtoContainers(ctx)
	require.NoError(t, err, "Failed to start ATProto containers")
	defer containers.Cleanup()

	// Create test account and get access token
	handle := "puttest.test"
	password := "test-password-123"

	did, err := containers.CreateTestAccount(handle, password)
	require.NoError(t, err, "Failed to create test account")

	accessToken, _, err := containers.CreateSession(handle, password)
	require.NoError(t, err, "Failed to create session")

	// Test 1: Try putRecord with custom lexicon and validate=false (Statusphere approach)
	t.Run("custom lexicon with validate=false", func(t *testing.T) {
		collection := "net.openmeet.survey"
		rkey := "test-survey-001"
		validateFalse := false

		record := map[string]interface{}{
			"$type":       collection,
			"name":        "Test Survey via PutRecord",
			"description": "Testing Statusphere approach",
			"questions": []map[string]interface{}{
				{
					"id":       "q1",
					"text":     "Test question?",
					"type":     "net.openmeet.survey#single",
					"required": true,
					"options": []map[string]interface{}{
						{"id": "opt1", "text": "Option 1"},
					},
				},
			},
			"createdAt": time.Now().UTC().Format(time.RFC3339),
		}

		uri, cid, err := containers.PutRecord(accessToken, collection, rkey, record, nil, &validateFalse)
		if err != nil {
			t.Logf("putRecord with validate=false failed: %v", err)
			// Check if it's a different error than createRecord
			assert.Contains(t, err.Error(), "Unvalidated writes are not yet supported",
				"Expected 'Unvalidated writes are not yet supported' error")
		} else {
			t.Logf("SUCCESS! putRecord with validate=false worked!")
			t.Logf("URI: %s", uri)
			t.Logf("CID: %s", cid)
			assert.Contains(t, uri, did, "URI should contain DID")
			assert.Contains(t, uri, collection, "URI should contain collection")
		}
	})

	// Test 2: Try putRecord with known lexicon and validate=nil (optimistic)
	t.Run("known lexicon with validate=nil", func(t *testing.T) {
		collection := "app.bsky.actor.profile"
		rkey := "self" // Profile uses "self" as rkey

		record := map[string]interface{}{
			"$type":       collection,
			"displayName": "Test User",
			"description": "Testing PutRecord",
		}

		uri, cid, err := containers.PutRecord(accessToken, collection, rkey, record, nil, nil)
		if err != nil {
			t.Logf("putRecord with known lexicon failed: %v", err)
		} else {
			t.Logf("putRecord with known lexicon succeeded!")
			t.Logf("URI: %s", uri)
			t.Logf("CID: %s", cid)
			assert.Contains(t, uri, did, "URI should contain DID")
		}
	})

	// Test 3: Try createRecord with validate=false for comparison
	t.Run("createRecord with validate=false", func(t *testing.T) {
		collection := "net.openmeet.survey"
		validateFalse := false

		record := map[string]interface{}{
			"$type": collection,
			"name":  "Test Survey via CreateRecord",
			"questions": []map[string]interface{}{
				{
					"id":       "q1",
					"text":     "Test?",
					"type":     "net.openmeet.survey#single",
					"required": true,
					"options": []map[string]interface{}{
						{"id": "opt1", "text": "Option 1"},
					},
				},
			},
			"createdAt": time.Now().UTC().Format(time.RFC3339),
		}

		uri, cid, err := containers.CreateRecord(accessToken, collection, "", record, &validateFalse)
		if err != nil {
			t.Logf("createRecord with validate=false failed: %v", err)
			// Check the error message
			assert.Contains(t, err.Error(), "Unvalidated writes are not yet supported",
				"Expected 'Unvalidated writes are not yet supported' error")
		} else {
			t.Logf("SUCCESS! createRecord with validate=false worked!")
			t.Logf("URI: %s", uri)
			t.Logf("CID: %s", cid)
		}
	})
}
