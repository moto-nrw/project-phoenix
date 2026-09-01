package api

import (
	"fmt"

	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"strings"
)

// Client handles HTTP communication with the backend API
type Client struct {
	baseURL string
	adapter Adapter
	auth    AuthRef
	token   string
	verbose bool
}

// NewClientWithAdapter creates a client that reuses a shared adapter.
func NewClientWithAdapter(adapter Adapter, verbose bool) *Client {
	client := &Client{adapter: adapter, verbose: verbose}
	if adapter != nil {
		client.baseURL = adapter.BaseURL()
	}
	return client
}

func (c *Client) BindAuth(auth AuthRef) {
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

// LoginParent authenticates with the parents portal API and stores the JWT token.
func (c *Client) LoginParent(email, password string) error {
	auth, err := c.adapter.LoginParent(context.Background(), email, password)
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

// PostPublicWithHeaders makes an unauthenticated POST request with extra headers.
func (c *Client) PostPublicWithHeaders(path string, body any, headers map[string]string) ([]byte, error) {
	return c.doRequestWithHeaders("POST", path, body, false, headers)
}

// PostWithHeaders makes an authenticated POST request with extra headers.
func (c *Client) PostWithHeaders(path string, body any, headers map[string]string) ([]byte, error) {
	return c.doRequestWithHeaders("POST", path, body, true, headers)
}

func (c *Client) GetWithAuth(auth AuthRef, path string) ([]byte, error) {
	return c.doRequestWithExplicitAuth("GET", path, nil, auth, nil)
}

func (c *Client) PostWithAuth(auth AuthRef, path string, body any) ([]byte, error) {
	return c.doRequestWithExplicitAuth("POST", path, body, auth, nil)
}

func (c *Client) PutWithAuth(auth AuthRef, path string, body any) ([]byte, error) {
	return c.doRequestWithExplicitAuth("PUT", path, body, auth, nil)
}

func (c *Client) PatchWithAuth(auth AuthRef, path string, body any) ([]byte, error) {
	return c.doRequestWithExplicitAuth("PATCH", path, body, auth, nil)
}

func (c *Client) DeleteWithAuth(auth AuthRef, path string) ([]byte, error) {
	return c.doRequestWithExplicitAuth("DELETE", path, nil, auth, nil)
}

func (c *Client) DeleteWithAuthBody(auth AuthRef, path string, body any) ([]byte, error) {
	return c.doRequestWithExplicitAuth("DELETE", path, body, auth, nil)
}

func (c *Client) PostWithAuthAndHeaders(auth AuthRef, path string, body any, headers map[string]string) ([]byte, error) {
	return c.doRequestWithExplicitAuth("POST", path, body, auth, headers)
}

// DevicePost makes a device-authenticated POST request (API key + PIN).
func (c *Client) DevicePost(path string, body any, apiKey, pin string) ([]byte, error) {
	return c.PostWithAuth(DeviceAuth(apiKey, pin, apiKey), path, body)
}

// DeviceGet makes a device-authenticated GET request (API key + PIN).
func (c *Client) DeviceGet(path string, apiKey, pin string) ([]byte, error) {
	return c.GetWithAuth(DeviceAuth(apiKey, pin, apiKey), path)
}

// DevicePut makes a device-authenticated PUT request (API key + PIN).
func (c *Client) DevicePut(path string, body any, apiKey, pin string) ([]byte, error) {
	return c.PutWithAuth(DeviceAuth(apiKey, pin, apiKey), path, body)
}

// PostFile makes an authenticated multipart POST with a single file part,
// which is the shape every document upload in the API expects.
func (c *Client) PostFile(path, fieldName, filename string, contents []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile(fieldName, filename)
	if err != nil {
		return nil, fmt.Errorf("build multipart body: %w", err)
	}
	if _, err := part.Write(contents); err != nil {
		return nil, fmt.Errorf("write multipart body: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart body: %w", err)
	}

	respBody, statusCode, err := c.adapter.RawUpload(context.Background(), c.authRef(), "POST", path, writer.FormDataContentType(), buf.Bytes())
	if err != nil {
		return nil, err
	}
	if c.verbose {
		c.logRequest("POST", path, nil)
		c.logResponse(statusCode, respBody)
	}
	return respBody, nil
}

// Get makes an authenticated GET request
func (c *Client) Get(path string) ([]byte, error) {
	return c.doRequestWithHeaders("GET", path, nil, true, nil)
}

// Put makes an authenticated PUT request
func (c *Client) Put(path string, body any) ([]byte, error) {
	return c.doRequestWithHeaders("PUT", path, body, true, nil)
}

// Delete makes an authenticated DELETE request.
func (c *Client) Delete(path string) ([]byte, error) {
	return c.doRequestWithHeaders("DELETE", path, nil, true, nil)
}

// DeleteWithBody makes an authenticated DELETE request with an explicit
// confirmation payload for destructive APIs that require a preview token.
func (c *Client) DeleteWithBody(path string, body any) ([]byte, error) {
	return c.doRequestWithHeaders("DELETE", path, body, true, nil)
}

func (c *Client) doRequestWithHeaders(method, path string, body any, auth bool, headers map[string]string) ([]byte, error) {
	authRef := AuthRef{}
	if auth {
		authRef = c.authRef()
	}
	return c.doRequestWithExplicitAuth(method, path, body, authRef, headers)
}

// authRef is the bound authentication, falling back to a bare token when only
// that was set.
func (c *Client) authRef() AuthRef {
	if c.auth.Token == "" && c.token != "" {
		return AuthRef{
			Kind:  AuthBearer,
			Label: "seed",
			Token: c.token,
		}
	}
	return c.auth
}

func (c *Client) doRequestWithExplicitAuth(method, path string, body any, authRef AuthRef, headers map[string]string) ([]byte, error) {
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
	return fmt.Sprintf("{%s}", strings.Join(parts, ", "))
}
