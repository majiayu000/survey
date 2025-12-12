package oauth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTokenConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  TokenConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: TokenConfig{
				Code:            "test-code",
				CodeVerifier:    "test-verifier",
				ClientID:        "https://example.com/client",
				RedirectURI:     "https://example.com/callback",
				TokenEndpoint:   "https://auth.example.com/token",
				ClientKey:       "test-key",
				DPoPKey:         "test-dpop-key",
				AuthServerURL:   "https://auth.example.com",
			},
			wantErr: false,
		},
		{
			name: "missing code",
			config: TokenConfig{
				CodeVerifier:  "test-verifier",
				ClientID:      "https://example.com/client",
				RedirectURI:   "https://example.com/callback",
				TokenEndpoint: "https://auth.example.com/token",
				ClientKey:     "test-key",
				DPoPKey:       "test-dpop-key",
				AuthServerURL: "https://auth.example.com",
			},
			wantErr: true,
			errMsg:  "Code is required",
		},
		{
			name: "missing code verifier",
			config: TokenConfig{
				Code:          "test-code",
				ClientID:      "https://example.com/client",
				RedirectURI:   "https://example.com/callback",
				TokenEndpoint: "https://auth.example.com/token",
				ClientKey:     "test-key",
				DPoPKey:       "test-dpop-key",
				AuthServerURL: "https://auth.example.com",
			},
			wantErr: true,
			errMsg:  "CodeVerifier is required",
		},
		{
			name: "missing token endpoint",
			config: TokenConfig{
				Code:          "test-code",
				CodeVerifier:  "test-verifier",
				ClientID:      "https://example.com/client",
				RedirectURI:   "https://example.com/callback",
				ClientKey:     "test-key",
				DPoPKey:       "test-dpop-key",
				AuthServerURL: "https://auth.example.com",
			},
			wantErr: true,
			errMsg:  "TokenEndpoint is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err.Error() != tt.errMsg {
				t.Errorf("Validate() error = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestExchangeToken_Success(t *testing.T) {
	// Create test server that returns a valid token response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		// Verify Content-Type
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/x-www-form-urlencoded" {
			t.Errorf("Expected Content-Type application/x-www-form-urlencoded, got %s", contentType)
		}

		// Verify DPoP header is present
		dpop := r.Header.Get("DPoP")
		if dpop == "" {
			t.Error("Expected DPoP header to be present")
		}

		// Parse form data
		if err := r.ParseForm(); err != nil {
			t.Fatalf("Failed to parse form: %v", err)
		}

		// Verify required form fields
		if r.FormValue("grant_type") != "authorization_code" {
			t.Errorf("Expected grant_type=authorization_code, got %s", r.FormValue("grant_type"))
		}
		if r.FormValue("code") == "" {
			t.Error("Expected code to be present")
		}
		if r.FormValue("code_verifier") == "" {
			t.Error("Expected code_verifier to be present")
		}
		if r.FormValue("redirect_uri") == "" {
			t.Error("Expected redirect_uri to be present")
		}
		if r.FormValue("client_assertion_type") != "urn:ietf:params:oauth:client-assertion-type:jwt-bearer" {
			t.Error("Expected client_assertion_type to be jwt-bearer")
		}
		if r.FormValue("client_assertion") == "" {
			t.Error("Expected client_assertion to be present")
		}

		// Return success response
		response := map[string]interface{}{
			"access_token":  "test-access-token",
			"token_type":    "DPoP",
			"expires_in":    3600,
			"refresh_token": "test-refresh-token",
			"sub":           "did:plc:test123",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Generate test keys
	clientKey := GenerateSecretJWK()
	dpopKey := GenerateSecretJWK()

	config := TokenConfig{
		Code:          "test-code",
		CodeVerifier:  "test-verifier",
		ClientID:      "https://example.com/client",
		RedirectURI:   "https://example.com/callback",
		TokenEndpoint: server.URL,
		ClientKey:     clientKey,
		DPoPKey:       dpopKey,
		AuthServerURL: server.URL,
	}

	result, err := ExchangeToken(config)
	if err != nil {
		t.Fatalf("ExchangeToken() failed: %v", err)
	}

	if result.AccessToken != "test-access-token" {
		t.Errorf("Expected access_token=test-access-token, got %s", result.AccessToken)
	}
	if result.TokenType != "DPoP" {
		t.Errorf("Expected token_type=DPoP, got %s", result.TokenType)
	}
	if result.ExpiresIn != 3600 {
		t.Errorf("Expected expires_in=3600, got %d", result.ExpiresIn)
	}
	if result.RefreshToken != "test-refresh-token" {
		t.Errorf("Expected refresh_token=test-refresh-token, got %s", result.RefreshToken)
	}
	if result.Sub != "did:plc:test123" {
		t.Errorf("Expected sub=did:plc:test123, got %s", result.Sub)
	}
}

func TestExchangeToken_WithDPoPNonce(t *testing.T) {
	// Track number of requests
	requestCount := 0
	nonce := "test-nonce-12345"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		// First request: return use_dpop_nonce error
		if requestCount == 1 {
			w.Header().Set("DPoP-Nonce", nonce)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":             "use_dpop_nonce",
				"error_description": "DPoP nonce is required",
			})
			return
		}

		// Second request: verify nonce is in DPoP proof and return success
		// Note: We can't easily verify the nonce is in the JWT without decoding,
		// but we can verify the header changed
		if requestCount == 2 {
			response := map[string]interface{}{
				"access_token":  "test-access-token",
				"token_type":    "DPoP",
				"expires_in":    3600,
				"refresh_token": "test-refresh-token",
				"sub":           "did:plc:test456",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
			return
		}

		t.Errorf("Unexpected request count: %d", requestCount)
	}))
	defer server.Close()

	clientKey := GenerateSecretJWK()
	dpopKey := GenerateSecretJWK()

	config := TokenConfig{
		Code:          "test-code",
		CodeVerifier:  "test-verifier",
		ClientID:      "https://example.com/client",
		RedirectURI:   "https://example.com/callback",
		TokenEndpoint: server.URL,
		ClientKey:     clientKey,
		DPoPKey:       dpopKey,
		AuthServerURL: server.URL,
	}

	result, err := ExchangeToken(config)
	if err != nil {
		t.Fatalf("ExchangeToken() failed: %v", err)
	}

	if requestCount != 2 {
		t.Errorf("Expected 2 requests (nonce retry), got %d", requestCount)
	}

	if result.Sub != "did:plc:test456" {
		t.Errorf("Expected sub=did:plc:test456, got %s", result.Sub)
	}
}

