// Package routetest holds the request, response and router helpers of HTTP
// adapter tests. It builds on the shared fixture catalog only, so a test that
// needs no composed application links this package instead of api/testutil,
// whose module builders pull in the legacy service graph. api/testutil
// re-exports every helper here under its old name.
package routetest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// HTTP header constants (S1192 - avoid duplicate string literals)
const (
	headerContentType = "Content-Type"
	contentTypeJSON   = "application/json"
)

// RequestOption configures an HTTP request for testing.
type RequestOption func(*http.Request)

// WithTestTenant supplies the package test's tenant without exposing tenant
// runtime details to adapter tests.
func WithTestTenant(tb testing.TB) RequestOption {
	tenantID := testpkg.Tenant(tb)
	return func(req *http.Request) {
		*req = *req.WithContext(tenant.WithTenantID(req.Context(), tenantID))
	}
}

// ProtectedTestTenantGroup is the authorization-free test boundary for HTTP
// adapters whose permission middleware is tested separately. It retains the
// real tenant transaction and request caches.
func ProtectedTestTenantGroup(db *bun.DB, r chi.Router, fn func(chi.Router, func(http.Handler) http.Handler)) {
	r.Group(func(gr chi.Router) {
		gr.Use(testpkg.TenantTxMiddleware(db))
		fn(gr, func(next http.Handler) http.Handler { return next })
	})
}

func ProtectedTestTenantGroupFunc(db *bun.DB) func(chi.Router, func(chi.Router, func(http.Handler) http.Handler)) {
	return func(r chi.Router, fn func(chi.Router, func(http.Handler) http.Handler)) {
		ProtectedTestTenantGroup(db, r, fn)
	}
}

func UnprotectedGroupFunc() func(chi.Router, func(chi.Router, func(http.Handler) http.Handler)) {
	return func(r chi.Router, fn func(chi.Router, func(http.Handler) http.Handler)) {
		fn(r, IdentityMiddleware)
	}
}

func RecordingUnprotectedGroupFunc(called *bool) func(chi.Router, func(chi.Router, func(http.Handler) http.Handler)) {
	return func(r chi.Router, fn func(chi.Router, func(http.Handler) http.Handler)) {
		*called = true
		fn(r, IdentityMiddleware)
	}
}

func IdentityMiddleware(next http.Handler) http.Handler { return next }

func RespondSuccess(w http.ResponseWriter, r *http.Request, status int, data any, message string) {
	render.Status(r, status)
	render.JSON(w, r, Response{Status: "success", Data: data, Message: message})
}

func RespondNoContent(w http.ResponseWriter, r *http.Request) { render.NoContent(w, r) }

func RespondError(w http.ResponseWriter, r *http.Request, status int, err error) {
	render.Status(r, status)
	render.JSON(w, r, Response{Status: "error", Error: err.Error()})
}

func RespondInvalidRequest(w http.ResponseWriter, r *http.Request, err error) {
	RespondError(w, r, http.StatusBadRequest, err)
}

// RespondCoded renders the shared error envelope with a stable code and the
// optional structured details, the way api/common does for the kiosk. It
// stands in for a composition root's failure renderer in HTTP adapter tests
// that must not import the shared HTTP package themselves.
func RespondCoded(w http.ResponseWriter, r *http.Request, status int, code string, err error, details map[string]string, clientMessage string) {
	message := clientMessage
	if message == "" && err != nil {
		message = err.Error()
	}
	var payload map[string]any
	if len(details) > 0 {
		payload = make(map[string]any, len(details))
		for key, value := range details {
			payload[key] = value
		}
	}
	render.Status(r, status)
	render.JSON(w, r, struct {
		Status  string         `json:"status"`
		Error   string         `json:"error,omitempty"`
		Code    string         `json:"code,omitempty"`
		Details map[string]any `json:"details,omitempty"`
	}{Status: "error", Error: message, Code: code, Details: payload})
}

func ErrorResponder(resolve func(error) (int, error)) func(http.ResponseWriter, *http.Request, error, string) {
	return func(w http.ResponseWriter, r *http.Request, err error, _ string) {
		status, responseErr := resolve(err)
		RespondError(w, r, status, responseErr)
	}
}

