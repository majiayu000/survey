//go:build e2e

package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	testcontainers "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	// Container images
	// PLC from fakermaker (DID registry)
	plcImage = "ghcr.io/bluesky-social/did-method-plc:plc-c54aea0373e65df0b87f5bc81710007092f539b1"
	// PDS from bluesky-social/pds repo - supports validate=false for custom lexicons
	pdsImage = "ghcr.io/bluesky-social/pds:0.4"

	// Database configuration (for PLC)
	postgresUser     = "bsky"
	postgresPassword = "yksb"
	postgresPort     = "6543"

	// PDS configuration
	adminPassword    = "test-admin-password"
	availableDomains = ".test"
	pdsPort          = "3000"
)

// ATProtoContainers manages the lifecycle of ATProto testcontainers (PostgreSQL, PLC, PDS)
type ATProtoContainers struct {
	postgresC testcontainers.Container
	plcC      testcontainers.Container
	pdsC      testcontainers.Container

	PDSUrl string // HTTP URL to PDS server
	PLCUrl string // HTTP URL to PLC server

	adminPassword string
}

// createAccountRequest matches the PDS createAccount XRPC request
type createAccountRequest struct {
	Email      string `json:"email"`
	Handle     string `json:"handle"`
	Password   string `json:"password"`
	InviteCode string `json:"inviteCode"`
}

// createAccountResponse matches the PDS createAccount XRPC response
type createAccountResponse struct {
	DID          string `json:"did"`
	Handle       string `json:"handle"`
	AccessJwt    string `json:"accessJwt"`
	RefreshJwt   string `json:"refreshJwt"`
}

// createSessionRequest matches the com.atproto.server.createSession XRPC request
type createSessionRequest struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

// createSessionResponse matches the com.atproto.server.createSession XRPC response
type createSessionResponse struct {
	DID          string `json:"did"`
	Handle       string `json:"handle"`
	AccessJwt    string `json:"accessJwt"`
	RefreshJwt   string `json:"refreshJwt"`
}

