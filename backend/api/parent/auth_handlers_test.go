package parent_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
}

func (s *stubParentAuthService) LoginParentWithAudit(
	ctx context.Context, email, password, ipAddress, userAgent string,
) (string, string, error) {
	return s.loginParent(ctx, email, password, ipAddress, userAgent)
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
