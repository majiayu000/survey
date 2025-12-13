package oauth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// PARConfig contains all parameters needed for a PAR request
type PARConfig struct {
	ClientID      string // OAuth client ID (usually client metadata URL)
	RedirectURI   string // Callback URL
	Scope         string // OAuth scopes (e.g., "atproto transition:generic")
	State         string // Random state for CSRF protection
	CodeVerifier  string // PKCE code verifier
	DPoPKey       string // DPoP private key (JWK)
	ClientKey     string // Client signing key (JWK)
	PAREndpoint   string // PAR endpoint URL
	AuthServerURL string // Authorization server URL
}

// Validate checks that all required fields are present
func (c *PARConfig) Validate() error {
	if c.ClientID == "" {
		return fmt.Errorf("ClientID is required")
	}
	if c.RedirectURI == "" {
		return fmt.Errorf("RedirectURI is required")
	}
	if c.State == "" {
		return fmt.Errorf("State is required")
	}
	if c.CodeVerifier == "" {
		return fmt.Errorf("CodeVerifier is required")
	}
	if c.DPoPKey == "" {
		return fmt.Errorf("DPoPKey is required")
	}
	if c.ClientKey == "" {
		return fmt.Errorf("ClientKey is required")
	}
	if c.PAREndpoint == "" {
		return fmt.Errorf("PAREndpoint is required")
	}
	if c.AuthServerURL == "" {
		return fmt.Errorf("AuthServerURL is required")
	}
	return nil
}

// PARResponse is the response from a PAR request
type PARResponse struct {
	RequestURI string `json:"request_uri"`
	ExpiresIn  int    `json:"expires_in"`
}

// ExecutePAR executes a Pushed Authorization Request
// Returns the request_uri to use in the authorization redirect
func ExecutePAR(config PARConfig) (string, error) {
	if err := config.Validate(); err != nil {
		return "", fmt.Errorf("invalid config: %v", err)
	}

	// Generate code challenge from verifier
	codeChallenge := GenerateCodeChallenge(config.CodeVerifier)

	// Sign client assertion JWT
	clientAssertion, err := SignClientAssertion(config.ClientKey, config.ClientID, config.AuthServerURL)
	if err != nil {
		return "", fmt.Errorf("failed to create client assertion: %v", err)
	}

	// Create DPoP proof (no access token for PAR endpoint)
	dpopProof, err := CreateDPoPProof(config.DPoPKey, "POST", config.PAREndpoint, "", "")
	if err != nil {
		return "", fmt.Errorf("failed to create DPoP proof: %v", err)
	}

	// Make PAR request
	requestURI, dpopNonce, err := makePARRequest(config, codeChallenge, clientAssertion, dpopProof)
	if err != nil {
		return "", err
	}

	// If server requires DPoP nonce, retry with nonce
	if dpopNonce != "" {
		dpopProof, err = CreateDPoPProof(config.DPoPKey, "POST", config.PAREndpoint, dpopNonce, "")
		if err != nil {
			return "", fmt.Errorf("failed to create DPoP proof with nonce: %v", err)
		}

		requestURI, _, err = makePARRequest(config, codeChallenge, clientAssertion, dpopProof)
		if err != nil {
			return "", err
		}
	}

	return requestURI, nil
}

// makePARRequest performs the actual HTTP POST to the PAR endpoint
func makePARRequest(config PARConfig, codeChallenge, clientAssertion, dpopProof string) (requestURI string, dpopNonce string, err error) {
	// Build form data
	data := url.Values{}
	data.Set("client_id", config.ClientID)
	data.Set("response_type", "code")
	data.Set("code_challenge", codeChallenge)
	data.Set("code_challenge_method", "S256")
	data.Set("redirect_uri", config.RedirectURI)
	data.Set("scope", config.Scope)
	data.Set("state", config.State)
	data.Set("client_assertion_type", "urn:ietf:params:oauth:client-assertion-type:jwt-bearer")
	data.Set("client_assertion", clientAssertion)

	// Create HTTP request
	req, err := http.NewRequest("POST", config.PAREndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("DPoP", dpopProof)

	// Execute request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to make PAR request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to read response: %v", err)
	}

	// Parse response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", fmt.Errorf("failed to parse response: %v", err)
	}

	// Check for DPoP nonce requirement
	if resp.StatusCode == http.StatusBadRequest {
		if errorCode, ok := result["error"].(string); ok && errorCode == "use_dpop_nonce" {
			nonce := resp.Header.Get("DPoP-Nonce")
			return "", nonce, nil
		}
	}

	// Check for errors
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		errorMsg := string(body)
		if errorDesc, ok := result["error_description"].(string); ok {
			errorMsg = errorDesc
		}
		return "", "", fmt.Errorf("PAR request failed with status %d: %s", resp.StatusCode, errorMsg)
	}

	// Extract request_uri
	requestURIValue, ok := result["request_uri"].(string)
	if !ok {
		return "", "", fmt.Errorf("missing or invalid request_uri in response")
	}

	return requestURIValue, "", nil
}
