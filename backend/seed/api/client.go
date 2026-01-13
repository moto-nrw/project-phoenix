package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client handles HTTP communication with the backend API
type Client struct {
	baseURL    string
	httpClient *http.Client
	token      string
	verbose    bool
}

// NewClient creates a new API client
func NewClient(baseURL string, verbose bool) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		verbose: verbose,
	}
}

// Login authenticates with the API and stores the JWT token
func (c *Client) Login(email, password string) error {
	body := map[string]string{
		"email":    email,
		"password": password,
	}

	resp, err := c.doRequest("POST", "/auth/login", body, false)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}

	// Extract token from response - login returns token at root level
	var loginResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}

	if err := json.Unmarshal(resp, &loginResp); err != nil {
		return fmt.Errorf("failed to parse login response: %w", err)
	}

	if loginResp.AccessToken == "" {
		return fmt.Errorf("no access token in response")
	}

	c.token = loginResp.AccessToken
	return nil
}

// CheckHealth verifies the server is reachable
func (c *Client) CheckHealth() error {
	resp, err := c.httpClient.Get(c.baseURL + "/health")
	if err != nil {
		return fmt.Errorf("server not reachable at %s: %w", c.baseURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server health check failed: status %d", resp.StatusCode)
	}
	return nil
}

// Post makes an authenticated POST request
func (c *Client) Post(path string, body any) ([]byte, error) {
	return c.doRequest("POST", path, body, true)
}

// Get makes an authenticated GET request
func (c *Client) Get(path string) ([]byte, error) {
	return c.doRequest("GET", path, nil, true)
}

// Put makes an authenticated PUT request
func (c *Client) Put(path string, body any) ([]byte, error) {
	return c.doRequest("PUT", path, body, true)
}

// doRequest is the central method for all HTTP requests with verbose logging
func (c *Client) doRequest(method, path string, body any, auth bool) ([]byte, error) {
	var jsonBody []byte
	var err error

	if body != nil {
		jsonBody, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}

	var req *http.Request
	if jsonBody != nil {
		req, err = http.NewRequest(method, c.baseURL+path, bytes.NewBuffer(jsonBody))
	} else {
		req, err = http.NewRequest(method, c.baseURL+path, nil)
	}
	if err != nil {
		return nil, err
	}

	if jsonBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if auth && c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	// Log request in verbose mode
	if c.verbose {
		c.logRequest(method, path, body)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Log response in verbose mode
	if c.verbose {
		c.logResponse(resp.StatusCode, respBody)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%s %s failed: %d - %s", method, path, resp.StatusCode, string(respBody))
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
// Package-level logging functions (shared by Client and RuntimeSeeder)
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
