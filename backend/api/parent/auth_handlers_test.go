package parent_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/parent"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
)

// stubParentAuthService embeds the AuthService interface so all methods exist
// at compile time. Only LoginParentWithAudit is overridden — calling any other
// method nil-derefs and fails the test loudly, which is exactly what we want
// (the parent login handler must not touch anything else).
type stubParentAuthService struct {
	authService.AuthService
	loginParent func(ctx context.Context, email, password, ip, ua string) (string, string, error)
	resetParent func(ctx context.Context, email string) (*authModel.PasswordResetToken, error)
	confirm     func(ctx context.Context, token, newPassword string) error
}

func (s *stubParentAuthService) LoginParentWithAudit(
	ctx context.Context, email, password, ipAddress, userAgent string,
) (string, string, error) {
	return s.loginParent(ctx, email, password, ipAddress, userAgent)
}

func (s *stubParentAuthService) InitiateParentPasswordReset(ctx context.Context, email string) (*authModel.PasswordResetToken, error) {
	return s.resetParent(ctx, email)
}

func (s *stubParentAuthService) ResetPassword(ctx context.Context, token, newPassword string) error {
	return s.confirm(ctx, token, newPassword)
}

// newTestResource wires a Resource with just the AuthService — the login
// handler doesn't touch ParentService, RequestService, GuardianProfileLoader,
// SchoolRepo, AccountTenantRepo, or db. If a future change to the handler
// reaches into them, the nil deref will surface as a test failure, which is
// the desired guardrail.
func newTestResource(svc authService.AuthService) *parent.Resource {
	return parent.NewResource(svc, nil, nil, nil, nil, nil)
}

// postLogin runs a single POST against the login handler via the resource's
// router so we exercise the same dispatch path the real server uses.
func postLogin(t *testing.T, rs *parent.Resource, body map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	jsonBody, err := json.Marshal(body)
	require.NoError(t, err)

	// Mount via the public router so we cover routing + middleware shape,
	// matching the convention in CLAUDE.md (rule 5: no test-export wrappers).
	// Router() returns the chi router scoped to the resource, so the
	// /parent prefix isn't part of the request path here.
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	rs.Router().ServeHTTP(rr, req)
	return rr
}

func postPasswordReset(t *testing.T, rs *parent.Resource, body map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	jsonBody, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/auth/password-reset", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	rs.Router().ServeHTTP(rr, req)
	return rr
}

func postPasswordResetConfirm(t *testing.T, rs *parent.Resource, body map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	jsonBody, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/auth/password-reset/confirm", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	rs.Router().ServeHTTP(rr, req)
	return rr
}

type loginErrorBody struct {
	Status string `json:"status"`
	Error  string `json:"error"`
	Code   string `json:"code"`
}

func decodeError(t *testing.T, rr *httptest.ResponseRecorder) loginErrorBody {
	t.Helper()
	var body loginErrorBody
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	return body
}

func TestParentLogin_InvalidCredentials_Returns401WithCode(t *testing.T) {
	t.Parallel()

	svc := &stubParentAuthService{
		loginParent: func(_ context.Context, _, _, _, _ string) (string, string, error) {
			return "", "", &authService.AuthError{Op: "login", Err: authService.ErrInvalidCredentials}
		},
	}
	rr := postLogin(t, newTestResource(svc), map[string]string{
		"email":    "parent@example.com",
		"password": "wrong",
	})

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	body := decodeError(t, rr)
	assert.Equal(t, "invalid_credentials", body.Code,
		"frontend disambiguates by this code; do not change without updating parent-config.ts")
}

func TestParentLogin_AccountNotFound_MaskedAsInvalidCredentials(t *testing.T) {
	t.Parallel()

	// Account-enumeration mask: unknown email returns the same wire shape
	// as wrong password. If this test starts failing with code="account_not_found"
	// or similar, the masking has been removed and we're leaking which
	// emails exist — that's a security regression, not a test bug.
	svc := &stubParentAuthService{
		loginParent: func(_ context.Context, _, _, _, _ string) (string, string, error) {
			return "", "", &authService.AuthError{Op: "login", Err: authService.ErrAccountNotFound}
		},
	}
	rr := postLogin(t, newTestResource(svc), map[string]string{
		"email":    "unknown@example.com",
		"password": "anything",
	})

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	body := decodeError(t, rr)
	assert.Equal(t, "invalid_credentials", body.Code)
}

func TestParentLogin_AccountInactive_Returns401WithDistinctCode(t *testing.T) {
	t.Parallel()

	// This is the case the frontend turns into
	// "Ihr Konto ist deaktiviert. Bitte kontaktieren Sie die Schule."
	// If the code is anything other than "account_inactive", the parent
	// gets the generic "check your credentials" message and is confused
	// about why login fails — that was the bug this entire fix addressed.
	svc := &stubParentAuthService{
		loginParent: func(_ context.Context, _, _, _, _ string) (string, string, error) {
			return "", "", &authService.AuthError{Op: "login", Err: authService.ErrAccountInactive}
		},
	}
	rr := postLogin(t, newTestResource(svc), map[string]string{
		"email":    "disabled@example.com",
		"password": "correct",
	})

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	body := decodeError(t, rr)
	assert.Equal(t, "account_inactive", body.Code)
}

