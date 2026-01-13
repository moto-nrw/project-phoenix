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

	resp, err := c.post("/auth/login", body, false)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}

	// Extract token from response
	var loginResp struct {
		Status string `json:"status"`
		Data   struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
		Message string `json:"message"`
	}

	if err := json.Unmarshal(resp, &loginResp); err != nil {
		return fmt.Errorf("failed to parse login response: %w", err)
	}

	if loginResp.Data.AccessToken == "" {
		return fmt.Errorf("no access token in response")
	}

	c.token = loginResp.Data.AccessToken
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
	return c.post(path, body, true)
}

// Get makes an authenticated GET request
func (c *Client) Get(path string) ([]byte, error) {
	req, err := http.NewRequest("GET", c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
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

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GET %s failed: %d - %s", path, resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func (c *Client) post(path string, body any, auth bool) ([]byte, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.baseURL+path, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if auth && c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	if c.verbose {
		fmt.Printf("  → POST %s\n", path)
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

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("POST %s failed: %d - %s", path, resp.StatusCode, string(respBody))
	}

	return respBody, nil
}
