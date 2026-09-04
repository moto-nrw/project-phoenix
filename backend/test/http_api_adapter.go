package test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

type HTTPRequest = http.Request
type HTTPResponseWriter = http.ResponseWriter
type HTTPClient = http.Client
type HTTPServer = HTTPTestServer

type HTTPTestServer struct {
	URL      string
	listener net.Listener
	server   *http.Server
}

const (
	HTTPMethodGet    = http.MethodGet
	HTTPMethodPost   = http.MethodPost
	HTTPMethodPut    = http.MethodPut
	HTTPMethodDelete = http.MethodDelete

	HTTPStatusOK                  = http.StatusOK
	HTTPStatusCreated             = http.StatusCreated
	HTTPStatusNoContent           = http.StatusNoContent
	HTTPStatusBadRequest          = http.StatusBadRequest
	HTTPStatusUnauthorized        = http.StatusUnauthorized
	HTTPStatusForbidden           = http.StatusForbidden
	HTTPStatusNotFound            = http.StatusNotFound
	HTTPStatusConflict            = http.StatusConflict
	HTTPStatusInternalServerError = http.StatusInternalServerError
	HTTPStatusServiceUnavailable  = http.StatusServiceUnavailable
)

func NewHTTPTestServer(handler func(HTTPResponseWriter, *HTTPRequest)) *HTTPTestServer {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(fmt.Sprintf("start HTTP test server: %v", err))
	}
	server := &http.Server{Handler: http.HandlerFunc(handler), ReadHeaderTimeout: time.Second}
	testServer := &HTTPTestServer{URL: "http://" + listener.Addr().String(), listener: listener, server: server}
	go func() { _ = server.Serve(listener) }()
	return testServer
}

func NewHTTPRequest(method, url string, body io.Reader) (*HTTPRequest, error) {
	return http.NewRequest(method, url, body)
}

func HTTPNotFound(response HTTPResponseWriter, request *HTTPRequest) {
	http.NotFound(response, request)
}

func (s *HTTPTestServer) Close() {
	_ = s.server.Close()
	_ = s.listener.Close()
}

type APIAuth struct {
	Kind, Label, Token, APIKey, PIN string
}

type APIRequestError struct {
	Method     string
	Path       string
	StatusCode int
	Code       string
	Message    string
	Body       string
}

func (e *APIRequestError) Error() string {
	if e.Message != "" {
		if e.Code != "" {
			return fmt.Sprintf("%s %s failed: %d (%s) - %s", e.Method, e.Path, e.StatusCode, e.Code, e.Message)
		}
		return fmt.Sprintf("%s %s failed: %d - %s", e.Method, e.Path, e.StatusCode, e.Message)
	}
	return fmt.Sprintf("%s %s failed: %d - %s", e.Method, e.Path, e.StatusCode, e.Body)
}

func (e *APIRequestError) HTTPStatusCode() int   { return e.StatusCode }
func (e *APIRequestError) HTTPErrorCode() string { return e.Code }

type HTTPAPIAdapter struct {
	baseURL string
	client  *http.Client
}

func NewHTTPAPIAdapter(baseURL string) *HTTPAPIAdapter {
	return &HTTPAPIAdapter{baseURL: strings.TrimSuffix(baseURL, "/"), client: &http.Client{Timeout: 5 * time.Second}}
}

func (a *HTTPAPIAdapter) BaseURL() string { return a.baseURL }

func (a *HTTPAPIAdapter) CheckHealth(ctx context.Context) error {
	_, status, err := a.Raw(ctx, APIAuth{}, http.MethodGet, "/health", nil, nil)
	if err != nil {
		var requestErr *APIRequestError
		if !errors.As(err, &requestErr) {
			return fmt.Errorf("server not reachable at %s: %w", a.baseURL, err)
		}
		return fmt.Errorf("server health check failed: %w", err)
	}
	if status != http.StatusOK {
		return fmt.Errorf("GET /health failed: status %d", status)
	}
	return nil
}

func (a *HTTPAPIAdapter) Login(ctx context.Context, path, email, password, tenantSlug string) (APIAuth, error) {
	body := map[string]string{"email": email, "password": password}
	if tenantSlug != "" {
		body["tenant_slug"] = tenantSlug
	}
	raw, status, err := a.Raw(ctx, APIAuth{}, http.MethodPost, path, body, nil)
	if err != nil {
		label := apiLoginLabel(path)
		return APIAuth{}, fmt.Errorf("%s request failed: %w", label, err)
	}
	var response struct {
		AccessToken string `json:"access_token"`
		Data        struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return APIAuth{}, fmt.Errorf("parse %s response from POST %s with status %d: %w", apiLoginLabel(path), path, status, err)
	}
	if response.Data.AccessToken != "" {
		response.AccessToken = response.Data.AccessToken
	}
	if response.AccessToken == "" {
		return APIAuth{}, fmt.Errorf("POST %s response with status %d has no access token", path, status)
	}
	return APIAuth{Kind: "bearer", Token: response.AccessToken}, nil
}

func apiLoginLabel(path string) string {
	switch path {
	case "/operator/auth/login":
		return "operator login"
	case "/parent/auth/login":
		return "parent login"
	default:
		return "login"
	}
}

func (a *HTTPAPIAdapter) Raw(ctx context.Context, auth APIAuth, method, path string, body any, headers map[string]string) ([]byte, int, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		reader = bytes.NewReader(encoded)
	}
	return a.do(ctx, auth, method, path, "application/json", reader, headers)
}

func (a *HTTPAPIAdapter) RawUpload(ctx context.Context, auth APIAuth, method, path, contentType string, body []byte) ([]byte, int, error) {
	return a.do(ctx, auth, method, path, contentType, bytes.NewReader(body), nil)
}

func (a *HTTPAPIAdapter) do(ctx context.Context, auth APIAuth, method, path, contentType string, body io.Reader, headers map[string]string) ([]byte, int, error) {
	request, err := http.NewRequestWithContext(ctx, method, a.baseURL+path, body)
	if err != nil {
		return nil, 0, fmt.Errorf("%s %s failed: %w", method, path, err)
	}
	if body != nil {
		request.Header.Set("Content-Type", contentType)
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	if auth.Kind == "bearer" && auth.Token != "" {
		request.Header.Set("Authorization", "Bearer "+auth.Token)
	}
	if auth.Kind == "device" {
		request.Header.Set("Authorization", "Bearer "+auth.APIKey)
		request.Header.Set("X-Staff-PIN", auth.PIN)
	}
	response, err := a.client.Do(request)
	if err != nil {
		return nil, 0, fmt.Errorf("%s %s failed: %w", method, path, err)
	}
	defer func() { _ = response.Body.Close() }()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, response.StatusCode, fmt.Errorf("%s %s failed reading status %d response: %w", method, path, response.StatusCode, err)
	}
	if response.StatusCode >= http.StatusBadRequest {
		var payload struct {
			Message string `json:"message"`
			Error   string `json:"error"`
			Code    string `json:"code"`
		}
		_ = json.Unmarshal(raw, &payload)
		if payload.Message == "" {
			payload.Message = payload.Error
		}
		return raw, response.StatusCode, &APIRequestError{Method: method, Path: path, StatusCode: response.StatusCode, Code: payload.Code, Message: payload.Message, Body: string(raw)}
	}
	return raw, response.StatusCode, nil
}
