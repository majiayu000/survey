package oauth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// PDSRecord represents a record from a PDS collection
type PDSRecord struct {
	URI       string                 `json:"uri"`
	CID       string                 `json:"cid"`
	Value     map[string]interface{} `json:"value"`
	ValueJSON string                 `json:"-"` // formatted JSON for display
	RKey      string                 `json:"rkey"`      // extracted from URI
	Timestamp *time.Time             `json:"timestamp"` // parsed from TID if valid
}

// ListRecordsResponse represents the response from listRecords
type ListRecordsResponse struct {
	Records []PDSRecord `json:"records"`
	Cursor  string      `json:"cursor,omitempty"`
}

// CreateRecord writes an ATProto record to the user's PDS
// Returns the AT URI and CID of the created record
// If rkey is empty, the PDS will generate one
func CreateRecord(session *OAuthSession, collection string, rkey string, record interface{}) (string, string, error) {
	if session == nil {
		return "", "", fmt.Errorf("session cannot be nil")
	}

	if session.AccessToken == "" {
		return "", "", fmt.Errorf("session missing access token")
	}

	if session.PDSUrl == "" {
		return "", "", fmt.Errorf("session missing PDS URL")
	}

	if session.DPoPKey == "" {
		return "", "", fmt.Errorf("session missing DPoP key")
	}

	// Check if token is expired
	if session.TokenExpiresAt != nil && time.Now().After(*session.TokenExpiresAt) {
		return "", "", fmt.Errorf("access token expired and no refresh token available")
	}

	// Build request payload
	// validate: false is required for custom lexicons like net.openmeet.survey
	validateFalse := false
	payload := map[string]interface{}{
		"repo":       session.DID,
		"collection": collection,
		"record":     record,
		"validate":   &validateFalse,
	}

	// Add rkey if provided
	if rkey != "" {
		payload["rkey"] = rkey
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal record: %w", err)
	}

	// Build PDS URL
	pdsURL := strings.TrimSuffix(session.PDSUrl, "/") + "/xrpc/com.atproto.repo.createRecord"

	// Create DPoP proof for POST request with access token hash
	// RFC 9449 requires "ath" claim when using DPoP with access tokens
	dpopProof, err := CreateDPoPProof(session.DPoPKey, "POST", pdsURL, "", session.AccessToken)
	if err != nil {
		return "", "", fmt.Errorf("failed to create DPoP proof: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", pdsURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "DPoP "+session.AccessToken)
	req.Header.Set("DPoP", dpopProof)

	// Execute request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("PDS request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to read response: %w", err)
	}

	// Check for DPoP nonce requirement
	if resp.StatusCode == http.StatusUnauthorized {
		dpopNonce := resp.Header.Get("DPoP-Nonce")
		if dpopNonce != "" {
			// Retry with nonce and access token hash
			dpopProof, err = CreateDPoPProof(session.DPoPKey, "POST", pdsURL, dpopNonce, session.AccessToken)
			if err != nil {
				return "", "", fmt.Errorf("failed to create DPoP proof with nonce: %w", err)
			}

			req, err = http.NewRequest("POST", pdsURL, bytes.NewReader(payloadBytes))
			if err != nil {
				return "", "", fmt.Errorf("failed to create retry request: %w", err)
			}

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "DPoP "+session.AccessToken)
			req.Header.Set("DPoP", dpopProof)

			resp, err = client.Do(req)
			if err != nil {
				return "", "", fmt.Errorf("PDS retry request failed: %w", err)
			}
			defer resp.Body.Close()

			body, err = io.ReadAll(resp.Body)
			if err != nil {
				return "", "", fmt.Errorf("failed to read retry response: %w", err)
			}
		}
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("PDS returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result struct {
		URI string `json:"uri"`
		CID string `json:"cid"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", fmt.Errorf("failed to parse response: %w", err)
	}

	return result.URI, result.CID, nil
}

// RefreshAccessToken refreshes an expired access token using a refresh token
// Returns new access token, refresh token, and expires_in seconds
func RefreshAccessToken(session *OAuthSession, authServerURL, clientID, clientKey string) (string, string, int, error) {
	if session == nil {
		return "", "", 0, fmt.Errorf("session cannot be nil")
	}

	if session.RefreshToken == "" {
		return "", "", 0, fmt.Errorf("session missing refresh token")
	}

	if session.DPoPKey == "" {
		return "", "", 0, fmt.Errorf("session missing DPoP key")
	}

	// Get token endpoint from auth server
	tokenEndpoint, err := GetTokenEndpoint(authServerURL)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to get token endpoint: %w", err)
	}

	// Create client assertion for authentication
	clientAssertion, err := SignClientAssertion(clientKey, clientID, authServerURL)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to create client assertion: %w", err)
	}

	// Create DPoP proof (no access token for token endpoint)
	dpopProof, err := CreateDPoPProof(session.DPoPKey, "POST", tokenEndpoint, "", "")
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to create DPoP proof: %w", err)
	}

	// Build form data
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", session.RefreshToken)
	data.Set("client_id", clientID)
	data.Set("client_assertion_type", "urn:ietf:params:oauth:client-assertion-type:jwt-bearer")
	data.Set("client_assertion", clientAssertion)

	// Create HTTP request
	req, err := http.NewRequest("POST", tokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("DPoP", dpopProof)

	// Execute request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", 0, fmt.Errorf("token refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for DPoP nonce requirement
	if resp.StatusCode == http.StatusBadRequest {
		var errorResp map[string]interface{}
		if err := json.Unmarshal(body, &errorResp); err == nil {
			if errorCode, ok := errorResp["error"].(string); ok && errorCode == "use_dpop_nonce" {
				dpopNonce := resp.Header.Get("DPoP-Nonce")
				if dpopNonce != "" {
					// Retry with nonce (no access token for token endpoint)
					dpopProof, err = CreateDPoPProof(session.DPoPKey, "POST", tokenEndpoint, dpopNonce, "")
					if err != nil {
						return "", "", 0, fmt.Errorf("failed to create DPoP proof with nonce: %w", err)
					}

					req, err = http.NewRequest("POST", tokenEndpoint, strings.NewReader(data.Encode()))
					if err != nil {
						return "", "", 0, fmt.Errorf("failed to create retry request: %w", err)
					}

					req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
					req.Header.Set("DPoP", dpopProof)

					resp, err = client.Do(req)
					if err != nil {
						return "", "", 0, fmt.Errorf("token refresh retry request failed: %w", err)
					}
					defer resp.Body.Close()

					body, err = io.ReadAll(resp.Body)
					if err != nil {
						return "", "", 0, fmt.Errorf("failed to read retry response: %w", err)
					}
				}
			}
		}
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return "", "", 0, fmt.Errorf("token refresh failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse token response
	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", "", 0, fmt.Errorf("failed to parse response: %w", err)
	}

	return tokenResp.AccessToken, tokenResp.RefreshToken, tokenResp.ExpiresIn, nil
}

// ListRecords fetches records from a collection (public endpoint, no auth required)
func ListRecords(pdsURL, did, collection string, cursor string, limit int) (*ListRecordsResponse, error) {
	if pdsURL == "" {
		return nil, fmt.Errorf("PDS URL cannot be empty")
	}

	if did == "" {
		return nil, fmt.Errorf("DID cannot be empty")
	}

	if collection == "" {
		return nil, fmt.Errorf("collection cannot be empty")
	}

	// Build URL with query parameters
	baseURL := strings.TrimSuffix(pdsURL, "/") + "/xrpc/com.atproto.repo.listRecords"
	params := url.Values{}
	params.Set("repo", did)
	params.Set("collection", collection)
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}
	if cursor != "" {
		params.Set("cursor", cursor)
	}

	fullURL := baseURL + "?" + params.Encode()

	// Create HTTP request
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Execute request (no auth required for public listRecords)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("PDS request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("PDS returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result ListRecordsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract rkey from URI and format JSON for each record
	for i := range result.Records {
		parts := strings.Split(result.Records[i].URI, "/")
		if len(parts) > 0 {
			result.Records[i].RKey = parts[len(parts)-1]
		}
		// Format Value as indented JSON for display
		if jsonBytes, err := json.MarshalIndent(result.Records[i].Value, "", "  "); err == nil {
			result.Records[i].ValueJSON = string(jsonBytes)
		}
	}

	return &result, nil
}

// DeleteRecord deletes a single record from the user's PDS (requires auth)
func DeleteRecord(session *OAuthSession, collection, rkey string) error {
	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}

	if session.AccessToken == "" {
		return fmt.Errorf("session missing access token")
	}

	if session.PDSUrl == "" {
		return fmt.Errorf("session missing PDS URL")
	}

	if session.DPoPKey == "" {
		return fmt.Errorf("session missing DPoP key")
	}

	// Check if token is expired
	if session.TokenExpiresAt != nil && time.Now().After(*session.TokenExpiresAt) {
		return fmt.Errorf("access token expired")
	}

	// Build request payload
	payload := map[string]interface{}{
		"repo":       session.DID,
		"collection": collection,
		"rkey":       rkey,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Build PDS URL
	pdsURL := strings.TrimSuffix(session.PDSUrl, "/") + "/xrpc/com.atproto.repo.deleteRecord"

	// Create DPoP proof
	dpopProof, err := CreateDPoPProof(session.DPoPKey, "POST", pdsURL, "", session.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to create DPoP proof: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", pdsURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "DPoP "+session.AccessToken)
	req.Header.Set("DPoP", dpopProof)

	// Execute request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("PDS request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check for DPoP nonce requirement
	if resp.StatusCode == http.StatusUnauthorized {
		dpopNonce := resp.Header.Get("DPoP-Nonce")
		if dpopNonce != "" {
			// Retry with nonce
			dpopProof, err = CreateDPoPProof(session.DPoPKey, "POST", pdsURL, dpopNonce, session.AccessToken)
			if err != nil {
				return fmt.Errorf("failed to create DPoP proof with nonce: %w", err)
			}

			req, err = http.NewRequest("POST", pdsURL, bytes.NewReader(payloadBytes))
			if err != nil {
				return fmt.Errorf("failed to create retry request: %w", err)
			}

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "DPoP "+session.AccessToken)
			req.Header.Set("DPoP", dpopProof)

			resp, err = client.Do(req)
			if err != nil {
				return fmt.Errorf("PDS retry request failed: %w", err)
			}
			defer resp.Body.Close()

			body, err = io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("failed to read retry response: %w", err)
			}
		}
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("PDS returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// UpdateRecord updates an existing record in the user's PDS (requires auth)
func UpdateRecord(session *OAuthSession, collection, rkey string, record interface{}) (string, string, error) {
	if session == nil {
		return "", "", fmt.Errorf("session cannot be nil")
	}

	if session.AccessToken == "" {
		return "", "", fmt.Errorf("session missing access token")
	}

	if session.PDSUrl == "" {
		return "", "", fmt.Errorf("session missing PDS URL")
	}

	if session.DPoPKey == "" {
		return "", "", fmt.Errorf("session missing DPoP key")
	}

	// Check if token is expired
	if session.TokenExpiresAt != nil && time.Now().After(*session.TokenExpiresAt) {
		return "", "", fmt.Errorf("access token expired")
	}

	// Build request payload
	validateFalse := false
	payload := map[string]interface{}{
		"repo":       session.DID,
		"collection": collection,
		"rkey":       rkey,
		"record":     record,
		"validate":   &validateFalse,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal record: %w", err)
	}

	// Build PDS URL
	pdsURL := strings.TrimSuffix(session.PDSUrl, "/") + "/xrpc/com.atproto.repo.putRecord"

	// Create DPoP proof
	dpopProof, err := CreateDPoPProof(session.DPoPKey, "POST", pdsURL, "", session.AccessToken)
	if err != nil {
		return "", "", fmt.Errorf("failed to create DPoP proof: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", pdsURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "DPoP "+session.AccessToken)
	req.Header.Set("DPoP", dpopProof)

	// Execute request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("PDS request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to read response: %w", err)
	}

	// Check for DPoP nonce requirement
	if resp.StatusCode == http.StatusUnauthorized {
		dpopNonce := resp.Header.Get("DPoP-Nonce")
		if dpopNonce != "" {
			// Retry with nonce
			dpopProof, err = CreateDPoPProof(session.DPoPKey, "POST", pdsURL, dpopNonce, session.AccessToken)
			if err != nil {
				return "", "", fmt.Errorf("failed to create DPoP proof with nonce: %w", err)
			}

			req, err = http.NewRequest("POST", pdsURL, bytes.NewReader(payloadBytes))
			if err != nil {
				return "", "", fmt.Errorf("failed to create retry request: %w", err)
			}

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "DPoP "+session.AccessToken)
			req.Header.Set("DPoP", dpopProof)

			resp, err = client.Do(req)
			if err != nil {
				return "", "", fmt.Errorf("PDS retry request failed: %w", err)
			}
			defer resp.Body.Close()

			body, err = io.ReadAll(resp.Body)
			if err != nil {
				return "", "", fmt.Errorf("failed to read retry response: %w", err)
			}
		}
	}

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("PDS returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result struct {
		URI string `json:"uri"`
		CID string `json:"cid"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", fmt.Errorf("failed to parse response: %w", err)
	}

	return result.URI, result.CID, nil
}
