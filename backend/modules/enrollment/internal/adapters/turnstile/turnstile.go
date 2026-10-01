// Package turnstile verifies enrollment captcha tokens with Cloudflare
// Turnstile's siteverify endpoint.
package turnstile

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// VerifyURL is the canonical Turnstile siteverify endpoint.
const VerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

// Client posts secret, token and remote IP to the siteverify endpoint.
type Client struct {
	httpClient *http.Client
	verifyURL  string
}

// New builds a client. A nil httpClient falls back to a 10-second-timeout
// client; an empty verifyURL falls back to VerifyURL.
func New(httpClient *http.Client, verifyURL string) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	if verifyURL == "" {
		verifyURL = VerifyURL
	}
	return &Client{httpClient: httpClient, verifyURL: verifyURL}
}

// SiteVerify asks the provider whether token is valid. It returns the
// provider's verdict and error codes; err reports a request that could not
// be made or answered.
func (c *Client) SiteVerify(ctx context.Context, secret, token, remoteIP string) (bool, []string, error) {
	form := url.Values{}
	form.Set("secret", secret)
	form.Set("response", token)
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.verifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return false, nil, fmt.Errorf("captcha verify: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, nil, fmt.Errorf("captcha verify: http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var body struct {
		Success    bool     `json:"success"`
		ErrorCodes []string `json:"error-codes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return false, nil, fmt.Errorf("captcha verify: decode response: %w", err)
	}
	return body.Success, body.ErrorCodes, nil
}
