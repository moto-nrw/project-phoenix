package api

import (
	"fmt"

	"context"
	"encoding/json"
	"net/http"

	"github.com/moto-nrw/project-phoenix/integration/phoenixapi"
)

// Client handles HTTP communication with the backend API
type Client struct {
	baseURL    string
	adapter    *phoenixapi.Adapter
	httpClient *http.Client
	auth       phoenixapi.AuthRef
	token      string
	verbose    bool
}

// NewClient creates a new API client
func NewClient(baseURL string, verbose bool) *Client {
	adapter := phoenixapi.New(baseURL, verbose)
	return &Client{
		baseURL:    baseURL,
		adapter:    adapter,
		httpClient: adapter.HTTPClient(),
		verbose:    verbose,
	}
}

// NewClientWithAdapter creates a client that reuses a shared adapter.
func NewClientWithAdapter(adapter *phoenixapi.Adapter, verbose bool) *Client {
	return &Client{
		baseURL:    adapter.BaseURL(),
		adapter:    adapter,
		httpClient: adapter.HTTPClient(),
		verbose:    verbose,
	}
}

func (c *Client) BindAuth(auth phoenixapi.AuthRef) {
	c.auth = auth
	c.token = auth.Token
}

// CheckHealth verifies the server is reachable
func (c *Client) CheckHealth() error {
	return c.adapter.CheckHealth(context.Background())
}

// Login authenticates with the tenant API and stores the JWT token.
func (c *Client) Login(email, password string, tenantSlug ...string) error {
	slug := ""
	if len(tenantSlug) > 0 {
		slug = tenantSlug[0]
	}
	auth, err := c.adapter.LoginTenant(context.Background(), email, password, slug)
	if err != nil {
		return err
	}
	c.BindAuth(auth)
	return nil
}

// LoginOperator authenticates against the operator API and stores the JWT token.
func (c *Client) LoginOperator(email, password string) error {
	auth, err := c.adapter.LoginOperator(context.Background(), email, password)
	if err != nil {
		return err
	}
	c.BindAuth(auth)
	return nil
}

// Post makes an authenticated POST request
func (c *Client) Post(path string, body any) ([]byte, error) {
	return c.doRequestWithHeaders("POST", path, body, true, nil)
}

// PostPublic makes an unauthenticated POST request.
func (c *Client) PostPublic(path string, body any) ([]byte, error) {
	return c.doRequestWithHeaders("POST", path, body, false, nil)
}

// PostWithHeaders makes an authenticated POST request with extra headers.
func (c *Client) PostWithHeaders(path string, body any, headers map[string]string) ([]byte, error) {
	return c.doRequestWithHeaders("POST", path, body, true, headers)
}

// Get makes an authenticated GET request
func (c *Client) Get(path string) ([]byte, error) {
	return c.doRequestWithHeaders("GET", path, nil, true, nil)
}

// Put makes an authenticated PUT request
func (c *Client) Put(path string, body any) ([]byte, error) {
	return c.doRequestWithHeaders("PUT", path, body, true, nil)
}

func (c *Client) doRequestWithHeaders(method, path string, body any, auth bool, headers map[string]string) ([]byte, error) {
	authRef := phoenixapi.AuthRef{}
	if auth {
		authRef = c.auth
		if authRef.Token == "" && c.token != "" {
			authRef = phoenixapi.AuthRef{
				Kind:  phoenixapi.AuthBearer,
				Label: "seed",
				Token: c.token,
			}
		}
	}
	respBody, statusCode, err := c.adapter.Raw(context.Background(), authRef, method, path, body, headers)
	if err != nil {
		return nil, err
	}

	if c.verbose {
		c.logRequest(method, path, body)
		c.logResponse(statusCode, respBody)
	}
	return respBody, nil
}

// logRequest logs the HTTP request details (called by Client methods)
func (c *Client) logRequest(method, path string, body any) {
	logAPIRequest(method, path, body, "")
}

// logResponse logs the HTTP response details (called by Client methods)
func (c *Client) logResponse(statusCode int, body []byte) {
	logAPIResponse(statusCode, body)
}

// ============================================================================
// Package-level logging functions (shared by Client and seeders)
// ============================================================================

// logAPIRequest logs an HTTP request with optional auth context
func logAPIRequest(method, path string, body any, authContext string) {
	if authContext != "" {
		fmt.Printf("  ┌─ %s %s (%s)\n", method, path, authContext)
	} else {
		fmt.Printf("  ┌─ %s %s\n", method, path)
	}
	if body != nil {
		// Pretty print the request body
		prettyBody, err := json.MarshalIndent(body, "  │  ", "  ")
		if err == nil {
			fmt.Printf("  │  Request: %s\n", string(prettyBody))
		}
	}
}

// logAPIResponse logs an HTTP response with status and key fields
func logAPIResponse(statusCode int, body []byte) {
	statusIcon := "✓"
	if statusCode >= 400 {
		statusIcon = "✗"
	}

	// Try to extract just the important parts (id, status)
	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err == nil {
		// Extract key fields for concise logging
		summary := extractResponseSummary(resp)
		fmt.Printf("  └─ %s %d: %s\n", statusIcon, statusCode, summary)
	} else {
		// Fallback: show truncated raw response
		responseStr := string(body)
		if len(responseStr) > 100 {
			responseStr = responseStr[:100] + "..."
		}
		fmt.Printf("  └─ %s %d: %s\n", statusIcon, statusCode, responseStr)
	}
}

// extractResponseSummary extracts key fields from response for logging
func extractResponseSummary(resp map[string]any) string {
	parts := []string{}

	// Status
	if status, ok := resp["status"].(string); ok {
		parts = append(parts, fmt.Sprintf("status=%s", status))
	}

	// Extract fields from nested data object
	if data, ok := resp["data"].(map[string]any); ok {
		if id, ok := data["id"]; ok {
			parts = append(parts, fmt.Sprintf("id=%v", id))
		}
		if name, ok := data["name"]; ok {
			parts = append(parts, fmt.Sprintf("name=%v", name))
		}
		if email, ok := data["email"]; ok {
			parts = append(parts, fmt.Sprintf("email=%v", email))
		}
		if tagID, ok := data["tag_id"]; ok {
			parts = append(parts, fmt.Sprintf("tag_id=%v", tagID))
		}
		if studentID, ok := data["student_id"]; ok {
			parts = append(parts, fmt.Sprintf("student_id=%v", studentID))
		}
		if activeGroupID, ok := data["active_group_id"]; ok {
			parts = append(parts, fmt.Sprintf("active_group_id=%v", activeGroupID))
		}
		if activityName, ok := data["activity_name"]; ok {
			parts = append(parts, fmt.Sprintf("activity=%v", activityName))
		}
	}

	// Handle array responses (e.g., list of roles)
	if data, ok := resp["data"].([]any); ok {
		parts = append(parts, fmt.Sprintf("count=%d", len(data)))
	}

	// Handle message field (for enrollment responses)
	if msg, ok := resp["message"].(string); ok && len(parts) <= 1 {
		// Only show message if we don't have more specific fields
		if len(msg) > 40 {
			msg = msg[:40] + "..."
		}
		parts = append(parts, fmt.Sprintf("msg=%s", msg))
	}

	if len(parts) == 0 {
		return "ok"
	}
	return fmt.Sprintf("{%s}", joinStrings(parts, ", "))
}

// joinStrings joins strings with a separator
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
