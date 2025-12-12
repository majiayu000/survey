package oauth

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
)

// HandleToDID resolves a Bluesky handle to a DID
// It tries multiple resolution methods in order:
// 1. DNS TXT record at _atproto.<handle>
// 2. HTTP well-known at https://<handle>/.well-known/atproto-did
// 3. Bluesky API (for bsky.social handles)
func HandleToDID(handle string) (string, error) {
	if handle == "" {
		return "", fmt.Errorf("handle cannot be empty")
	}

	// Try DNS TXT record first
	if did, err := resolveHandleViaDNS(handle); err == nil {
		return did, nil
	}

	// Try HTTP well-known
	if did, err := resolveHandleViaHTTP(handle); err == nil {
		return did, nil
	}

	// Try Bluesky API as fallback (works for bsky.social handles)
	if did, err := resolveHandleViaAPI(handle); err == nil {
		return did, nil
	}

	return "", fmt.Errorf("failed to resolve handle: %s (tried DNS, HTTP, and API)", handle)
}

// resolveHandleViaDNS tries DNS TXT record resolution
func resolveHandleViaDNS(handle string) (string, error) {
	txtRecords, err := net.LookupTXT(fmt.Sprintf("_atproto.%s", handle))
	if err != nil {
		return "", err
	}

	for _, record := range txtRecords {
		if strings.HasPrefix(record, "did=") {
			did := strings.TrimPrefix(record, "did=")
			if strings.HasPrefix(did, "did:") {
				return did, nil
			}
		}
	}

	return "", fmt.Errorf("no valid DID in DNS records")
}

// resolveHandleViaHTTP tries HTTP well-known resolution
func resolveHandleViaHTTP(handle string) (string, error) {
	url := fmt.Sprintf("https://%s/.well-known/atproto-did", handle)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP status %d", resp.StatusCode)
	}

	// Read response body (should be just the DID)
	buf := make([]byte, 256)
	n, _ := resp.Body.Read(buf)
	did := strings.TrimSpace(string(buf[:n]))

	if strings.HasPrefix(did, "did:") {
		return did, nil
	}

	return "", fmt.Errorf("invalid DID format in response")
}

// resolveHandleViaAPI tries the Bluesky API for handle resolution
func resolveHandleViaAPI(handle string) (string, error) {
	url := fmt.Sprintf("https://bsky.social/xrpc/com.atproto.identity.resolveHandle?handle=%s", handle)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API status %d", resp.StatusCode)
	}

	var result struct {
		DID string `json:"did"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if strings.HasPrefix(result.DID, "did:") {
		return result.DID, nil
	}

	return "", fmt.Errorf("invalid DID from API")
}

// DIDToPDS resolves a DID to its Personal Data Server endpoint
func DIDToPDS(did string) (string, error) {
	if did == "" {
		return "", fmt.Errorf("DID cannot be empty")
	}

	if !strings.HasPrefix(did, "did:") {
		return "", fmt.Errorf("invalid DID format: %s", did)
	}

	var didDoc map[string]interface{}

	if strings.HasPrefix(did, "did:plc:") {
		// Resolve via PLC directory
		resp, err := http.Get(fmt.Sprintf("https://plc.directory/%s", did))
		if err != nil {
			return "", fmt.Errorf("failed to resolve DID: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("failed to resolve DID: status %d", resp.StatusCode)
		}

		if err := json.NewDecoder(resp.Body).Decode(&didDoc); err != nil {
			return "", fmt.Errorf("failed to parse DID document: %v", err)
		}
	} else if strings.HasPrefix(did, "did:web:") {
		// Resolve via did:web
		domain := strings.TrimPrefix(did, "did:web:")
		resp, err := http.Get(fmt.Sprintf("https://%s/.well-known/did.json", domain))
		if err != nil {
			return "", fmt.Errorf("failed to resolve DID: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("failed to resolve DID: status %d", resp.StatusCode)
		}

		if err := json.NewDecoder(resp.Body).Decode(&didDoc); err != nil {
			return "", fmt.Errorf("failed to parse DID document: %v", err)
		}
	} else {
		return "", fmt.Errorf("unsupported DID method: %s", did)
	}

	// Extract PDS endpoint from service array
	services, ok := didDoc["service"].([]interface{})
	if !ok {
		return "", fmt.Errorf("invalid or missing service in DID document")
	}

	for _, service := range services {
		svc, ok := service.(map[string]interface{})
		if !ok {
			continue
		}

		if svc["type"] == "AtprotoPersonalDataServer" {
			if endpoint, ok := svc["serviceEndpoint"].(string); ok {
				return endpoint, nil
			}
		}
	}

	return "", fmt.Errorf("no PDS endpoint found in DID document")
}

// PDSToAuthServer resolves a PDS URL to its authorization server
func PDSToAuthServer(pdsURL string) (string, error) {
	if pdsURL == "" {
		return "", fmt.Errorf("PDS URL cannot be empty")
	}

	if !strings.HasPrefix(pdsURL, "http://") && !strings.HasPrefix(pdsURL, "https://") {
		return "", fmt.Errorf("invalid PDS URL format: %s", pdsURL)
	}

	resp, err := http.Get(pdsURL + "/.well-known/oauth-protected-resource")
	if err != nil {
		return "", fmt.Errorf("failed to fetch PDS metadata: %v", err)
	}
	defer resp.Body.Close()

	var metadata map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return "", fmt.Errorf("failed to parse PDS metadata: %v", err)
	}

	authServers, ok := metadata["authorization_servers"].([]interface{})
	if !ok || len(authServers) == 0 {
		return "", fmt.Errorf("invalid or missing authorization_servers in PDS metadata")
	}

	authServer, ok := authServers[0].(string)
	if !ok {
		return "", fmt.Errorf("invalid authorization server format")
	}

	return authServer, nil
}

// AuthServerToPAREndpoint resolves an authorization server to its PAR endpoint
func AuthServerToPAREndpoint(authServer string) (string, error) {
	if authServer == "" {
		return "", fmt.Errorf("auth server URL cannot be empty")
	}

	if !strings.HasPrefix(authServer, "http://") && !strings.HasPrefix(authServer, "https://") {
		return "", fmt.Errorf("invalid auth server URL format: %s", authServer)
	}

	resp, err := http.Get(authServer + "/.well-known/oauth-authorization-server")
	if err != nil {
		return "", fmt.Errorf("failed to fetch auth server metadata: %v", err)
	}
	defer resp.Body.Close()

	var metadata map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return "", fmt.Errorf("failed to parse auth server metadata: %v", err)
	}

	parEndpoint, ok := metadata["pushed_authorization_request_endpoint"].(string)
	if !ok {
		return "", fmt.Errorf("invalid or missing PAR endpoint in auth server metadata")
	}

	return parEndpoint, nil
}
