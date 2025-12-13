//go:build e2e

package testutil

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests use custom net.openmeet.survey lexicons with the official bluesky-social/pds image.
//
// REQUIREMENTS:
// - PDS image: ghcr.io/bluesky-social/pds:0.4 (or newer)
// - PDS_DEV_MODE=true (allows HTTP instead of HTTPS)
// - validate=false parameter on CreateRecord/PutRecord calls (skips lexicon validation)
//
// The tests validate:
// 1. Our lexicon schema design (net.openmeet.survey, .response, .results)
// 2. Full CRUD operations on custom lexicons
// 3. PDS integration with custom data types

// validateFalse is used to skip lexicon validation for custom lexicons
var validateFalse = false

// extractRkeyFromURI extracts the rkey from an AT URI
// Example: at://did:plc:abc123/app.bsky.feed.post/3jzfcijpj2z2a -> 3jzfcijpj2z2a
func extractRkeyFromURI(uri string) string {
	parts := strings.Split(uri, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

// TestATProto_CreateRecord verifies we can create a survey record in a user's repo
func TestATProto_CreateRecord(t *testing.T) {
	ctx := context.Background()

	containers, err := NewATProtoContainers(ctx)
	require.NoError(t, err, "Failed to start ATProto containers")
	defer containers.Cleanup()

	// Create test account and get access token
	handle := "alice.test"
	password := "test-password-123"

	did, err := containers.CreateTestAccount(handle, password)
	require.NoError(t, err, "Failed to create test account")

	accessToken, _, err := containers.CreateSession(handle, password)
	require.NoError(t, err, "Failed to create session")

	// Create a survey record (using net.openmeet.survey lexicon)
	// Let PDS generate a TID by passing empty rkey
	collection := "net.openmeet.survey"
	record := map[string]interface{}{
		"$type":      collection,
		"name":       "What is your favorite color?",
		"description": "A simple color preference survey",
		"questions": []map[string]interface{}{
			{
				"id":       "q1",
				"text":     "Choose your favorite color",
				"type":     "net.openmeet.survey#single",
				"required": true,
				"options": []map[string]interface{}{
					{"id": "red", "text": "Red"},
					{"id": "blue", "text": "Blue"},
					{"id": "green", "text": "Green"},
				},
			},
		},
		"anonymous": false,
		"createdAt": time.Now().UTC().Format(time.RFC3339),
	}

	uri, cid, err := containers.CreateRecord(accessToken, collection, "", record, &validateFalse)
	require.NoError(t, err, "Failed to create survey record")

	// Verify URI format: at://did:plc:xxx/net.openmeet.survey/{tid}
	assert.NotEmpty(t, uri, "URI should not be empty")
	assert.Contains(t, uri, "at://", "URI should be AT URI")
	assert.Contains(t, uri, did, "URI should contain user DID")
	assert.Contains(t, uri, collection, "URI should contain collection")

	// Verify CID is returned
	assert.NotEmpty(t, cid, "CID should not be empty")
}

// TestATProto_GetRecord verifies we can retrieve a survey record
func TestATProto_GetRecord(t *testing.T) {
	ctx := context.Background()

	containers, err := NewATProtoContainers(ctx)
	require.NoError(t, err, "Failed to start ATProto containers")
	defer containers.Cleanup()

	// Create test account and get access token
	handle := "bob.test"
	password := "test-password-456"

	did, err := containers.CreateTestAccount(handle, password)
	require.NoError(t, err, "Failed to create test account")

	accessToken, _, err := containers.CreateSession(handle, password)
	require.NoError(t, err, "Failed to create session")

	// Create a survey record first
	collection := "net.openmeet.survey"
	originalName := "Favorite Programming Language"
	record := map[string]interface{}{
		"$type": collection,
		"name":  originalName,
		"questions": []map[string]interface{}{
			{
				"id":       "q1",
				"text":     "Which language do you prefer?",
				"type":     "net.openmeet.survey#single",
				"required": true,
				"options": []map[string]interface{}{
					{"id": "go", "text": "Go"},
					{"id": "rust", "text": "Rust"},
					{"id": "python", "text": "Python"},
				},
			},
		},
		"createdAt": time.Now().UTC().Format(time.RFC3339),
	}

	uri, _, err := containers.CreateRecord(accessToken, collection, "", record, &validateFalse)
	require.NoError(t, err, "Failed to create survey record")

	// Extract rkey from URI: at://did:plc:xxx/collection/rkey
	rkey := extractRkeyFromURI(uri)
	require.NotEmpty(t, rkey, "Should extract rkey from URI")

	// Get the record
	retrieved, err := containers.GetRecord(accessToken, did, collection, rkey)
	require.NoError(t, err, "Failed to get survey record")

	// Verify the record contents
	assert.Equal(t, collection, retrieved["$type"], "Record should have correct type")
	assert.Equal(t, originalName, retrieved["name"], "Record should have correct name")
	assert.NotEmpty(t, retrieved["createdAt"], "Record should have createdAt")
	assert.NotEmpty(t, retrieved["questions"], "Record should have questions")
}

// TestATProto_UpdateRecord verifies we can update a survey record using putRecord
func TestATProto_UpdateRecord(t *testing.T) {
	ctx := context.Background()

	containers, err := NewATProtoContainers(ctx)
	require.NoError(t, err, "Failed to start ATProto containers")
	defer containers.Cleanup()

	// Create test account and get access token
	handle := "charlie.test"
	password := "test-password-789"

	did, err := containers.CreateTestAccount(handle, password)
	require.NoError(t, err, "Failed to create test account")

	accessToken, _, err := containers.CreateSession(handle, password)
	require.NoError(t, err, "Failed to create session")

	// Create a survey record with a specific rkey for testing updates
	collection := "net.openmeet.survey"
	rkey := "test-survey-123"

	// Create initial survey
	originalName := "Original Survey Name"
	record := map[string]interface{}{
		"$type": collection,
		"name":  originalName,
		"questions": []map[string]interface{}{
			{
				"id":       "q1",
				"text":     "Original question?",
				"type":     "net.openmeet.survey#single",
				"required": true,
				"options": []map[string]interface{}{
					{"id": "opt1", "text": "Option 1"},
				},
			},
		},
		"createdAt": time.Now().UTC().Format(time.RFC3339),
	}

	uri, originalCid, err := containers.PutRecord(accessToken, collection, rkey, record, nil, &validateFalse)
	require.NoError(t, err, "Failed to create survey")
	assert.NotEmpty(t, uri, "URI should not be empty")
	assert.NotEmpty(t, originalCid, "CID should not be empty")

	// Update the survey - change name and add a question
	updatedName := "Updated Survey Name"
	record["name"] = updatedName
	record["questions"] = []map[string]interface{}{
		{
			"id":       "q1",
			"text":     "Original question?",
			"type":     "net.openmeet.survey#single",
			"required": true,
			"options": []map[string]interface{}{
				{"id": "opt1", "text": "Option 1"},
			},
		},
		{
			"id":       "q2",
			"text":     "New question added!",
			"type":     "net.openmeet.survey#multi",
			"required": false,
			"options": []map[string]interface{}{
				{"id": "opt2a", "text": "Choice A"},
				{"id": "opt2b", "text": "Choice B"},
			},
		},
	}

	updatedURI, newCid, err := containers.PutRecord(accessToken, collection, rkey, record, &originalCid, &validateFalse)
	require.NoError(t, err, "Failed to update survey")

	// Verify CID changed
	assert.NotEqual(t, originalCid, newCid, "CID should change after update")
	assert.Contains(t, updatedURI, rkey, "URI should contain rkey")

	// Verify the updated record
	retrieved, err := containers.GetRecord(accessToken, did, collection, rkey)
	require.NoError(t, err, "Failed to get updated survey")

	assert.Equal(t, updatedName, retrieved["name"], "Survey should have updated name")
	questions := retrieved["questions"].([]interface{})
	assert.Len(t, questions, 2, "Survey should have 2 questions after update")
}

// TestATProto_DeleteRecord verifies we can delete a survey record
func TestATProto_DeleteRecord(t *testing.T) {
	ctx := context.Background()

	containers, err := NewATProtoContainers(ctx)
	require.NoError(t, err, "Failed to start ATProto containers")
	defer containers.Cleanup()

	// Create test account and get access token
	handle := "dave.test"
	password := "test-password-abc"

	did, err := containers.CreateTestAccount(handle, password)
	require.NoError(t, err, "Failed to create test account")

	accessToken, _, err := containers.CreateSession(handle, password)
	require.NoError(t, err, "Failed to create session")

	// Create a survey record
	collection := "net.openmeet.survey"
	record := map[string]interface{}{
		"$type": collection,
		"name":  "Survey to Delete",
		"questions": []map[string]interface{}{
			{
				"id":       "q1",
				"text":     "Delete me?",
				"type":     "net.openmeet.survey#single",
				"required": true,
				"options": []map[string]interface{}{
					{"id": "yes", "text": "Yes"},
					{"id": "no", "text": "No"},
				},
			},
		},
		"createdAt": time.Now().UTC().Format(time.RFC3339),
	}

	uri, cid, err := containers.CreateRecord(accessToken, collection, "", record, &validateFalse)
	require.NoError(t, err, "Failed to create survey record")

	// Extract rkey from URI
	rkey := extractRkeyFromURI(uri)
	require.NotEmpty(t, rkey, "Should extract rkey from URI")

	// Verify record exists
	_, err = containers.GetRecord(accessToken, did, collection, rkey)
	require.NoError(t, err, "Survey record should exist before deletion")

	// Delete the record
	err = containers.DeleteRecord(accessToken, collection, rkey, &cid)
	require.NoError(t, err, "Failed to delete survey record")

	// Verify record is gone (PDS may return 400 or 404 for deleted/non-existent records)
	_, err = containers.GetRecord(accessToken, did, collection, rkey)
	assert.Error(t, err, "Survey record should not exist after deletion")
	assert.Contains(t, strings.ToLower(err.Error()), "could not locate record", "Should get error for deleted record")
}

// TestATProto_FullCRUDCycle tests full lifecycle with survey, response, and results records
func TestATProto_FullCRUDCycle(t *testing.T) {
	ctx := context.Background()

	containers, err := NewATProtoContainers(ctx)
	require.NoError(t, err, "Failed to start ATProto containers")
	defer containers.Cleanup()

	// Create test account and get access token
	handle := "eve.test"
	password := "test-password-xyz"

	did, err := containers.CreateTestAccount(handle, password)
	require.NoError(t, err, "Failed to create test account")

	accessToken, _, err := containers.CreateSession(handle, password)
	require.NoError(t, err, "Failed to create session")

	// 1. CREATE a survey
	t.Log("Step 1: CREATE survey record")
	surveyCollection := "net.openmeet.survey"
	originalName := "Full CRUD Test Survey"
	surveyRecord := map[string]interface{}{
		"$type":       surveyCollection,
		"name":        originalName,
		"description": "Testing complete survey workflow",
		"questions": []map[string]interface{}{
			{
				"id":       "q1",
				"text":     "What is your favorite framework?",
				"type":     "net.openmeet.survey#single",
				"required": true,
				"options": []map[string]interface{}{
					{"id": "react", "text": "React"},
					{"id": "vue", "text": "Vue"},
					{"id": "svelte", "text": "Svelte"},
				},
			},
		},
		"anonymous": false,
		"createdAt": time.Now().UTC().Format(time.RFC3339),
	}

	surveyURI, surveyCid, err := containers.CreateRecord(accessToken, surveyCollection, "", surveyRecord, &validateFalse)
	require.NoError(t, err, "CREATE survey failed")
	assert.NotEmpty(t, surveyURI, "Survey URI should not be empty")
	assert.NotEmpty(t, surveyCid, "Survey CID should not be empty")

	surveyRkey := extractRkeyFromURI(surveyURI)
	require.NotEmpty(t, surveyRkey, "Should extract rkey from survey URI")

	// 2. READ the survey (verify created)
	t.Log("Step 2: READ created survey")
	retrieved, err := containers.GetRecord(accessToken, did, surveyCollection, surveyRkey)
	require.NoError(t, err, "READ survey failed")
	assert.Equal(t, originalName, retrieved["name"], "Survey name should match")

	// 3. UPDATE the survey (add a question)
	t.Log("Step 3: UPDATE survey (add question)")
	updatedName := "Updated CRUD Test Survey"
	surveyRecord["name"] = updatedName
	surveyRecord["questions"] = []map[string]interface{}{
		{
			"id":       "q1",
			"text":     "What is your favorite framework?",
			"type":     "net.openmeet.survey#single",
			"required": true,
			"options": []map[string]interface{}{
				{"id": "react", "text": "React"},
				{"id": "vue", "text": "Vue"},
				{"id": "svelte", "text": "Svelte"},
			},
		},
		{
			"id":       "q2",
			"text":     "How many years of experience?",
			"type":     "net.openmeet.survey#single",
			"required": false,
			"options": []map[string]interface{}{
				{"id": "0-2", "text": "0-2 years"},
				{"id": "3-5", "text": "3-5 years"},
				{"id": "5plus", "text": "5+ years"},
			},
		},
	}

	_, newSurveyCid, err := containers.PutRecord(accessToken, surveyCollection, surveyRkey, surveyRecord, &surveyCid, &validateFalse)
	require.NoError(t, err, "UPDATE survey failed")
	assert.NotEqual(t, surveyCid, newSurveyCid, "Survey CID should change after update")

	// 4. READ updated survey
	t.Log("Step 4: READ updated survey")
	retrieved, err = containers.GetRecord(accessToken, did, surveyCollection, surveyRkey)
	require.NoError(t, err, "READ updated survey failed")
	assert.Equal(t, updatedName, retrieved["name"], "Survey name should be updated")
	questions := retrieved["questions"].([]interface{})
	assert.Len(t, questions, 2, "Survey should have 2 questions after update")

	// 5. CREATE a response to the survey (using strongRef)
	t.Log("Step 5: CREATE response to survey")
	responseCollection := "net.openmeet.survey.response"
	responseRecord := map[string]interface{}{
		"$type": responseCollection,
		"subject": map[string]interface{}{
			"uri": surveyURI,
			"cid": newSurveyCid,
		},
		"answers": []map[string]interface{}{
			{
				"questionId":      "q1",
				"selectedOptions": []string{"vue"},
			},
			{
				"questionId":      "q2",
				"selectedOptions": []string{"3-5"},
			},
		},
		"createdAt": time.Now().UTC().Format(time.RFC3339),
	}

	responseURI, responseCid, err := containers.CreateRecord(accessToken, responseCollection, "", responseRecord, &validateFalse)
	require.NoError(t, err, "CREATE response failed")
	assert.NotEmpty(t, responseURI, "Response URI should not be empty")
	assert.NotEmpty(t, responseCid, "Response CID should not be empty")

	responseRkey := extractRkeyFromURI(responseURI)
	require.NotEmpty(t, responseRkey, "Should extract rkey from response URI")

	// 6. READ the response
	t.Log("Step 6: READ response")
	retrieved, err = containers.GetRecord(accessToken, did, responseCollection, responseRkey)
	require.NoError(t, err, "READ response failed")
	assert.Equal(t, responseCollection, retrieved["$type"], "Response should have correct type")
	subject := retrieved["subject"].(map[string]interface{})
	assert.Equal(t, surveyURI, subject["uri"], "Response should reference survey URI")

	// 7. CREATE survey results
	t.Log("Step 7: CREATE results for survey")
	resultsCollection := "net.openmeet.survey.results"
	resultsRecord := map[string]interface{}{
		"$type": resultsCollection,
		"subject": map[string]interface{}{
			"uri": surveyURI,
			"cid": newSurveyCid,
		},
		"totalVotes": 1,
		"questionResults": []map[string]interface{}{
			{
				"questionId": "q1",
				"optionCounts": []map[string]interface{}{
					{"optionId": "react", "count": 0},
					{"optionId": "vue", "count": 1},
					{"optionId": "svelte", "count": 0},
				},
			},
			{
				"questionId": "q2",
				"optionCounts": []map[string]interface{}{
					{"optionId": "0-2", "count": 0},
					{"optionId": "3-5", "count": 1},
					{"optionId": "5plus", "count": 0},
				},
			},
		},
		"finalizedAt": time.Now().UTC().Format(time.RFC3339),
	}

	resultsURI, resultsCid, err := containers.CreateRecord(accessToken, resultsCollection, "", resultsRecord, &validateFalse)
	require.NoError(t, err, "CREATE results failed")
	assert.NotEmpty(t, resultsURI, "Results URI should not be empty")
	assert.NotEmpty(t, resultsCid, "Results CID should not be empty")

	resultsRkey := extractRkeyFromURI(resultsURI)
	require.NotEmpty(t, resultsRkey, "Should extract rkey from results URI")

	// 8. READ the results
	t.Log("Step 8: READ results")
	retrieved, err = containers.GetRecord(accessToken, did, resultsCollection, resultsRkey)
	require.NoError(t, err, "READ results failed")
	assert.Equal(t, resultsCollection, retrieved["$type"], "Results should have correct type")
	assert.Equal(t, float64(1), retrieved["totalVotes"], "Results should show 1 vote")

	// 9. DELETE all records (cleanup in reverse order)
	t.Log("Step 9: DELETE results")
	err = containers.DeleteRecord(accessToken, resultsCollection, resultsRkey, &resultsCid)
	require.NoError(t, err, "DELETE results failed")

	t.Log("Step 10: DELETE response")
	err = containers.DeleteRecord(accessToken, responseCollection, responseRkey, &responseCid)
	require.NoError(t, err, "DELETE response failed")

	t.Log("Step 11: DELETE survey")
	err = containers.DeleteRecord(accessToken, surveyCollection, surveyRkey, &newSurveyCid)
	require.NoError(t, err, "DELETE survey failed")

	// 10. VERIFY all records deleted
	t.Log("Step 12: VERIFY all records deleted")
	_, err = containers.GetRecord(accessToken, did, surveyCollection, surveyRkey)
	assert.Error(t, err, "Survey should not exist after deletion")

	_, err = containers.GetRecord(accessToken, did, responseCollection, responseRkey)
	assert.Error(t, err, "Response should not exist after deletion")

	_, err = containers.GetRecord(accessToken, did, resultsCollection, resultsRkey)
	assert.Error(t, err, "Results should not exist after deletion")
}
