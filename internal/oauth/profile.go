package oauth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Profile represents a Bluesky user profile
type Profile struct {
	DID         string
	Handle      string
	DisplayName string
	Avatar      string
}

// cachedProfile wraps a profile with expiry time
type cachedProfile struct {
	profile   *Profile
	expiresAt time.Time
}

// profileCacheStore holds cached profiles
type profileCacheStore struct {
	mu       sync.RWMutex
	profiles map[string]*cachedProfile
}

var (
	// profileCache stores profiles in memory
	profileCache = &profileCacheStore{
		profiles: make(map[string]*cachedProfile),
	}

	// defaultBlueskyAPIURL is the default Bluesky API endpoint
	defaultBlueskyAPIURL = "https://public.api.bsky.app"

	// profileCacheDuration is how long to cache profiles
	profileCacheDuration = 5 * time.Minute
)

// GetProfile fetches a Bluesky profile by DID, using cache when available
func GetProfile(did string) (*Profile, error) {
	// Check cache first
	profileCache.mu.RLock()
	cached, ok := profileCache.profiles[did]
	profileCache.mu.RUnlock()

	if ok && cached.expiresAt.After(time.Now()) {
		return cached.profile, nil
	}

	// Fetch from API
	profile, err := fetchProfileFromAPI(did, defaultBlueskyAPIURL)
	if err != nil {
		return nil, err
	}

	// Store in cache
	profileCache.mu.Lock()
	profileCache.profiles[did] = &cachedProfile{
		profile:   profile,
		expiresAt: time.Now().Add(profileCacheDuration),
	}
	profileCache.mu.Unlock()

	return profile, nil
}

// fetchProfileFromAPI fetches a profile from the Bluesky API
// The baseURL parameter allows testing with a mock server
func fetchProfileFromAPI(did, baseURL string) (*Profile, error) {
	// Build request URL
	endpoint := fmt.Sprintf("%s/xrpc/app.bsky.actor.getProfile", baseURL)
	params := url.Values{}
	params.Add("actor", did)

	reqURL := fmt.Sprintf("%s?%s", endpoint, params.Encode())

	// Make HTTP request
	resp, err := http.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch profile: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse response
	var data struct {
		DID         string `json:"did"`
		Handle      string `json:"handle"`
		DisplayName string `json:"displayName,omitempty"`
		Avatar      string `json:"avatar,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode profile: %w", err)
	}

	return &Profile{
		DID:         data.DID,
		Handle:      data.Handle,
		DisplayName: data.DisplayName,
		Avatar:      data.Avatar,
	}, nil
}
