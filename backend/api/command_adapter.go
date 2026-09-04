package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
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
	Code    string          `json:"code"`
}

type APIError struct {
	Method     string
	Path       string
	StatusCode int
	Code       string
	Message    string
	Body       string
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		if e.Code != "" {
			return fmt.Sprintf("%s %s failed: %d (%s) - %s", e.Method, e.Path, e.StatusCode, e.Code, e.Message)
		}
		return fmt.Sprintf("%s %s failed: %d - %s", e.Method, e.Path, e.StatusCode, e.Message)
	}
	if e.Body != "" {
		return fmt.Sprintf("%s %s failed: %d - %s", e.Method, e.Path, e.StatusCode, e.Body)
	}
	return fmt.Sprintf("%s %s failed: %d", e.Method, e.Path, e.StatusCode)
}

type Adapter struct {
	baseURL            string
	httpClient         *http.Client
	verbose            bool
	fetchLatestMFACode func(context.Context, string, time.Time) (string, error)
}

func NewCommandAdapter(baseURL string, verbose bool) *Adapter {
	return NewWithHTTPClient(baseURL, verbose, nil)
}

func NewWithHTTPClient(baseURL string, verbose bool, httpClient *http.Client) *Adapter {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}
	return &Adapter{
		baseURL:            strings.TrimSuffix(baseURL, "/"),
		httpClient:         httpClient,
		verbose:            verbose,
		fetchLatestMFACode: FetchLatestMFACode,
	}
}

func (a *Adapter) BaseURL() string {
	return a.baseURL
}

func (a *Adapter) HTTPClient() *http.Client {
	return a.httpClient
}

func (a *Adapter) CheckHealth(ctx context.Context) error {
	_, status, err := a.Raw(ctx, AuthRef{}, http.MethodGet, "/health", nil, nil)
	if err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) {
			return fmt.Errorf("server health check failed: %w", err)
		}
		return fmt.Errorf("server not reachable at %s: %w", a.baseURL, err)
	}
	if status != http.StatusOK {
		return fmt.Errorf("GET /health failed: status %d", status)
	}
	return nil
}

func (a *Adapter) LoginOperator(ctx context.Context, email, password string) (AuthRef, error) {
	body := map[string]string{
		"email":    email,
		"password": password,
	}

	respBody, status, err := a.Raw(ctx, AuthRef{}, http.MethodPost, "/operator/auth/login", body, nil)
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
		return AuthRef{}, fmt.Errorf("parse operator login response from POST /operator/auth/login with status %d: %w", status, err)
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
		return AuthRef{}, fmt.Errorf("POST /operator/auth/login response with status %d has no access token", status)
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
	code, err := a.fetchLatestMFACode(ctx, recipient, watermark)
	if err != nil {
		return "", err
	}
	respBody, status, err := a.Raw(ctx, AuthRef{}, http.MethodPost, "/operator/auth/mfa/verify",
		map[string]any{"challenge_token": challengeToken, "code": code}, nil)
	if err != nil {
		return "", fmt.Errorf("mfa verify: %w", err)
	}
	token, err := parseLoginToken(respBody)
	if err != nil {
		return "", fmt.Errorf("decode POST /operator/auth/mfa/verify response with status %d: %w", status, err)
	}
	if token == "" {
		return "", fmt.Errorf("POST /operator/auth/mfa/verify response with status %d has no access token", status)
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

	code, err := a.fetchLatestMFACode(ctx, recipient, watermark)
	if err != nil {
		return "", err
	}

	respBody, status, err := a.Raw(ctx, enrollAuth, http.MethodPost, "/operator/auth/mfa/enroll/confirm", map[string]string{"code": code}, nil)
	if err != nil {
		return "", fmt.Errorf("enroll confirm: %w", err)
	}
	token, err := parseLoginToken(respBody)
	if err != nil {
		return "", fmt.Errorf("decode POST /operator/auth/mfa/enroll/confirm response with status %d: %w", status, err)
	}
	if token == "" {
		return "", fmt.Errorf("POST /operator/auth/mfa/enroll/confirm response with status %d has no access token", status)
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

	respBody, status, err := a.Raw(ctx, AuthRef{}, http.MethodPost, "/auth/login", body, nil)
	if err != nil {
		return AuthRef{}, fmt.Errorf("login request failed: %w", err)
	}

	token, err := parseLoginToken(respBody)
	if err != nil {
		return AuthRef{}, fmt.Errorf("parse login response from POST /auth/login with status %d: %w", status, err)
	}
	if token == "" {
		return AuthRef{}, fmt.Errorf("POST /auth/login response with status %d has no access token", status)
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

	respBody, status, err := a.Raw(ctx, AuthRef{}, http.MethodPost, "/parent/auth/login", body, nil)
	if err != nil {
		return AuthRef{}, fmt.Errorf("parent login request failed: %w", err)
	}

	token, err := parseLoginToken(respBody)
	if err != nil {
		return AuthRef{}, fmt.Errorf("parse parent login response from POST /parent/auth/login with status %d: %w", status, err)
	}
	if token == "" {
		return AuthRef{}, fmt.Errorf("POST /parent/auth/login response with status %d has no access token", status)
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

func (a *Adapter) Raw(ctx context.Context, auth AuthRef, method, path string, body any, headers map[string]string) ([]byte, int, error) {
	req, err := a.buildRequest(ctx, auth, method, path, body, headers)
	if err != nil {
		return nil, 0, fmt.Errorf("%s %s failed: %w", method, path, err)
	}
	return a.send(req, auth, method, path)
}

// RawUpload sends a body that is already encoded — a multipart upload, say.
// Raw marshals its body as JSON, which would turn file bytes into a base64
// string instead of a file.
func (a *Adapter) RawUpload(ctx context.Context, auth AuthRef, method, path, contentType string, body []byte) ([]byte, int, error) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	req, err := http.NewRequestWithContext(ctx, method, a.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("%s %s failed: create request: %w", method, path, err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "project-phoenix-integration/0.1")
	applyAuth(req, auth)
	return a.send(req, auth, method, path)
}

// send executes a prepared request and turns a 4xx/5xx into an APIError.
func (a *Adapter) send(req *http.Request, auth AuthRef, method, path string) ([]byte, int, error) {
	if a.verbose {
		fmt.Printf("  -> %s %s (%s)\n", method, path, authModeLabel(auth))
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("%s %s failed: execute request: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("%s %s failed: read response with status %d: %w", method, path, resp.StatusCode, err)
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
			Code:       envelope.Code,
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
		Code    string `json:"code"`
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
			Code:       strings.TrimSpace(payload.Code),
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
	body = strings.TrimSpace(body)
	if len(body) <= 200 {
		return body
	}
	return body[:200] + "..."
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
