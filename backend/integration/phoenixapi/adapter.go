package phoenixapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/strutil"
)

type AuthKind string

const (
	AuthBearer AuthKind = "bearer"
	AuthDevice AuthKind = "device"
)

type AuthRef struct {
	Kind   AuthKind
	Label  string
	Token  string
	APIKey string
	PIN    string
}

type Envelope struct {
	Status  string          `json:"status"`
	Data    json.RawMessage `json:"data"`
	Message string          `json:"message"`
}

type APIError struct {
	Method     string
	Path       string
	StatusCode int
	Message    string
	Body       string
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return fmt.Sprintf("%s %s failed: %d - %s", e.Method, e.Path, e.StatusCode, e.Message)
	}
	if e.Body != "" {
		return fmt.Sprintf("%s %s failed: %d - %s", e.Method, e.Path, e.StatusCode, e.Body)
	}
	return fmt.Sprintf("%s %s failed: %d", e.Method, e.Path, e.StatusCode)
}

type Adapter struct {
	baseURL    string
	httpClient *http.Client
	verbose    bool
}

func New(baseURL string, verbose bool) *Adapter {
	return NewWithHTTPClient(baseURL, verbose, nil)
}

func NewWithHTTPClient(baseURL string, verbose bool, httpClient *http.Client) *Adapter {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}
	return &Adapter{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		httpClient: httpClient,
		verbose:    verbose,
	}
}

func (a *Adapter) BaseURL() string {
	return a.baseURL
}

func (a *Adapter) HTTPClient() *http.Client {
	return a.httpClient
}

