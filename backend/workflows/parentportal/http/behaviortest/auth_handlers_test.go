package parent_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	rootAPI "github.com/moto-ogs/project-phoenix/api"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	parent "github.com/moto-ogs/project-phoenix/workflows/parentportal/http"
)

// stubParentAuthService supplies the reset runtime the composition root
// binds in production. The sign-in routes are the owner's (parentauth, which
// tests the login itself); here a stand-in mounts /login so the rate limit
// of the public routes stays covered.
type stubParentAuthService struct {
	resetParent func(ctx context.Context, email string) error
	confirm     func(ctx context.Context, token, newPassword string) error
}

type stubSignInRoutes struct{}

func (stubSignInRoutes) MountSignIn(r chi.Router, _ func(context.Context, string)) {
	r.Post("/login", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
}
func (stubSignInRoutes) MountSession(chi.Router)          {}
func (stubSignInRoutes) MountCredentialChecks(chi.Router) {}

func (s *stubParentAuthService) loginRuntime() parent.LoginRuntime {
	return parent.LoginRuntime{Routes: stubSignInRoutes{}}
}

// The reset runtime the composition root binds (#3332), stood up here from
// the stub's two closures plus the classifiers the root supplies in
// production. The sentinels are the stub's own, so a test proves the
// handler's mapping and not the owner's error shapes.
var (
	errStubResetLinkUnusable = errors.New("stub: reset link is unusable")
	errStubPasswordTooWeak   = errors.New("stub: password is too weak")
)

// errStubRateLimited carries the retry window a rate-limited stub reports.
type errStubRateLimited struct{ retryAfter int }

func (e *errStubRateLimited) Error() string { return "stub: too many password reset requests" }

func (s *stubParentAuthService) resetRuntime() parent.PasswordResetRoutes {
	return parent.PasswordResetRoutes{
		Initiate: func(ctx context.Context, email string) error { return s.resetParent(ctx, email) },
		Reset: func(ctx context.Context, token, newPassword string) error {
			return s.confirm(ctx, token, newPassword)
		},
		LinkUnusable: func(err error) bool { return errors.Is(err, errStubResetLinkUnusable) },
		TooWeak:      func(err error) bool { return errors.Is(err, errStubPasswordTooWeak) },
		RetryAfter: func(err error) (int, bool) {
			var limited *errStubRateLimited
			if !errors.As(err, &limited) {
				return 0, false
			}
			return limited.retryAfter, true
		},
	}
}

// newTestResource wires a Resource with just the AuthService and the reset
// runtime — the login handler doesn't touch ParentService, RequestService,
// GuardianProfileLoader, SchoolRepo, AccountTenantRepo, or db. If a future
// change to the handler reaches into them, the nil deref will surface as a
// test failure, which is the desired guardrail.
func newTestResource(svc *stubParentAuthService) *parent.Resource {
	return parent.NewResource(parent.ResourceConfig{Auth: svc.loginRuntime(), Resets: svc.resetRuntime()})
}

func postPasswordReset(t *testing.T, rs *parent.Resource, body map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	jsonBody, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/auth/password-reset", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	rs.Router(rootAPI.ParentSessionChain).ServeHTTP(rr, req)
	return rr
}

func postPasswordResetConfirm(t *testing.T, rs *parent.Resource, body map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	jsonBody, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/auth/password-reset/confirm", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	rs.Router(rootAPI.ParentSessionChain).ServeHTTP(rr, req)
	return rr
}

type loginErrorBody struct {
	Status int    `json:"status"`
	Detail string `json:"detail"`
	Code   string `json:"code"`
}

func decodeError(t *testing.T, rr *httptest.ResponseRecorder) loginErrorBody {
	t.Helper()
	var body loginErrorBody
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	return body
}

func TestParentPasswordReset_NeutralSuccess(t *testing.T) {
	t.Parallel()

	called := false
	svc := &stubParentAuthService{
		resetParent: func(_ context.Context, email string) error {
			called = true
			assert.Equal(t, "parent@example.com", email)
			return nil
		},
	}

	rr := postPasswordReset(t, newTestResource(svc), map[string]string{
		"email": "PARENT@EXAMPLE.COM",
	})

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "If the email exists")
}

func TestParentPasswordReset_RateLimitedSetsRetryAfter(t *testing.T) {
	t.Parallel()

	svc := &stubParentAuthService{
		resetParent: func(_ context.Context, _ string) error {
			return &errStubRateLimited{retryAfter: 600}
		},
	}

	rr := postPasswordReset(t, newTestResource(svc), map[string]string{
		"email": "parent@example.com",
	})

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.NotEmpty(t, rr.Header().Get("Retry-After"))
	// Same code as the staff flow, so parents read "wait and try again" in
	// their language (#2518).
	assert.Equal(t, "identity.password_reset_rate_limited", decodeError(t, rr).Code)
}