// WithJWTBearer sets an Authorization: Bearer <token> header on the request.
// Use together with MintTestJWT when exercising a Resource via Router(), where
// the production JWT middleware chain (Verifier → Authenticator → TenantMiddleware)
// runs and rejects requests that lack a real signed token.
func WithJWTBearer(token string) RequestOption {
	return func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

// SeedTestJWTConfig installs deterministic viper defaults for JWT auth so that
// tests work in environments without a populated .env (e.g. CI). Use it from
// init() or TestMain in test packages that build a Resource which calls
// jwt.MustNewTokenAuth() and then use MintTestJWT to sign requests — both
// callers must see the same secret.
//
// SetDefault leaves env-supplied values alone, so this is safe to call when
// AUTH_JWT_SECRET is already set in the environment.
func SeedTestJWTConfig() {
	viper.SetDefault("auth_jwt_secret", testpkg.TestJWTSecret)
	viper.SetDefault("auth_jwt_expiry", "15m")
	viper.SetDefault("auth_jwt_refresh_expiry", "1h")
}

// NewRequest creates a new HTTP request for testing.
func NewRequest(method, target string, body io.Reader, opts ...RequestOption) *http.Request {
	req := httptest.NewRequest(method, target, body)
	req.Header.Set(headerContentType, contentTypeJSON)

	for _, opt := range opts {
		opt(req)
	}

	return req
}

// NewAuthenticatedRequest creates a request with authentication context.
// This is a convenience function that combines common options.
func NewAuthenticatedRequest(t *testing.T, method, target string, body interface{}, opts ...RequestOption) *http.Request {
	t.Helper()

	var reader io.Reader
	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal JSON body: %v", err)
		}
		reader = bytes.NewBuffer(jsonBytes)
	}

	req := httptest.NewRequest(method, target, reader)
	req.Header.Set(headerContentType, contentTypeJSON)

	for _, opt := range opts {
		opt(req)
	}

	return req
}

// NewJSONRequest creates a request with JSON body.
func NewJSONRequest(t *testing.T, method, target string, body interface{}) *http.Request {
	t.Helper()

	var reader io.Reader
	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal JSON body: %v", err)
		}
		reader = bytes.NewBuffer(jsonBytes)
	}

	req := httptest.NewRequest(method, target, reader)
	req.Header.Set(headerContentType, contentTypeJSON)

	return req
}

// NewMultipartRequest creates a multipart form request with file upload.
func NewMultipartRequest(t *testing.T, method, target string, fieldName, fileName, content string, opts ...RequestOption) *http.Request {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Create form file field
	fw, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}

	if _, err := fw.Write([]byte(content)); err != nil {
		t.Fatalf("failed to write file content: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req := httptest.NewRequest(method, target, &buf)
	req.Header.Set(headerContentType, writer.FormDataContentType())

	for _, opt := range opts {
		opt(req)
	}

	return req
}

// NewTenantRouter creates a chi.Router with the test transaction boundary
// (testpkg.TenantTxMiddleware) at the root. Tests that inject identity into
// the request context get the same transaction decision as production;
// resource routers that apply the production jwt + TenantTxMiddleware chain
// themselves run it unchanged underneath, since their requests arrive here
// unauthenticated and pass through.
func NewTenantRouter(db *bun.DB) chi.Router {
	router := NewJSONRouter()
	router.Use(testpkg.TenantTxMiddleware(db))
	return router
}

// NewJSONRouter provides the JSON response middleware for isolated handler
// tests that supply their own identity and do not need a database transaction.
func NewJSONRouter() chi.Router {
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	return router
}

// ExecuteRequest executes an HTTP request against a Chi router and returns the response recorder.
func ExecuteRequest(router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	ctx := testpkg.WithPackageTenantRuntime(req.Context())
	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		// Request options inject identity before this helper installs the runtime.
		// Reapply the tenant so the adapter-owned repository scope matches the
		// production middleware order (runtime first, authentication second).
		ctx = tenant.WithTenantID(ctx, tenantID)
	}
	servedBy(ctx, router).ServeHTTP(rr, req.WithContext(ctx))
	return rr
}