func (a *Adapter) CheckHealth(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/health", nil)
	if err != nil {
		return fmt.Errorf("build health request: %w", err)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("server not reachable at %s: %w", a.baseURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server health check failed: status %d", resp.StatusCode)
	}
	return nil
}

func (a *Adapter) LoginOperator(ctx context.Context, email, password string) (AuthRef, error) {
	body := map[string]string{
		"email":    email,
		"password": password,
	}

	respBody, _, err := a.Raw(ctx, AuthRef{}, http.MethodPost, "/operator/auth/login", body, nil)
	if err != nil {
		return AuthRef{}, fmt.Errorf("operator login request failed: %w", err)
	}

	var loginResp struct {
		Status string `json:"status"`
		Data   struct {
			Status                string `json:"status"`
			AccessToken           string `json:"access_token"`
			ChallengeToken        string `json:"challenge_token"`
			MFAEnrollmentRequired bool   `json:"mfa_enrollment_required"`
		} `json:"data"`
		AccessToken           string `json:"access_token"`
		ChallengeToken        string `json:"challenge_token"`
		MFAEnrollmentRequired bool   `json:"mfa_enrollment_required"`
	}
	if err := json.Unmarshal(respBody, &loginResp); err != nil {
		return AuthRef{}, fmt.Errorf("parse operator login response: %w", err)
	}

	token := loginResp.Data.AccessToken
	if token == "" {
		token = loginResp.AccessToken
	}
	innerStatus := loginResp.Data.Status
	if innerStatus == "" {
		innerStatus = loginResp.Status
	}
	challengeToken := loginResp.Data.ChallengeToken
	if challengeToken == "" {
		challengeToken = loginResp.ChallengeToken
	}
	enrollmentRequired := loginResp.Data.MFAEnrollmentRequired ||
		loginResp.MFAEnrollmentRequired ||
		innerStatus == "mfa_enrollment_required"
	verifyRequired := challengeToken != "" || innerStatus == "mfa_required"

	switch {
	case enrollmentRequired:
		token, err = a.completeOperatorMFAEnrollment(ctx, token, email)
		if err != nil {
			return AuthRef{}, fmt.Errorf("complete operator mfa enrollment: %w", err)
		}
	case verifyRequired:
		token, err = a.completeOperatorMFAVerify(ctx, challengeToken, email)
		if err != nil {
			return AuthRef{}, fmt.Errorf("complete operator mfa verify: %w", err)
		}
	case token == "":
		return AuthRef{}, fmt.Errorf("no access token in operator login response")
	}

	return AuthRef{
		Kind:  AuthBearer,
		Label: "operator",
		Token: token,
	}, nil
}

// completeOperatorMFAVerify drives the dev-only mailpit-backed verify
// loop for an already-enrolled operator. The login itself triggers the
// challenge email, so we only need to poll for the code and POST it
// alongside the challenge_token.
func (a *Adapter) completeOperatorMFAVerify(ctx context.Context, challengeToken, recipient string) (string, error) {
	watermark := time.Now().Add(-5 * time.Second)
	code, err := FetchLatestMFACode(ctx, recipient, watermark)
	if err != nil {
		return "", err
	}
	respBody, _, err := a.Raw(ctx, AuthRef{}, http.MethodPost, "/operator/auth/mfa/verify",
		map[string]any{"challenge_token": challengeToken, "code": code}, nil)
	if err != nil {
		return "", fmt.Errorf("mfa verify: %w", err)
	}
	token, err := parseLoginToken(respBody)
	if err != nil {
		return "", fmt.Errorf("decode mfa verify: %w", err)
	}
	if token == "" {
		return "", fmt.Errorf("mfa verify returned no access token")
	}
	return token, nil
}

// completeOperatorMFAEnrollment drives the dev-only mailpit-backed
// enrollment loop introduced by branch feat/1308-2fa-email. The operator
// login returns a pending-enrollment token; the seeder cannot proceed
// until enrollment is confirmed. Triggers the email, polls mailpit for
// the 6-digit code, and exchanges it for a real access token.
func (a *Adapter) completeOperatorMFAEnrollment(ctx context.Context, enrollmentToken, recipient string) (string, error) {
	enrollAuth := AuthRef{Kind: AuthBearer, Label: "operator-enroll", Token: enrollmentToken}

	watermark := time.Now()
	if _, _, err := a.Raw(ctx, enrollAuth, http.MethodPost, "/operator/auth/mfa/enroll/start", nil, nil); err != nil {
		return "", fmt.Errorf("enroll start: %w", err)
	}

	code, err := FetchLatestMFACode(ctx, recipient, watermark)
	if err != nil {
		return "", err
	}

	respBody, _, err := a.Raw(ctx, enrollAuth, http.MethodPost, "/operator/auth/mfa/enroll/confirm", map[string]string{"code": code}, nil)
	if err != nil {
		return "", fmt.Errorf("enroll confirm: %w", err)
	}
	token, err := parseLoginToken(respBody)
	if err != nil {
		return "", fmt.Errorf("decode enroll confirm: %w", err)
	}
	if token == "" {
		return "", fmt.Errorf("enroll confirm returned no access token")
	}
	return token, nil
}

func (a *Adapter) LoginTenant(ctx context.Context, email, password, tenantSlug string) (AuthRef, error) {
	body := map[string]string{
		"email":    email,
		"password": password,
	}
	if strings.TrimSpace(tenantSlug) != "" {
		body["tenant_slug"] = tenantSlug
	}

	respBody, _, err := a.Raw(ctx, AuthRef{}, http.MethodPost, "/auth/login", body, nil)
	if err != nil {
		return AuthRef{}, fmt.Errorf("login request failed: %w", err)
	}

	token, err := parseLoginToken(respBody)
	if err != nil {
		return AuthRef{}, fmt.Errorf("parse login response: %w", err)
	}
	if token == "" {
		return AuthRef{}, fmt.Errorf("no access token in login response")
	}

	label := "tenant"
	if strings.TrimSpace(tenantSlug) != "" {
		label = tenantSlug
	}
	return AuthRef{
		Kind:  AuthBearer,
		Label: label,
		Token: token,
	}, nil
}

func (a *Adapter) LoginParent(ctx context.Context, email, password string) (AuthRef, error) {
	body := map[string]string{
		"email":    email,
		"password": password,
	}

	respBody, _, err := a.Raw(ctx, AuthRef{}, http.MethodPost, "/parent/auth/login", body, nil)
	if err != nil {
		return AuthRef{}, fmt.Errorf("parent login request failed: %w", err)
	}

	token, err := parseLoginToken(respBody)
	if err != nil {
		return AuthRef{}, fmt.Errorf("parse parent login response: %w", err)
	}
	if token == "" {
		return AuthRef{}, fmt.Errorf("no access token in parent login response")
	}

	return AuthRef{
		Kind:  AuthBearer,
		Label: "parent",
		Token: token,
	}, nil
}

// parseLoginToken pulls the access token from a login response, tolerating both
// the enveloped ({"data":{"access_token":...}}) and flat shapes. An empty
// string (without an error) means no token was present.
func parseLoginToken(respBody []byte) (string, error) {
	var loginResp struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(respBody, &loginResp); err != nil {
		return "", err
	}
	token := loginResp.Data.AccessToken
	if token == "" {
		token = loginResp.AccessToken
	}
	return token, nil
}

func DeviceAuth(apiKey, pin, label string) AuthRef {
	return AuthRef{
		Kind:   AuthDevice,
		Label:  label,
		APIKey: apiKey,
		PIN:    pin,
	}
}

func (a *Adapter) Raw(ctx context.Context, auth AuthRef, method, path string, body any, headers map[string]string) ([]byte, int, error) {
	req, err := a.buildRequest(ctx, auth, method, path, body, headers)
	if err != nil {
		return nil, 0, err
	}

	if a.verbose {
		fmt.Printf("  -> %s %s (%s)\n", method, path, authModeLabel(auth))
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}

	if a.verbose {
		statusIcon := "ok"
		if resp.StatusCode >= 400 {
			statusIcon = "FAIL"
		}
		fmt.Printf("  <- %d %s\n", resp.StatusCode, statusIcon)
	}

	if resp.StatusCode >= 400 {
		return nil, resp.StatusCode, parseHTTPError(method, path, resp.StatusCode, respBody)
	}

	return respBody, resp.StatusCode, nil
}

func (a *Adapter) JSON(ctx context.Context, auth AuthRef, method, path string, body any, out any) error {
	respBody, _, err := a.Raw(ctx, auth, method, path, body, nil)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("decode %s %s response: %w", method, path, err)
	}
	return nil
}