func TestParentPasswordResetConfirm_Success(t *testing.T) {
	t.Parallel()

	called := false
	svc := &stubParentAuthService{
		confirm: func(_ context.Context, token, newPassword string) error {
			called = true
			assert.Equal(t, "reset-token", token)
			assert.Equal(t, "Str0ngP@ssword!", newPassword)
			return nil
		},
	}

	rr := postPasswordResetConfirm(t, newTestResource(svc), map[string]string{
		"token":            "reset-token",
		"new_password":     "Str0ngP@ssword!",
		"confirm_password": "Str0ngP@ssword!",
	})

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// --- Validation + error-branch coverage for the three handlers. These guard
// the HTTP status contract the parents portal depends on: a 400 keeps the user
// on the form with a correction hint, a 500 surfaces a generic failure. A
// regression that collapsed these into one status would either hide a real
// server error behind a "check your input" message or vice versa.

func TestParentPasswordReset_MalformedEmail_Returns400(t *testing.T) {
	t.Parallel()

	svc := &stubParentAuthService{
		resetParent: func(_ context.Context, _ string) error {
			t.Fatal("service must not be called for an invalid request body")
			return nil
		},
	}
	rr := postPasswordReset(t, newTestResource(svc), map[string]string{
		"email": "not-an-email",
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestParentPasswordReset_UnexpectedError_Returns500(t *testing.T) {
	t.Parallel()

	svc := &stubParentAuthService{
		resetParent: func(_ context.Context, _ string) error {
			return errors.New("smtp config missing")
		},
	}
	rr := postPasswordReset(t, newTestResource(svc), map[string]string{
		"email": "parent@example.com",
	})

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestParentPasswordResetConfirm_PasswordMismatch_Returns400(t *testing.T) {
	t.Parallel()

	// Bind() enforces new_password == confirm_password before the service runs.
	svc := &stubParentAuthService{
		confirm: func(_ context.Context, _, _ string) error {
			t.Fatal("service must not be called when passwords do not match")
			return nil
		},
	}
	rr := postPasswordResetConfirm(t, newTestResource(svc), map[string]string{
		"token":            "reset-token",
		"new_password":     "Str0ngP@ssword!",
		"confirm_password": "Different1!",
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestParentPasswordResetConfirm_InvalidToken_Returns410(t *testing.T) {
	t.Parallel()

	// An expired/unknown/used token comes back as ErrInvalidToken and must map
	// to 410 Gone, which the parents reset page renders as the "request a new
	// link" copy — distinct from the 400 weak-password case (fix your password)
	// and from a 500 (server fault).
	svc := &stubParentAuthService{
		confirm: func(_ context.Context, _, _ string) error {
			return errStubResetLinkUnusable
		},
	}
	rr := postPasswordResetConfirm(t, newTestResource(svc), map[string]string{
		"token":            "expired-token",
		"new_password":     "Str0ngP@ssword!",
		"confirm_password": "Str0ngP@ssword!",
	})

	assert.Equal(t, http.StatusGone, rr.Code)
	assert.Equal(t, "identity.password_reset_link_invalid", decodeError(t, rr).Code)
}

func TestParentPasswordResetConfirm_WeakPassword_Returns400(t *testing.T) {
	t.Parallel()

	// The bind layer only checks length; the service enforces full complexity
	// and returns ErrPasswordTooWeak, which must still surface as a 400.
	svc := &stubParentAuthService{
		confirm: func(_ context.Context, _, _ string) error {
			return errStubPasswordTooWeak
		},
	}
	rr := postPasswordResetConfirm(t, newTestResource(svc), map[string]string{
		"token":            "reset-token",
		"new_password":     "alllowercase1!",
		"confirm_password": "alllowercase1!",
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	body := decodeError(t, rr)
	assert.Equal(t, "identity.password_too_weak", body.Code)
	// The field is marked, so the reset form focuses the password input.
	assert.Contains(t, rr.Body.String(), `"field":"new_password"`)
}

func TestParentPasswordResetConfirm_UnexpectedError_Returns500(t *testing.T) {
	t.Parallel()

	svc := &stubParentAuthService{
		confirm: func(_ context.Context, _, _ string) error {
			return errors.New("token store unavailable")
		},
	}
	rr := postPasswordResetConfirm(t, newTestResource(svc), map[string]string{
		"token":            "reset-token",
		"new_password":     "Str0ngP@ssword!",
		"confirm_password": "Str0ngP@ssword!",
	})

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// TestRouterWithAuthRateLimiter_ThrottlesOnlyPublicAuthRoutes pins the
// composition root's brute-force guard: the limiter passed to
// RouterWithAuthRateLimiter wraps every public /auth route and nothing that
// sits behind the parent-scope token check.
func TestRouterWithAuthRateLimiter_ThrottlesOnlyPublicAuthRoutes(t *testing.T) {
	t.Parallel()
	limiter := func(http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
		})
	}
	router := newTestResource(&stubParentAuthService{}).RouterWithAuthRateLimiter(limiter, rootAPI.ParentSessionChain)

	for _, path := range []string{"/auth/login", "/auth/password-reset", "/auth/password-reset/confirm"} {
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, path, bytes.NewReader([]byte("{}"))))
		assert.Equal(t, http.StatusTooManyRequests, rr.Code, path)
	}

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/me/profile", nil))
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}