// NewATProtoContainers starts all required ATProto containers for e2e testing
func NewATProtoContainers(ctx context.Context) (*ATProtoContainers, error) {
	containers := &ATProtoContainers{
		adminPassword: adminPassword,
	}

	// Create a shared network for all containers
	// Use a unique network name to avoid conflicts between tests
	networkName := fmt.Sprintf("atproto-test-network-%d", time.Now().UnixNano())
	network, err := testcontainers.GenericNetwork(ctx, testcontainers.GenericNetworkRequest{
		NetworkRequest: testcontainers.NetworkRequest{
			Name:           networkName,
			CheckDuplicate: false,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create network: %w", err)
	}

	// Step 1: Start PostgreSQL with multiple databases
	postgresC, err := postgres.Run(ctx,
		"postgres:14.3-alpine",
		postgres.WithUsername(postgresUser),
		postgres.WithPassword(postgresPassword),
		postgres.WithDatabase("postgres"), // Default database
		postgres.WithInitScripts("../../testdata/init-atproto-dbs.sql"),
		testcontainers.WithEnv(map[string]string{
			"POSTGRES_USER":     postgresUser,
			"POSTGRES_PASSWORD": postgresPassword,
		}),
		testcontainers.WithCmd("-p", postgresPort),
		testcontainers.WithExposedPorts(postgresPort+"/tcp"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second)),
		testcontainers.CustomizeRequest(testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Name:     "postgres-db",
				Networks: []string{networkName},
				NetworkAliases: map[string][]string{
					networkName: {"db", "postgres-db"},
				},
			},
		}),
	)
	if err != nil {
		network.Remove(ctx)
		return nil, fmt.Errorf("failed to start PostgreSQL container: %w", err)
	}
	containers.postgresC = postgresC

	// Containers on the same network can reference each other by network alias
	postgresInternalHost := "db"

	// Step 2: Start PLC (DID registry)
	plcReq := testcontainers.ContainerRequest{
		Image:        plcImage,
		Name:         "plc-server",
		ExposedPorts: []string{"2582/tcp"},
		Env: map[string]string{
			"ENV":          "dev",
			"DATABASE_URL": fmt.Sprintf("postgres://%s:%s@%s:%s/plc_dev", postgresUser, postgresPassword, postgresInternalHost, postgresPort),
			"PORT":         "2582",
			"DEBUG_MODE":   "1",
			"LOG_ENABLED":  "true",
			"LOG_LEVEL":    "info",
		},
		WorkingDir: "/app/packages/server",
		Cmd:        []string{"yarn", "run", "start"},
		WaitingFor: wait.ForLog("PLC server is running").
			WithStartupTimeout(60 * time.Second),
		Networks: []string{networkName},
		NetworkAliases: map[string][]string{
			networkName: {"plc", "plc-server"},
		},
	}

	plcC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: plcReq,
		Started:          true,
	})
	if err != nil {
		postgresC.Terminate(ctx)
		network.Remove(ctx)
		return nil, fmt.Errorf("failed to start PLC container: %w", err)
	}
	containers.plcC = plcC

	// Get PLC URL for external access
	plcHost, err := plcC.Host(ctx)
	if err != nil {
		containers.Cleanup()
		network.Remove(ctx)
		return nil, fmt.Errorf("failed to get PLC host: %w", err)
	}
	plcPort, err := plcC.MappedPort(ctx, "2582")
	if err != nil {
		containers.Cleanup()
		network.Remove(ctx)
		return nil, fmt.Errorf("failed to get PLC port: %w", err)
	}
	containers.PLCUrl = fmt.Sprintf("http://%s:%s", plcHost, plcPort.Port())

	// PLC internal URL for PDS (using network alias)
	plcInternalUrl := "http://plc:2582"

	// Step 3: Start PDS (Personal Data Server)
	// Using the official bluesky-social/pds image which supports validate=false for custom lexicons
	pdsReq := testcontainers.ContainerRequest{
		Image:        pdsImage,
		Name:         "pds-server",
		ExposedPorts: []string{pdsPort + "/tcp"},
		Env: map[string]string{
			// Dev mode - allows HTTP instead of HTTPS
			"PDS_DEV_MODE": "true",
			// Core configuration
			"PDS_HOSTNAME":      "localhost",
			"PDS_PORT":          pdsPort,
			"PDS_JWT_SECRET":    "test-jwt-secret-at-least-32-chars-long-for-security",
			"PDS_ADMIN_PASSWORD": adminPassword,
			// Use a test rotation key (32 bytes hex = 64 chars)
			"PDS_PLC_ROTATION_KEY_K256_PRIVATE_KEY_HEX": "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			// Storage (SQLite-based)
			"PDS_DATA_DIRECTORY":         "/pds",
			"PDS_BLOBSTORE_DISK_LOCATION": "/pds/blocks",
			// Identity (DID PLC)
			"PDS_DID_PLC_URL": plcInternalUrl,
			// Handle domains
			"PDS_SERVICE_HANDLE_DOMAINS": availableDomains,
			// Disable features not needed for testing
			"PDS_INVITE_REQUIRED":      "false",
			"PDS_EMAIL_SMTP_URL":       "",
			"PDS_CRAWLERS":             "",
			"PDS_BSKY_APP_VIEW_URL":    "",
			"PDS_BSKY_APP_VIEW_DID":    "",
			"PDS_REPORT_SERVICE_URL":   "",
			"PDS_REPORT_SERVICE_DID":   "",
			"PDS_MOD_SERVICE_URL":      "",
			"PDS_MOD_SERVICE_DID":      "",
			// Logging
			"LOG_ENABLED": "true",
			"LOG_LEVEL":   "info",
		},
		// Use tmpfs mount for /pds data directory (SQLite storage)
		Tmpfs: map[string]string{
			"/pds": "rw,noexec,nosuid,size=100m",
		},
		WaitingFor: wait.ForHTTP("/xrpc/_health").
			WithPort(pdsPort + "/tcp").
			WithStartupTimeout(90 * time.Second),
		Networks: []string{networkName},
		NetworkAliases: map[string][]string{
			networkName: {"pds", "pds-server"},
		},
	}

	pdsC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: pdsReq,
		Started:          true,
	})
	if err != nil {
		containers.Cleanup()
		network.Remove(ctx)
		return nil, fmt.Errorf("failed to start PDS container: %w", err)
	}
	containers.pdsC = pdsC

	// Get PDS URL for external access
	pdsHost, err := pdsC.Host(ctx)
	if err != nil {
		containers.Cleanup()
		network.Remove(ctx)
		return nil, fmt.Errorf("failed to get PDS host: %w", err)
	}
	pdsMappedPort, err := pdsC.MappedPort(ctx, pdsPort)
	if err != nil {
		containers.Cleanup()
		network.Remove(ctx)
		return nil, fmt.Errorf("failed to get PDS port: %w", err)
	}
	containers.PDSUrl = fmt.Sprintf("http://%s:%s", pdsHost, pdsMappedPort.Port())

	return containers, nil
}

