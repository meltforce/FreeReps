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

	// Scopes required for FreeReps integration.
	ouraScopes = "daily heartrate workout spo2Daily"

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
type TokenManager struct {
	clientID     string
	clientSecret string
	db           *storage.DB
	httpClient   *http.Client
	authorizeURL string
	tokenURL     string
}

// NewTokenManager creates a new token manager.
func NewTokenManager(clientID, clientSecret string, db *storage.DB) *TokenManager {
	return &TokenManager{
		clientID:     clientID,
		clientSecret: clientSecret,
		db:           db,
		httpClient:   &http.Client{Timeout: 15 * time.Second},
		authorizeURL: defaultAuthorizeURL,
		tokenURL:     defaultTokenURL,
	}
}

// AuthorizeURL returns the Oura OAuth2 authorization URL for user consent.
func (tm *TokenManager) AuthorizeURL(redirectURI, state string) string {
	params := url.Values{
		"response_type": {"code"},
		"client_id":     {tm.clientID},
		"redirect_uri":  {redirectURI},
		"scope":         {ouraScopes},
		"state":         {state},
	}
	return tm.authorizeURL + "?" + params.Encode()
}

// ExchangeCode exchanges an OAuth2 authorization code for tokens and stores them.
func (tm *TokenManager) ExchangeCode(ctx context.Context, code, redirectURI string, userID int) error {
	tok, err := tm.postToken(ctx, url.Values{
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
	if stored == nil {
		return "", fmt.Errorf("no oura token for user %d", userID)
	}

	// If token is still valid (with buffer), return it.
	if time.Until(stored.ExpiresAt) > refreshBuffer {
		return stored.AccessToken, nil
	}

	// Refresh the token.
	tok, err := tm.postToken(ctx, url.Values{
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

// postToken performs a POST to the Oura token endpoint with client credentials.
func (tm *TokenManager) postToken(ctx context.Context, form url.Values) (*tokenResponse, error) {
	form.Set("client_id", tm.clientID)
	form.Set("client_secret", tm.clientSecret)

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
