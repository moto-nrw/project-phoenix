package simulate

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/moto-nrw/project-phoenix/integration/phoenixapi"
)

// Client is a multi-auth HTTP client that supports both JWT and device API key authentication.
type Client struct {
	baseURL    string
	adapter    *phoenixapi.Adapter
	httpClient *http.Client
	auth       phoenixapi.AuthRef
	jwtToken   string
	verbose    bool
}

// NewClient creates a new simulation client.
func NewClient(baseURL string, verbose bool) *Client {
	adapter := phoenixapi.New(baseURL, verbose)
	return &Client{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		adapter:    adapter,
		httpClient: adapter.HTTPClient(),
		verbose:    verbose,
	}
}

// Login authenticates with the backend and stores the JWT token.
func (c *Client) Login(email, password string) error {
	auth, err := c.adapter.LoginTenant(context.Background(), email, password, "")
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}
	c.auth = auth
	c.jwtToken = auth.Token
	return nil
}

// CheckHealth verifies the server is reachable.
func (c *Client) CheckHealth() error {
	return c.adapter.CheckHealth(context.Background())
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
	auth := c.auth
	headers := map[string]string{
		"User-Agent": "project-phoenix-simulate/0.1",
	}
	if deviceAPIKey != "" {
		auth = phoenixapi.DeviceAuth(deviceAPIKey, devicePIN, deviceAPIKey)
	} else if auth.Token == "" && c.jwtToken != "" {
		auth = phoenixapi.AuthRef{
			Kind:  phoenixapi.AuthBearer,
			Label: "tenant",
			Token: c.jwtToken,
		}
	}

	respBody, _, err := c.adapter.Raw(context.Background(), auth, method, path, body, headers)
	if err != nil {
		return nil, err
	}
	return respBody, nil
}