// CreateTestAccount creates a new test account on the PDS
// In test mode with PDS_INVITE_REQUIRED=false, no invite code is needed
func (c *ATProtoContainers) CreateTestAccount(handle, password string) (string, error) {
	email := fmt.Sprintf("%s@example.com", handle)

	reqBody := createAccountRequest{
		Email:    email,
		Handle:   handle,
		Password: password,
		// InviteCode left empty since PDS_INVITE_REQUIRED=false
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/xrpc/com.atproto.server.createAccount", c.PDSUrl)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("createAccount failed with status %d: %s", resp.StatusCode, string(body))
	}

	var createResp createAccountResponse
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return createResp.DID, nil
}

// CreateSession authenticates with handle/password and returns access and refresh tokens
func (c *ATProtoContainers) CreateSession(handle, password string) (accessToken, refreshToken string, err error) {
	reqBody := createSessionRequest{
		Identifier: handle,
		Password:   password,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/xrpc/com.atproto.server.createSession", c.PDSUrl)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("createSession failed with status %d: %s", resp.StatusCode, string(body))
	}

	var sessionResp createSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&sessionResp); err != nil {
		return "", "", fmt.Errorf("failed to decode response: %w", err)
	}

	return sessionResp.AccessJwt, sessionResp.RefreshJwt, nil
}

// createRecordRequest matches the com.atproto.repo.createRecord XRPC request
type createRecordRequest struct {
	Repo       string                 `json:"repo"`
	Collection string                 `json:"collection"`
	Rkey       string                 `json:"rkey,omitempty"`
	Record     map[string]interface{} `json:"record"`
	Validate   *bool                  `json:"validate,omitempty"` // false=skip validation, true=require, nil=optimistic
}

// createRecordResponse matches the com.atproto.repo.createRecord XRPC response
type createRecordResponse struct {
	URI string `json:"uri"`
	CID string `json:"cid"`
}

// putRecordRequest matches the com.atproto.repo.putRecord XRPC request
type putRecordRequest struct {
	Repo       string                 `json:"repo"`
	Collection string                 `json:"collection"`
	Rkey       string                 `json:"rkey"`
	Record     map[string]interface{} `json:"record"`
	SwapRecord *string                `json:"swapRecord,omitempty"`
	Validate   *bool                  `json:"validate,omitempty"` // false=skip validation, true=require, nil=optimistic
}

// putRecordResponse matches the com.atproto.repo.putRecord XRPC response
type putRecordResponse struct {
	URI string `json:"uri"`
	CID string `json:"cid"`
}

// deleteRecordRequest matches the com.atproto.repo.deleteRecord XRPC request
type deleteRecordRequest struct {
	Repo       string  `json:"repo"`
	Collection string  `json:"collection"`
	Rkey       string  `json:"rkey"`
	SwapRecord *string `json:"swapRecord,omitempty"`
}

// getRecordResponse matches the com.atproto.repo.getRecord XRPC response
type getRecordResponse struct {
	URI   string                 `json:"uri"`
	CID   string                 `json:"cid"`
	Value map[string]interface{} `json:"value"`
}

