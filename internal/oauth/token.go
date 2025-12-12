package oauth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// TokenConfig contains all parameters needed for token exchange
type TokenConfig struct {
	Code          string // Authorization code from callback
	CodeVerifier  string // PKCE code verifier
	ClientID      string // OAuth client ID
	RedirectURI   string // Callback URL (must match PAR request)
	TokenEndpoint string // Token endpoint URL
	ClientKey     string // Client signing key (JWK)
	DPoPKey       string // DPoP private key (JWK)
	AuthServerURL string // Authorization server URL
}

// Validate checks that all required fields are present
func (c *TokenConfig) Validate() error {
	if c.Code == "" {
		return fmt.Errorf("Code is required")
	}
	if c.CodeVerifier == "" {
		return fmt.Errorf("CodeVerifier is required")
	}
	if c.ClientID == "" {
		return fmt.Errorf("ClientID is required")
	}
	if c.RedirectURI == "" {
		return fmt.Errorf("RedirectURI is required")
	}
	if c.TokenEndpoint == "" {
		return fmt.Errorf("TokenEndpoint is required")
	}
	if c.ClientKey == "" {
		return fmt.Errorf("ClientKey is required")
	}
	if c.DPoPKey == "" {
		return fmt.Errorf("DPoPKey is required")
	}
	if c.AuthServerURL == "" {
		return fmt.Errorf("AuthServerURL is required")
	}
	return nil
}

// TokenResponse represents the OAuth token response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
	Sub          string `json:"sub"` // User's DID
}

// ExchangeToken exchanges an authorization code for access and refresh tokens
// This implements the OAuth 2.0 token exchange with DPoP (ATProto requirements)
func ExchangeToken(config TokenConfig) (*TokenResponse, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %v", err)
	}

	// Sign client assertion JWT for authentication
	clientAssertion, err := SignClientAssertion(config.ClientKey, config.ClientID, config.AuthServerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create client assertion: %v", err)
	}

	// Create DPoP proof
	dpopProof, err := CreateDPoPProof(config.DPoPKey, "POST", config.TokenEndpoint, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create DPoP proof: %v", err)
	}

	// Make token request
	tokenResp, dpopNonce, err := makeTokenRequest(config, clientAssertion, dpopProof)
	if err != nil {
		return nil, err
	}

	// If server requires DPoP nonce, retry with nonce
	if dpopNonce != "" {
		dpopProof, err = CreateDPoPProof(config.DPoPKey, "POST", config.TokenEndpoint, dpopNonce)
		if err != nil {
			return nil, fmt.Errorf("failed to create DPoP proof with nonce: %v", err)
		}

		tokenResp, _, err = makeTokenRequest(config, clientAssertion, dpopProof)
		if err != nil {
			return nil, err
		}
	}

	return tokenResp, nil
}

// makeTokenRequest performs the actual HTTP POST to the token endpoint
func makeTokenRequest(config TokenConfig, clientAssertion, dpopProof string) (*TokenResponse, string, error) {
	// Build form data
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", config.Code)
	data.Set("code_verifier", config.CodeVerifier)
	data.Set("redirect_uri", config.RedirectURI)
	data.Set("client_id", config.ClientID)
	data.Set("client_assertion_type", "urn:ietf:params:oauth:client-assertion-type:jwt-bearer")
	data.Set("client_assertion", clientAssertion)

	// Create HTTP request
	req, err := http.NewRequest("POST", config.TokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("DPoP", dpopProof)

	// Execute request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to make token request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read response: %v", err)
	}

	// Parse response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, "", fmt.Errorf("failed to parse response: %v", err)
	}

	// Check for DPoP nonce requirement
	if resp.StatusCode == http.StatusBadRequest {
		if errorCode, ok := result["error"].(string); ok && errorCode == "use_dpop_nonce" {
			nonce := resp.Header.Get("DPoP-Nonce")
			return nil, nonce, nil
		}
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		errorMsg := string(body)
		if errorDesc, ok := result["error_description"].(string); ok {
			errorMsg = errorDesc
		}
		return nil, "", fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, errorMsg)
	}

	// Parse token response
	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, "", fmt.Errorf("failed to parse token response: %v", err)
	}

	return &tokenResp, "", nil
}

// GetTokenEndpoint fetches the token endpoint from the auth server metadata
func GetTokenEndpoint(authServer string) (string, error) {
	resp, err := http.Get(authServer + "/.well-known/oauth-authorization-server")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var metadata map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return "", err
	}

	endpoint, ok := metadata["token_endpoint"].(string)
	if !ok {
		return "", fmt.Errorf("missing token_endpoint")
	}

	return endpoint, nil
}