func (a *Adapter) Envelope(ctx context.Context, auth AuthRef, method, path string, body any, out any) error {
	respBody, _, err := a.Raw(ctx, auth, method, path, body, nil)
	if err != nil {
		return err
	}

	var envelope Envelope
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return fmt.Errorf("decode %s %s envelope: %w", method, path, err)
	}
	if envelope.Status != "" && !strings.EqualFold(envelope.Status, "success") {
		return &APIError{
			Method:     method,
			Path:       path,
			StatusCode: http.StatusOK,
			Message:    envelope.Message,
			Body:       truncateBody(string(respBody)),
		}
	}
	if out == nil || len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return fmt.Errorf("decode %s %s envelope data: %w", method, path, err)
	}
	return nil
}

func (a *Adapter) buildRequest(ctx context.Context, auth AuthRef, method, path string, body any, headers map[string]string) (*http.Request, error) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, a.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "project-phoenix-integration/0.1")
	applyAuth(req, auth)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	return req, nil
}

func applyAuth(req *http.Request, auth AuthRef) {
	switch auth.Kind {
	case AuthBearer:
		if auth.Token != "" {
			req.Header.Set("Authorization", "Bearer "+auth.Token)
		}
	case AuthDevice:
		if auth.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+auth.APIKey)
		}
		if auth.PIN != "" {
			req.Header.Set("X-Staff-PIN", auth.PIN)
		}
	}
}

func parseHTTPError(method, path string, statusCode int, body []byte) error {
	var payload struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		message := strings.TrimSpace(payload.Message)
		if message == "" {
			message = strings.TrimSpace(payload.Error)
		}
		return &APIError{
			Method:     method,
			Path:       path,
			StatusCode: statusCode,
			Message:    message,
			Body:       truncateBody(string(body)),
		}
	}
	return &APIError{
		Method:     method,
		Path:       path,
		StatusCode: statusCode,
		Body:       truncateBody(string(body)),
	}
}

func truncateBody(body string) string {
	return strutil.TruncateBytes(strings.TrimSpace(body), 200, "...")
}

func authModeLabel(auth AuthRef) string {
	switch auth.Kind {
	case AuthBearer:
		if auth.Label != "" {
			return auth.Label
		}
		return "bearer"
	case AuthDevice:
		if auth.Label != "" {
			return "device:" + auth.Label
		}
		return "device"
	default:
		return "public"
	}
}
