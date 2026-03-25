package oura

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/claude/freereps/internal/storage"
)

const (
	defaultAuthorizeURL = "https://cloud.ouraring.com/oauth/authorize"
	defaultTokenURL     = "https://api.ouraring.com/oauth/token"

	// Scopes required for FreeReps integration. Includes newer scopes
	// (spo2, stress, heart_health) not yet in the v1.28 OpenAPI spec.
	ouraScopes = "daily heartrate workout spo2 spo2Daily stress heart_health session personal"

	// refreshBuffer is how far before expiry we refresh proactively.
	refreshBuffer = 5 * time.Minute
)

// tokenResponse is the JSON returned by the Oura token endpoint.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

// TokenManager handles OAuth2 token exchange and refresh for the Oura API.
// Client credentials are loaded per-user from the database.
type TokenManager struct {
	db           *storage.DB
	httpClient   *http.Client
	authorizeURL string
	tokenURL     string
}

// NewTokenManager creates a new token manager.
func NewTokenManager(db *storage.DB) *TokenManager {
	return &TokenManager{
		db:           db,
		httpClient:   &http.Client{Timeout: 15 * time.Second},
		authorizeURL: defaultAuthorizeURL,
		tokenURL:     defaultTokenURL,
	}
}

// AuthorizeURL returns the Oura OAuth2 authorization URL for user consent.
// Loads the user's client_id from the database.
func (tm *TokenManager) AuthorizeURL(ctx context.Context, userID int, redirectURI, state string) (string, error) {
	stored, err := tm.db.GetOuraToken(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("getting oura credentials: %w", err)
	}
	if stored == nil || stored.ClientID == "" {
		return "", fmt.Errorf("no oura credentials configured for user %d", userID)
	}

	params := url.Values{
		"response_type": {"code"},
		"client_id":     {stored.ClientID},
		"redirect_uri":  {redirectURI},
		"scope":         {ouraScopes},
		"state":         {state},
	}
	return tm.authorizeURL + "?" + params.Encode(), nil
}

// ExchangeCode exchanges an OAuth2 authorization code for tokens and stores them.
func (tm *TokenManager) ExchangeCode(ctx context.Context, code, redirectURI string, userID int) error {
	stored, err := tm.db.GetOuraToken(ctx, userID)
	if err != nil {
		return fmt.Errorf("getting oura credentials: %w", err)
	}
	if stored == nil || stored.ClientID == "" {
		return fmt.Errorf("no oura credentials for user %d", userID)
	}

	tok, err := tm.postToken(ctx, stored.ClientID, stored.ClientSecret, url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {redirectURI},
	})
	if err != nil {
		return fmt.Errorf("exchanging code: %w", err)
	}

	return tm.db.UpsertOuraToken(ctx, storage.OuraToken{
		UserID:       userID,
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		TokenType:    tok.TokenType,
		ExpiresAt:    time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second),
	})
}

// GetValidToken returns a valid access token, refreshing if close to expiry.
func (tm *TokenManager) GetValidToken(ctx context.Context, userID int) (string, error) {
	stored, err := tm.db.GetOuraToken(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("getting stored token: %w", err)
	}
	if stored == nil || stored.AccessToken == "" {
		return "", fmt.Errorf("no oura token for user %d", userID)
	}

	// If token is still valid (with buffer), return it.
	if time.Until(stored.ExpiresAt) > refreshBuffer {
		return stored.AccessToken, nil
	}

	// Refresh the token.
	tok, err := tm.postToken(ctx, stored.ClientID, stored.ClientSecret, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {stored.RefreshToken},
	})
	if err != nil {
		return "", fmt.Errorf("refreshing token: %w", err)
	}

	err = tm.db.UpsertOuraToken(ctx, storage.OuraToken{
		UserID:       userID,
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		TokenType:    tok.TokenType,
		ExpiresAt:    time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second),
	})
	if err != nil {
		return "", fmt.Errorf("storing refreshed token: %w", err)
	}
	return tok.AccessToken, nil
}

// Disconnect removes Oura tokens and sync state for a user.
func (tm *TokenManager) Disconnect(ctx context.Context, userID int) error {
	if err := tm.db.DeleteOuraSyncStates(ctx, userID); err != nil {
		return err
	}
	return tm.db.DeleteOuraToken(ctx, userID)
}

// postToken performs a POST to the Oura token endpoint with per-user client credentials.
func (tm *TokenManager) postToken(ctx context.Context, clientID, clientSecret string, form url.Values) (*tokenResponse, error) {
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tm.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := tm.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading token response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var tok tokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return nil, fmt.Errorf("decoding token response: %w", err)
	}
	return &tok, nil
}
