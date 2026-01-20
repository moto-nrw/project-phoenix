package betterauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Client wraps HTTP client for BetterAuth service communication.
// It handles session validation and member lookup by forwarding cookies
// from incoming requests to the BetterAuth service.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new BetterAuth client.
// The base URL is read from BETTERAUTH_URL environment variable,
// defaulting to http://localhost:3001 for development.
func NewClient() *Client {
	baseURL := os.Getenv("BETTERAUTH_URL")
	if baseURL == "" {
		baseURL = "http://localhost:3001"
	}

	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// NewClientWithURL creates a new BetterAuth client with a specific base URL.
// Useful for testing or custom configurations.
func NewClientWithURL(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// GetSession validates session cookies with BetterAuth and returns session data.
// It forwards all cookies from the incoming request to BetterAuth's session endpoint.
//
// Returns:
//   - *SessionResponse: Session data including user info and active organization
//   - ErrNoSession: If no valid session exists (401 from BetterAuth)
//   - ErrNoActiveOrg: If session exists but no organization is selected
//   - Other errors: Network failures, invalid responses, etc.
func (c *Client) GetSession(ctx context.Context, r *http.Request) (*SessionResponse, error) {
	// Create request to BetterAuth session endpoint
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		c.baseURL+"/api/auth/get-session",
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("create session request: %w", err)
	}

	// Forward all cookies from original request
	// BetterAuth uses cookies for session management
	for _, cookie := range r.Cookies() {
		req.AddCookie(cookie)
	}

	// Make request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("session request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Check status - 401 means no valid session
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrNoSession
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected session status %d: %s", resp.StatusCode, body)
	}

	// Parse response
	var session SessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return nil, fmt.Errorf("decode session response: %w", err)
	}

	// Validate required fields - user must have selected an organization
	if session.Session.ActiveOrganizationID == "" {
		return nil, ErrNoActiveOrg
	}

	return &session, nil
}

// GetActiveMember retrieves the current user's member record in their active organization.
// This includes the role name which is used to resolve permissions.
//
// IMPORTANT: BetterAuth returns the role NAME (e.g., "supervisor"), NOT the
// permissions array. Permissions must be resolved from the role using the
// RolePermissions map in auth/tenant/roles.go.
//
// Returns:
//   - *MemberInfo: Member record including role
//   - ErrNoSession: If no valid session (401)
//   - ErrMemberNotFound: If user is not a member of the active org
//   - Other errors: Network failures, invalid responses, etc.
func (c *Client) GetActiveMember(ctx context.Context, r *http.Request) (*MemberInfo, error) {
	// Create request to BetterAuth member endpoint
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		c.baseURL+"/api/auth/organization/get-active-member",
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("create member request: %w", err)
	}

	// Forward cookies from original request
	for _, cookie := range r.Cookies() {
		req.AddCookie(cookie)
	}

	// Make request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("member request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Check status
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrNoSession
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrMemberNotFound
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("member lookup failed: status %d: %s", resp.StatusCode, body)
	}

	// Parse response
	var member MemberInfo
	if err := json.NewDecoder(resp.Body).Decode(&member); err != nil {
		return nil, fmt.Errorf("decode member response: %w", err)
	}

	return &member, nil
}

// BaseURL returns the configured BetterAuth service URL.
// Useful for diagnostics and logging.
func (c *Client) BaseURL() string {
	return c.baseURL
}