func TestParentLogin_NotAGuardian_Returns403WithCode(t *testing.T) {
	t.Parallel()

	// Staff account hitting the parent portal. Backend MUST signal this
	// distinctly so the frontend can mask it as invalid_credentials at
	// the UI layer (account-enumeration mask) while keeping the
	// staff-login hint in the German copy. Pre-fix this returned 403
	// without a code and the frontend mis-mapped it to "Konto deaktiviert"
	// — that mismatch is exactly what this test exists to prevent.
	svc := &stubParentAuthService{
		loginParent: func(_ context.Context, _, _, _, _ string) (string, string, error) {
			return "", "", &authService.AuthError{Op: "parent login", Err: authService.ErrAccountNoGuardianRole}
		},
	}
	rr := postLogin(t, newTestResource(svc), map[string]string{
		"email":    "staff@example.com",
		"password": "correct",
	})

	assert.Equal(t, http.StatusForbidden, rr.Code)
	body := decodeError(t, rr)
	assert.Equal(t, "not_a_guardian", body.Code)
}

func TestParentLogin_Success_Returns200WithTokens(t *testing.T) {
	t.Parallel()

	svc := &stubParentAuthService{
		loginParent: func(_ context.Context, email, password, _, _ string) (string, string, error) {
			assert.Equal(t, "parent@example.com", email)
			assert.Equal(t, "correct-horse-battery", password)
			return "access-tok", "refresh-tok", nil
		},
	}
	rr := postLogin(t, newTestResource(svc), map[string]string{
		"email":    "parent@example.com",
		"password": "correct-horse-battery",
	})

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp struct {
		Status string `json:"status"`
		Data   struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "access-tok", resp.Data.AccessToken)
	assert.Equal(t, "refresh-tok", resp.Data.RefreshToken)
}

func TestParentPasswordReset_NeutralSuccess(t *testing.T) {
	t.Parallel()

	called := false
	svc := &stubParentAuthService{
		resetParent: func(_ context.Context, email string) (*authModel.PasswordResetToken, error) {
			called = true
			assert.Equal(t, "parent@example.com", email)
			return nil, nil
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

	retryAt := time.Now().Add(10 * time.Minute)
	svc := &stubParentAuthService{
		resetParent: func(_ context.Context, _ string) (*authModel.PasswordResetToken, error) {
			return nil, &authService.AuthError{
				Op: "initiate parent password reset",
				Err: &authService.RateLimitError{
					Err:      authService.ErrRateLimitExceeded,
					Attempts: 3,
					RetryAt:  retryAt,
				},
			}
		},
	}

	rr := postPasswordReset(t, newTestResource(svc), map[string]string{
		"email": "parent@example.com",
	})

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.NotEmpty(t, rr.Header().Get("Retry-After"))
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

func TestParentLogin_MalformedEmail_Returns400(t *testing.T) {
	t.Parallel()

	// Bind() rejects a non-email before the service is ever called.
	svc := &stubParentAuthService{
		loginParent: func(_ context.Context, _, _, _, _ string) (string, string, error) {
			t.Fatal("service must not be called for an invalid request body")
			return "", "", nil
		},
	}
	rr := postLogin(t, newTestResource(svc), map[string]string{
		"email":    "not-an-email",
		"password": "whatever",
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestParentLogin_UnexpectedError_Returns500(t *testing.T) {
	t.Parallel()

	// A non-AuthError (or AuthError with an unrecognized cause) is a real
	// server fault — it must not masquerade as a credentials problem.
	svc := &stubParentAuthService{
		loginParent: func(_ context.Context, _, _, _, _ string) (string, string, error) {
			return "", "", errors.New("database exploded")
		},
	}
	rr := postLogin(t, newTestResource(svc), map[string]string{
		"email":    "parent@example.com",
		"password": "correct",
	})

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestParentPasswordReset_MalformedEmail_Returns400(t *testing.T) {
	t.Parallel()

	svc := &stubParentAuthService{
		resetParent: func(_ context.Context, _ string) (*authModel.PasswordResetToken, error) {
			t.Fatal("service must not be called for an invalid request body")
			return nil, nil
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
		resetParent: func(_ context.Context, _ string) (*authModel.PasswordResetToken, error) {
			return nil, errors.New("smtp config missing")
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

func TestParentPasswordResetConfirm_TooShortPassword_Returns400(t *testing.T) {
	t.Parallel()

	// Length(8, 0) validation rejects short passwords at the bind layer.
	svc := &stubParentAuthService{
		confirm: func(_ context.Context, _, _ string) error {
			t.Fatal("service must not be called for a too-short password")
			return nil
		},
	}
	rr := postPasswordResetConfirm(t, newTestResource(svc), map[string]string{
		"token":            "reset-token",
		"new_password":     "short",
		"confirm_password": "short",
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
			return &authService.AuthError{Op: "reset password", Err: authService.ErrInvalidToken}
		},
	}
	rr := postPasswordResetConfirm(t, newTestResource(svc), map[string]string{
		"token":            "expired-token",
		"new_password":     "Str0ngP@ssword!",
		"confirm_password": "Str0ngP@ssword!",
	})

	assert.Equal(t, http.StatusGone, rr.Code)
}

func TestParentPasswordResetConfirm_WeakPassword_Returns400(t *testing.T) {
	t.Parallel()

	// The bind layer only checks length; the service enforces full complexity
	// and returns ErrPasswordTooWeak, which must still surface as a 400.
	svc := &stubParentAuthService{
		confirm: func(_ context.Context, _, _ string) error {
			return &authService.AuthError{Op: "reset password", Err: authService.ErrPasswordTooWeak}
		},
	}
	rr := postPasswordResetConfirm(t, newTestResource(svc), map[string]string{
		"token":            "reset-token",
		"new_password":     "alllowercase1!",
		"confirm_password": "alllowercase1!",
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
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