// CreateRecord creates a record in the user's repo
// Returns the URI and CID of the created record
// If rkey is empty, the PDS will generate a TID automatically
// Set validate to false to skip lexicon validation (for custom lexicons)
func (c *ATProtoContainers) CreateRecord(accessToken, collection, rkey string, record map[string]interface{}, validate *bool) (uri, cid string, err error) {
	// Extract repo (DID) from the access token by decoding it
	// For simplicity, we'll make a request to com.atproto.server.getSession to get the DID
	repo, err := c.getDIDFromToken(accessToken)
	if err != nil {
		return "", "", fmt.Errorf("failed to get DID from token: %w", err)
	}

	reqBody := createRecordRequest{
		Repo:       repo,
		Collection: collection,
		Record:     record,
		Validate:   validate,
	}

	// Only set rkey if provided (some collections don't allow custom rkeys)
	if rkey != "" {
		reqBody.Rkey = rkey
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/xrpc/com.atproto.repo.createRecord", c.PDSUrl)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("createRecord failed with status %d: %s", resp.StatusCode, string(body))
	}

	var createResp createRecordResponse
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		return "", "", fmt.Errorf("failed to decode response: %w", err)
	}

	return createResp.URI, createResp.CID, nil
}

// GetRecord retrieves a record from a repo
func (c *ATProtoContainers) GetRecord(accessToken, repo, collection, rkey string) (map[string]interface{}, error) {
	// Build query parameters
	url := fmt.Sprintf("%s/xrpc/com.atproto.repo.getRecord?repo=%s&collection=%s&rkey=%s",
		c.PDSUrl, repo, collection, rkey)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// getRecord is public, but we can optionally use auth if provided
	if accessToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("getRecord failed with status %d: %s", resp.StatusCode, string(body))
	}

	var getResp getRecordResponse
	if err := json.NewDecoder(resp.Body).Decode(&getResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return getResp.Value, nil
}

// PutRecord updates a record (or creates it with specific rkey)
// Set validate to false to skip lexicon validation (for custom lexicons)
func (c *ATProtoContainers) PutRecord(accessToken, collection, rkey string, record map[string]interface{}, swapRecord *string, validate *bool) (uri, cid string, err error) {
	// Extract repo (DID) from the access token
	repo, err := c.getDIDFromToken(accessToken)
	if err != nil {
		return "", "", fmt.Errorf("failed to get DID from token: %w", err)
	}

	reqBody := putRecordRequest{
		Repo:       repo,
		Collection: collection,
		Rkey:       rkey,
		Record:     record,
		SwapRecord: swapRecord,
		Validate:   validate,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/xrpc/com.atproto.repo.putRecord", c.PDSUrl)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("putRecord failed with status %d: %s", resp.StatusCode, string(body))
	}

	var putResp putRecordResponse
	if err := json.NewDecoder(resp.Body).Decode(&putResp); err != nil {
		return "", "", fmt.Errorf("failed to decode response: %w", err)
	}

	return putResp.URI, putResp.CID, nil
}

// DeleteRecord deletes a record from the user's repo
func (c *ATProtoContainers) DeleteRecord(accessToken, collection, rkey string, swapRecord *string) error {
	// Extract repo (DID) from the access token
	repo, err := c.getDIDFromToken(accessToken)
	if err != nil {
		return fmt.Errorf("failed to get DID from token: %w", err)
	}

	reqBody := deleteRecordRequest{
		Repo:       repo,
		Collection: collection,
		Rkey:       rkey,
		SwapRecord: swapRecord,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/xrpc/com.atproto.repo.deleteRecord", c.PDSUrl)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("deleteRecord failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// getSessionResponse matches the com.atproto.server.getSession XRPC response
type getSessionResponse struct {
	DID    string `json:"did"`
	Handle string `json:"handle"`
}

// getDIDFromToken retrieves the DID associated with an access token
func (c *ATProtoContainers) getDIDFromToken(accessToken string) (string, error) {
	url := fmt.Sprintf("%s/xrpc/com.atproto.server.getSession", c.PDSUrl)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("getSession failed with status %d: %s", resp.StatusCode, string(body))
	}

	var sessionResp getSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&sessionResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return sessionResp.DID, nil
}

// Cleanup terminates all containers
func (c *ATProtoContainers) Cleanup() {
	ctx := context.Background()

	if c.pdsC != nil {
		c.pdsC.Terminate(ctx)
	}
	if c.plcC != nil {
		c.plcC.Terminate(ctx)
	}
	if c.postgresC != nil {
		c.postgresC.Terminate(ctx)
	}
}