// ExecuteRequestForTest is ExecuteRequest with t's disposable database
// runtime when the test opted into one.
func ExecuteRequestForTest(t *testing.T, router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	ctx := testpkg.WithTestTenantRuntime(t, req.Context())
	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		ctx = tenant.WithTenantID(ctx, tenantID)
	}
	servedBy(ctx, router).ServeHTTP(rr, req.WithContext(ctx))
	return rr
}

// Response represents a standard API response for testing.
type Response struct {
	Status  string      `json:"status"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// ParseResponse parses the response body into a Response struct.
func ParseResponse(t *testing.T, body []byte) Response {
	t.Helper()

	var response Response
	err := json.Unmarshal(body, &response)
	require.NoError(t, err, "Failed to parse response body: %s", string(body))
	return response
}

// ParseJSONResponse parses the response body into a map.
func ParseJSONResponse(t *testing.T, body []byte) map[string]interface{} {
	t.Helper()

	var response map[string]interface{}
	err := json.Unmarshal(body, &response)
	require.NoError(t, err, "Failed to parse response body: %s", string(body))
	return response
}

// AssertSuccessResponse validates that the response has success status and expected HTTP code.
func AssertSuccessResponse(t *testing.T, rr *httptest.ResponseRecorder, expectedStatus int) {
	t.Helper()

	assert.Equal(t, expectedStatus, rr.Code, "Unexpected HTTP status code. Body: %s", rr.Body.String())

	if rr.Code == http.StatusNoContent {
		return // No body to parse
	}

	response := ParseResponse(t, rr.Body.Bytes())
	assert.Equal(t, "success", response.Status, "Expected success status. Body: %s", rr.Body.String())
}

// AssertErrorResponse validates the expected HTTP code and, for a JSON body,
// the shared error envelope's members a handler writes itself: status "error"
// and the text in `error`, never `message` (#2507). The remaining problem
// members are filled by ProblemResponseMiddleware, which handler tests bypass.
func AssertErrorResponse(t *testing.T, rr *httptest.ResponseRecorder, expectedStatus int) {
	t.Helper()

	assert.Equal(t, expectedStatus, rr.Code, "Unexpected HTTP status code. Body: %s", rr.Body.String())
	assertHandlerErrorEnvelope(t, rr)
}

func assertHandlerErrorEnvelope(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	var body map[string]json.RawMessage
	if json.Unmarshal(rr.Body.Bytes(), &body) != nil || body == nil {
		return // plain-text bodies get their envelope from ProblemResponseMiddleware
	}
	if status, ok := body["status"]; ok {
		assert.JSONEq(t, `"error"`, string(status), "error body status must be \"error\". Body: %s", rr.Body.String())
	}
	assert.NotContains(t, body, "message", "error body text belongs in \"error\". Body: %s", rr.Body.String())
}

// AssertUnauthorized validates a 401 Unauthorized response.
func AssertUnauthorized(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	AssertErrorResponse(t, rr, http.StatusUnauthorized)
}

// AssertForbidden validates a 403 Forbidden response in the shared envelope.
func AssertForbidden(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	assert.Equal(t, http.StatusForbidden, rr.Code, "Expected 403 Forbidden. Body: %s", rr.Body.String())
	assertHandlerErrorEnvelope(t, rr)
}

// AssertNotFound validates a 404 Not Found response.
func AssertNotFound(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	AssertErrorResponse(t, rr, http.StatusNotFound)
}

// AssertBadRequest validates a 400 Bad Request response.
func AssertBadRequest(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	AssertErrorResponse(t, rr, http.StatusBadRequest)
}

// WithSessionVerifier mounts the verifier the API root mounts once for every
// route. A resource router served on its own has none and rejects each token.
func WithSessionVerifier(next http.Handler) http.Handler { return testpkg.SessionVerifier(next) }

// servedBy mounts the session verifier unless the test already placed a
// verified token on the context, which the verifier would overwrite.
func servedBy(ctx context.Context, router http.Handler) http.Handler {
	if token, _, _ := jwtauth.FromContext(ctx); token != nil {
		return router
	}
	return WithSessionVerifier(router)
}
