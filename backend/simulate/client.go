package simulate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is a multi-auth HTTP client that supports both JWT and device API key authentication.
type Client struct {
	baseURL    string
	httpClient *http.Client
	jwtToken   string
	verbose    bool
}

// NewClient creates a new simulation client.
func NewClient(baseURL string, verbose bool) *Client {
	return &Client{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
		verbose:    verbose,
	}
}

// Login authenticates with the backend and stores the JWT token.
func (c *Client) Login(email, password string) error {
	body := map[string]string{
		"email":    email,
		"password": password,
	}

	respBody, err := c.doRequest("POST", "/auth/login", body, "", "")
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	var loginResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(respBody, &loginResp); err != nil {
		return fmt.Errorf("parse login response: %w", err)
	}
	if loginResp.AccessToken == "" {
		return fmt.Errorf("no access token in login response")
	}

	c.jwtToken = loginResp.AccessToken
	return nil
}

// CheckHealth verifies the server is reachable.
func (c *Client) CheckHealth() error {
	resp, err := c.httpClient.Get(c.baseURL + "/health")
	if err != nil {
		return fmt.Errorf("server not reachable at %s: %w", c.baseURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed: status %d", resp.StatusCode)
	}
	return nil
}

// AdminGet makes a JWT-authenticated GET request.
func (c *Client) AdminGet(path string) ([]byte, error) {
	return c.doJWTRequest("GET", path, nil)
}

// AdminPost makes a JWT-authenticated POST request.
func (c *Client) AdminPost(path string, body any) ([]byte, error) {
	return c.doJWTRequest("POST", path, body)
}

// AdminPut makes a JWT-authenticated PUT request.
func (c *Client) AdminPut(path string, body any) ([]byte, error) {
	return c.doJWTRequest("PUT", path, body)
}

// DevicePost makes a device-authenticated POST request (API key + PIN).
func (c *Client) DevicePost(path string, body any, apiKey, pin string) ([]byte, error) {
	return c.doRequest("POST", path, body, apiKey, pin)
}

func (c *Client) doJWTRequest(method, path string, body any) ([]byte, error) {
	if c.jwtToken == "" {
		return nil, fmt.Errorf("not logged in (no JWT token)")
	}
	return c.doRequest(method, path, body, "", "")
}

func (c *Client) doRequest(method, path string, body any, deviceAPIKey, devicePIN string) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "project-phoenix-simulate/0.1")

	// Auth: device key+PIN takes precedence, otherwise use JWT
	if deviceAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+deviceAPIKey)
		req.Header.Set("X-Staff-PIN", devicePIN)
	} else if c.jwtToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.jwtToken)
	}

	if c.verbose {
		authCtx := "jwt"
		if deviceAPIKey != "" {
			authCtx = "device"
		}
		fmt.Printf("  -> %s %s (%s)\n", method, path, authCtx)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if c.verbose {
		statusIcon := "ok"
		if resp.StatusCode >= 400 {
			statusIcon = "FAIL"
		}
		fmt.Printf("  <- %d %s\n", resp.StatusCode, statusIcon)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%s %s: status %d: %s", method, path, resp.StatusCode, truncate(string(respBody), 200))
	}

	return respBody, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
