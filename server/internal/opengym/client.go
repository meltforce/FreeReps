package opengym

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// maxBodyBytes bounds a response read into memory. openGym accepts a state of
// up to 5 MiB on writes; the bound leaves room above that for media references
// and growth without letting a misbehaving server exhaust memory.
const maxBodyBytes = 32 << 20

// CodePairInvalid is the error code openGym answers an unknown, used or expired
// pairing code with.
const CodePairInvalid = "pair-invalid"

// APIError is a non-200 response from an openGym instance.
type APIError struct {
	Path       string
	StatusCode int
	Code       string // the `code` field of the error body, when present
	Message    string // the `error` field of the error body, or the raw body
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("openGym %s returned %d (%s): %s", e.Path, e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("openGym %s returned %d: %s", e.Path, e.StatusCode, e.Message)
}

// IsUnauthorized reports whether the token was rejected: expired after
// SESSION_DAYS without renewal, or revoked by "Sign out everywhere".
func IsUnauthorized(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusUnauthorized
}

// Client talks to one openGym instance's HTTP API. Requests leave through the
// host's network stack, not the tsnet node; the reachability that relies on is
// recorded in server/specs/opengym-api.md, *Network reach*.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewClient creates a client for the instance at baseURL.
func NewClient(baseURL string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    strings.TrimRight(baseURL, "/"),
	}
}

// NormalizeBaseURL checks that s is an https URL without path, query or
// credentials and returns it as scheme://host. https is required because the
// token travels in every request; openGym's own MCP client refuses http too.
func NormalizeBaseURL(s string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(s))
	if err != nil {
		return "", fmt.Errorf("parsing URL: %w", err)
	}
	if u.Scheme != "https" || u.Host == "" {
		return "", errors.New("the URL must start with https:// and name a host")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.Trim(u.Path, "/") != "" {
		return "", errors.New("the URL must be the instance's address only, without path, query or credentials")
	}
	return "https://" + u.Host, nil
}

// Me is the body of GET /api/me.
type Me struct {
	User struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"user"`
	// Token is set once the presented token is past half its lifetime. It
	// replaces the old one, which still works until it expires.
	Token string `json:"token"`
}

// Rev is the body of GET /api/data/rev.
type Rev struct {
	Rev int64  `json:"rev"`
	WID string `json:"wid"`
}

// Redeem turns a pairing code into a session token. The code is minted in a
// browser signed in to the profile and is valid once, for five minutes.
func (c *Client) Redeem(ctx context.Context, code string) (string, error) {
	body, err := json.Marshal(map[string]string{"code": code})
	if err != nil {
		return "", err
	}
	raw, err := c.do(ctx, http.MethodPost, "/api/pair/redeem", "", body)
	if err != nil {
		return "", err
	}
	var resp struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("decoding pairing response: %w", err)
	}
	if resp.Token == "" {
		return "", errors.New("pairing response carried no token")
	}
	return resp.Token, nil
}

// Me validates a token and returns a renewed one when openGym issues it.
func (c *Client) Me(ctx context.Context, token string) (*Me, error) {
	raw, err := c.do(ctx, http.MethodGet, "/api/me", token, nil)
	if err != nil {
		return nil, err
	}
	var me Me
	if err := json.Unmarshal(raw, &me); err != nil {
		return nil, fmt.Errorf("decoding /api/me: %w", err)
	}
	return &me, nil
}

// Rev returns the revision of the profile's state.
func (c *Client) Rev(ctx context.Context, token string) (*Rev, error) {
	raw, err := c.do(ctx, http.MethodGet, "/api/data/rev", token, nil)
	if err != nil {
		return nil, err
	}
	var rev Rev
	if err := json.Unmarshal(raw, &rev); err != nil {
		return nil, fmt.Errorf("decoding /api/data/rev: %w", err)
	}
	return &rev, nil
}

// Data returns the profile's state.
func (c *Client) Data(ctx context.Context, token string) (*DataResponse, error) {
	raw, err := c.do(ctx, http.MethodGet, "/api/data", token, nil)
	if err != nil {
		return nil, err
	}
	var resp DataResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("decoding /api/data: %w", err)
	}
	return &resp, nil
}

func (c *Client) do(ctx context.Context, method, path, token string, body []byte) ([]byte, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "FreeReps")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading response from %s: %w", path, err)
	}
	if len(raw) > maxBodyBytes {
		return nil, fmt.Errorf("response from %s exceeds %d bytes", path, maxBodyBytes)
	}
	if resp.StatusCode != http.StatusOK {
		apiErr := &APIError{Path: path, StatusCode: resp.StatusCode, Message: string(raw)}
		var e struct {
			Error string `json:"error"`
			Code  string `json:"code"`
		}
		if json.Unmarshal(raw, &e) == nil && e.Error != "" {
			apiErr.Message, apiErr.Code = e.Error, e.Code
		}
		return nil, apiErr
	}
	return raw, nil
}
