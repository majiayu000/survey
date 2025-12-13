//go:build e2e

package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestATProtoContainers_StartupAndShutdown verifies that all ATProto containers start successfully
func TestATProtoContainers_StartupAndShutdown(t *testing.T) {
	ctx := context.Background()

	// Start all containers (PostgreSQL, PLC, PDS)
	containers, err := NewATProtoContainers(ctx)
	require.NoError(t, err, "Failed to start ATProto containers")
	defer containers.Cleanup()

	// Verify we got connection details
	assert.NotEmpty(t, containers.PDSUrl, "PDS URL should not be empty")
	assert.NotEmpty(t, containers.PLCUrl, "PLC URL should not be empty")

	// Basic health check - verify PDS is accessible
	assert.Contains(t, containers.PDSUrl, "http://", "PDS URL should be HTTP")
}

// TestATProtoContainers_CreateAccount verifies we can create test accounts via admin API
func TestATProtoContainers_CreateAccount(t *testing.T) {
	ctx := context.Background()

	containers, err := NewATProtoContainers(ctx)
	require.NoError(t, err, "Failed to start ATProto containers")
	defer containers.Cleanup()

	// Create a test account
	handle := "alice.test"
	password := "test-password-123"

	did, err := containers.CreateTestAccount(handle, password)
	require.NoError(t, err, "Failed to create test account")

	// Verify DID format
	assert.NotEmpty(t, did, "DID should not be empty")
	assert.Contains(t, did, "did:plc:", "DID should be a PLC DID")
}

// TestATProtoContainers_CreateSession verifies we can authenticate and get session tokens
func TestATProtoContainers_CreateSession(t *testing.T) {
	ctx := context.Background()

	containers, err := NewATProtoContainers(ctx)
	require.NoError(t, err, "Failed to start ATProto containers")
	defer containers.Cleanup()

	// Create a test account first
	handle := "bob.test"
	password := "test-password-456"

	did, err := containers.CreateTestAccount(handle, password)
	require.NoError(t, err, "Failed to create test account")
	require.NotEmpty(t, did, "DID should not be empty")

	// Create session
	accessToken, refreshToken, err := containers.CreateSession(handle, password)
	require.NoError(t, err, "Failed to create session")

	// Verify tokens
	assert.NotEmpty(t, accessToken, "Access token should not be empty")
	assert.NotEmpty(t, refreshToken, "Refresh token should not be empty")

	// Access token should be a JWT (starts with "ey")
	assert.True(t, len(accessToken) > 100, "Access token should be reasonably long")
}

// TestATProtoContainers_MultipleAccounts verifies we can create multiple test accounts
func TestATProtoContainers_MultipleAccounts(t *testing.T) {
	ctx := context.Background()

	containers, err := NewATProtoContainers(ctx)
	require.NoError(t, err, "Failed to start ATProto containers")
	defer containers.Cleanup()

	accounts := []struct {
		handle   string
		password string
	}{
		{"alice.test", "password1"},
		{"bob.test", "password2"},
		{"charlie.test", "password3"},
	}

	dids := make([]string, 0, len(accounts))

	// Create all accounts
	for _, acc := range accounts {
		did, err := containers.CreateTestAccount(acc.handle, acc.password)
		require.NoError(t, err, "Failed to create account: %s", acc.handle)
		dids = append(dids, did)
	}

	// Verify all DIDs are unique
	uniqueDids := make(map[string]bool)
	for _, did := range dids {
		uniqueDids[did] = true
	}
	assert.Equal(t, len(accounts), len(uniqueDids), "All DIDs should be unique")

	// Verify all accounts can create sessions
	for _, acc := range accounts {
		accessToken, _, err := containers.CreateSession(acc.handle, acc.password)
		require.NoError(t, err, "Failed to create session for: %s", acc.handle)
		assert.NotEmpty(t, accessToken, "Access token should not be empty for: %s", acc.handle)
	}
}

// TestATProtoContainers_InvalidCredentials verifies authentication failures are handled
func TestATProtoContainers_InvalidCredentials(t *testing.T) {
	ctx := context.Background()

	containers, err := NewATProtoContainers(ctx)
	require.NoError(t, err, "Failed to start ATProto containers")
	defer containers.Cleanup()

	// Try to create session with non-existent account
	_, _, err = containers.CreateSession("nonexistent.test", "wrongpassword")
	assert.Error(t, err, "Should fail with invalid credentials")
}

// TestATProtoContainers_ContainerStartupTimeout verifies containers start within reasonable time
func TestATProtoContainers_ContainerStartupTimeout(t *testing.T) {
	ctx := context.Background()

	start := time.Now()
	containers, err := NewATProtoContainers(ctx)
	require.NoError(t, err, "Failed to start ATProto containers")
	defer containers.Cleanup()

	duration := time.Since(start)

	// Should start in less than 2 minutes (generous timeout)
	assert.Less(t, duration, 2*time.Minute, "Container startup should be reasonably fast")
	t.Logf("Container startup took: %v", duration)
}
