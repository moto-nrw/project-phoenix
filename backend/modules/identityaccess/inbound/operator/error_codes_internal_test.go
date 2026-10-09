package operator

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// renderedCode renders r and returns the status and wire code.
func renderedCode(t *testing.T, r render.Renderer) (int, string) {
	t.Helper()
	w := httptest.NewRecorder()
	require.NoError(t, render.Render(w, httptest.NewRequest(http.MethodPost, "/", nil), r))
	return w.Code, wireCode(t, w)
}

func wireCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body.Code
}

type codeCase struct {
	name   string
	render render.Renderer
	status int
	code   string
}

func assertCodes(t *testing.T, cases []codeCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			status, code := renderedCode(t, tc.render)
			assert.Equal(t, tc.status, status)
			assert.Equal(t, tc.code, code)
		})
	}
}

// #2519: the operator identity outcomes name their reason by code.
func TestOperatorIdentityErrorsAnswerWithCodes(t *testing.T) {
	t.Parallel()

	// The cases below never reach the access fallback.
	rs := NewResource(nil, OperatorResponses(func(error) render.Renderer { return nil }))
	assertCodes(t, []codeCase{
		{"login invalid", rs.authError(identityaccess.ErrOperatorInvalidCredentials), http.StatusUnauthorized, "identity.invalid_credentials"},
		{"login inactive", rs.authError(identityaccess.ErrOperatorInactive), http.StatusForbidden, "identity.account_inactive"},
		{"profile password", rs.profileError(identityaccess.ErrOperatorPasswordMismatch), http.StatusBadRequest, "identity.current_password_wrong"},
		{"profile missing", rs.profileError(identityaccess.ErrOperatorNotFound), http.StatusNotFound, "identity.account_not_found"},
		{"profile inactive", rs.profileError(identityaccess.ErrOperatorInactive), http.StatusForbidden, "identity.account_inactive"},
		{"profile weak password", rs.profileError(&identityaccess.InvalidInputError{Err: errors.New("password doesn't meet complexity requirements")}), http.StatusBadRequest, "identity.password_too_weak"},
		{"access account", rs.accessError(identityaccess.ErrAccountNotFound), http.StatusNotFound, "identity.account_not_found"},
		{"access missing", rs.accessError(identityaccess.ErrAccountTenantAccessNotFound), http.StatusNotFound, "identity.tenant_access_not_found"},
		{"access exists", rs.accessError(identityaccess.ErrAccountTenantAccessExists), http.StatusConflict, "identity.account_already_has_tenant_access"},
		{"access school", rs.accessError(identityaccess.ErrSchoolNotFound), http.StatusNotFound, "provisioning.school_not_found"},
		{"access school deleted", rs.accessError(identityaccess.ErrSchoolDeleted), http.StatusConflict, "provisioning.school_already_deleted"},
		{"access lehrkraft", rs.accessError(&identityaccess.InvalidInputError{Err: identityaccess.ErrLehrkraftRoleImmutable}), http.StatusBadRequest, "identity.lehrkraft_role_immutable"},
		{"auth mfa locked", AuthErrorRenderer(ErrMFALocked), http.StatusTooManyRequests, "identity.mfa_blocked"},
		{"auth mfa code", AuthErrorRenderer(ErrMFACodeInvalid), http.StatusUnauthorized, "identity.mfa_code_invalid"},
		{"auth inactive", AuthErrorRenderer(ErrOperatorInactive), http.StatusForbidden, "identity.account_inactive"},
		{"invitation missing", invitationErrorRenderer(ErrOperatorInvitationNotFound), http.StatusNotFound, "identity.invitation_not_found"},
		{"invitation operator exists", invitationErrorRenderer(ErrOperatorEmailExists), http.StatusConflict, "identity.email_already_exists"},
		{"invitation rate", invitationErrorRenderer(ErrOperatorInvitationRateLimited), http.StatusTooManyRequests, "identity.invitation_rate_limited"},
		{"public invitation", publicInvitationErrorRenderer(ErrOperatorInvitationNotFound), http.StatusBadRequest, "identity.operator_invitation_invalid"},
		{"email in use", ProfileErrorRenderer(ErrOperatorEmailInUse), http.StatusConflict, "identity.email_already_exists"},
		{"legacy profile missing", ProfileErrorRenderer(ErrOperatorNotFound), http.StatusNotFound, "identity.account_not_found"},
		{"email rate", ProfileErrorRenderer(ErrOperatorEmailChangeRateLimited), http.StatusTooManyRequests, "identity.email_change_rate_limited"},
		{"email same", ProfileErrorRenderer(ErrOperatorEmailChangeSameEmail), http.StatusBadRequest, "identity.email_change_same_email"},
		{"email link", ProfileErrorRenderer(ErrOperatorEmailChangeNotFound), http.StatusBadRequest, "identity.email_change_link_invalid"},
		{"email confirm link", confirmEmailChangeErrorRenderer(ErrOperatorEmailChangeNotFound), http.StatusBadRequest, "identity.email_change_link_invalid"},
	})
}

func TestOperatorInvitationInputMarksField(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	require.NoError(t, render.Render(w, httptest.NewRequest(http.MethodPost, "/", nil),
		invitationErrorRenderer(&identityaccess.InvalidInputError{Err: errors.New("display name is required")})))
	var body struct {
		Code   string `json:"code"`
		Errors []struct {
			Field string `json:"field"`
		} `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "general.input", body.Code)
	require.Len(t, body.Errors, 1)
	assert.Equal(t, "display_name", body.Errors[0].Field)
}

func TestOperatorMFAAndPasskeyErrorsAnswerWithCodes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		write  func(http.ResponseWriter, *http.Request)
		status int
		code   string
	}{
		{"mfa not enrolled", func(w http.ResponseWriter, r *http.Request) { mapOperatorMFAError(w, r, ErrMFANotEnrolled) }, http.StatusForbidden, "identity.mfa_not_enrolled"},
		{"mfa enrolled", func(w http.ResponseWriter, r *http.Request) { mapOperatorMFAError(w, r, ErrMFAAlreadyEnrolled) }, http.StatusConflict, "identity.mfa_already_enrolled"},
		{"passkey failed", func(w http.ResponseWriter, r *http.Request) { mapOperatorPasskeyError(w, r, ErrPasskeySessionInvalid) }, http.StatusUnauthorized, "identity.passkey_login_failed"},
		{"passkey origin", func(w http.ResponseWriter, r *http.Request) { mapOperatorPasskeyError(w, r, ErrPasskeyOriginInvalid) }, http.StatusForbidden, "identity.passkey_login_failed"},
		{"passkey blocked", func(w http.ResponseWriter, r *http.Request) { mapOperatorPasskeyError(w, r, ErrMFALocked) }, http.StatusTooManyRequests, "identity.mfa_blocked"},
		{"passkey missing", func(w http.ResponseWriter, r *http.Request) { mapOperatorPasskeyError(w, r, ErrPasskeyNotFound) }, http.StatusNotFound, "identity.passkey_not_found"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			tc.write(w, httptest.NewRequest(http.MethodPost, "/", nil))
			assert.Equal(t, tc.status, w.Code)
			assert.Equal(t, tc.code, wireCode(t, w))
		})
	}
}