func TestExchangeToken_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":             "invalid_grant",
			"error_description": "Authorization code is invalid or expired",
		})
	}))
	defer server.Close()

	clientKey := GenerateSecretJWK()
	dpopKey := GenerateSecretJWK()

	config := TokenConfig{
		Code:          "invalid-code",
		CodeVerifier:  "test-verifier",
		ClientID:      "https://example.com/client",
		RedirectURI:   "https://example.com/callback",
		TokenEndpoint: server.URL,
		ClientKey:     clientKey,
		DPoPKey:       dpopKey,
		AuthServerURL: server.URL,
	}

	_, err := ExchangeToken(config)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	expectedMsg := "token exchange failed with status 400: Authorization code is invalid or expired"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error %q, got %q", expectedMsg, err.Error())
	}
}

func TestGetTokenEndpoint(t *testing.T) {
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request path
		if r.URL.Path != "/.well-known/oauth-authorization-server" {
			t.Errorf("Expected path /.well-known/oauth-authorization-server, got %s", r.URL.Path)
		}

		metadata := map[string]interface{}{
			"issuer":                 serverURL,
			"authorization_endpoint": serverURL + "/authorize",
			"token_endpoint":         serverURL + "/token",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(metadata)
	}))
	defer server.Close()
	serverURL = server.URL

	endpoint, err := GetTokenEndpoint(server.URL)
	if err != nil {
		t.Fatalf("GetTokenEndpoint() failed: %v", err)
	}

	expected := server.URL + "/token"
	if endpoint != expected {
		t.Errorf("Expected endpoint %s, got %s", expected, endpoint)
	}
}

func TestGetTokenEndpoint_MissingField(t *testing.T) {
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return metadata without token_endpoint
		metadata := map[string]interface{}{
			"issuer":                 serverURL,
			"authorization_endpoint": serverURL + "/authorize",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(metadata)
	}))
	defer server.Close()
	serverURL = server.URL

	_, err := GetTokenEndpoint(server.URL)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if err.Error() != "missing token_endpoint" {
		t.Errorf("Expected error 'missing token_endpoint', got %q", err.Error())
	}
}
