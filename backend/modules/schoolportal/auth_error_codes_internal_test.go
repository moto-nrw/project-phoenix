package schoolportal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// refusalRuntime refuses every login, token issue and school switch with
// err, as the owner reports it.
func refusalRuntime(err error) *Resource {
	return &Resource{AuthService: AuthRuntime{
		Login: func(context.Context, string, string, string, string, string) (*identityaccess.LoginResult, error) {
			return nil, err
		},
		LoginAtSchool: func(context.Context, string, string, string, string, string, string) (*identityaccess.LoginResult, error) {
			return nil, err
		},
		IssueTokens: func(context.Context, VerifiedChallenge, string, string) (string, string, error) {
			return "", "", err
		},
		SwitchSchool: func(context.Context, int64, string, string, string, string) (string, string, error) {
			return "", "", err
		},
	}}
}

func owner(err error) error { return &identityaccess.AuthenticationError{Op: "school", Err: err} }

var errPortalNotBooked = &identityaccess.PortalNotBookedError{Module: "unterricht_vormittag", ModuleLabel: "Unterricht am Vormittag"}

func postLogin(rs *Resource) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"lehrkraft@example.com","password":"secret"}`))
	request.Header.Set("Content-Type", "application/json")
	rs.login(recorder, request)
	return recorder
}

type refusalCase struct {
	err    error
	status int
	code   string
}

func decodeRefusal(t *testing.T, recorder *httptest.ResponseRecorder) (int, string) {
	t.Helper()
	var body struct {
		Status int    `json:"status"`
		Code   string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body), recorder.Body.String())
	assert.Equal(t, recorder.Code, body.Status, recorder.Body.String())
	return recorder.Code, body.Code
}

// The school login names every refusal by the code the tenant login sends
// for the same case (#2520): a wrong password, a foreign school and an
// unknown address share identity.invalid_credentials, so the login page
// tells a caller nothing about other schools.
func TestSchoolLoginRefusalsCarryIdentityCodes(t *testing.T) {
	t.Parallel()

	for _, tc := range []refusalCase{
		{owner(identityaccess.ErrInvalidCredentials), http.StatusUnauthorized, "identity.invalid_credentials"},
		{owner(identityaccess.ErrAccountNotFound), http.StatusUnauthorized, "identity.invalid_credentials"},
		{owner(identityaccess.ErrAccountInactive), http.StatusUnauthorized, "identity.session_account_inactive"},
		{owner(identityaccess.ErrAccountNoSchoolPortalRole), http.StatusForbidden, "school.no_school_portal_role"},
		{owner(identityaccess.ErrTenantNotFound), http.StatusNotFound, "general.input"},
		{owner(identityaccess.ErrTenantAccessDenied), http.StatusUnauthorized, "identity.invalid_credentials"},
		{owner(identityaccess.ErrMFALocked), http.StatusTooManyRequests, "identity.mfa_blocked"},
		{owner(identityaccess.ErrMFAStatusUnavailable), http.StatusServiceUnavailable, "identity.mfa_status_unavailable"},
		{owner(errPortalNotBooked), http.StatusForbidden, "module.not_booked"},
	} {
		t.Run(tc.err.Error(), func(t *testing.T) {
			t.Parallel()
			recorder := postLogin(refusalRuntime(tc.err))
			status, code := decodeRefusal(t, recorder)
			assert.Equal(t, tc.status, status)
			assert.Equal(t, tc.code, code)
		})
	}
}

// The token exchange after the second factor answers like the login.
func TestSchoolExchangeRefusalsCarryIdentityCodes(t *testing.T) {
	t.Parallel()

	for _, tc := range []refusalCase{
		{owner(identityaccess.ErrInvalidToken), http.StatusUnauthorized, "identity.mfa_code_invalid"},
		{owner(identityaccess.ErrAccountNotFound), http.StatusUnauthorized, "identity.invalid_credentials"},
		{owner(identityaccess.ErrAccountInactive), http.StatusUnauthorized, "identity.session_account_inactive"},
		{owner(identityaccess.ErrAccountNoSchoolPortalRole), http.StatusForbidden, "school.no_school_portal_role"},
		{owner(identityaccess.ErrTenantNotFound), http.StatusNotFound, "general.input"},
		{owner(identityaccess.ErrTenantAccessDenied), http.StatusUnauthorized, "identity.invalid_credentials"},
		{owner(errPortalNotBooked), http.StatusForbidden, "module.not_booked"},
	} {
		t.Run(tc.err.Error(), func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			refusalRuntime(tc.err).completeSchoolExchange(recorder, httptest.NewRequest(http.MethodPost, "/auth/mfa/verify", nil), VerifiedChallenge{AccountID: 1, TenantID: 2}, false)
			status, code := decodeRefusal(t, recorder)
			assert.Equal(t, tc.status, status)
			assert.Equal(t, tc.code, code)
		})
	}
}

// A signed-in teacher switching schools gets the codes of the tenant switch.
func TestSchoolSwitchRefusalsCarryIdentityCodes(t *testing.T) {
	t.Parallel()

	for _, tc := range []refusalCase{
		{owner(identityaccess.ErrInvalidToken), http.StatusUnauthorized, "identity.mfa_session_required"},
		{owner(identityaccess.ErrMFAStatusUnavailable), http.StatusServiceUnavailable, "identity.mfa_status_unavailable"},
		{owner(identityaccess.ErrAccountNotFound), http.StatusUnauthorized, "identity.account_not_found"},
		{owner(identityaccess.ErrAccountInactive), http.StatusUnauthorized, "identity.session_account_inactive"},
		{owner(identityaccess.ErrTenantNotFound), http.StatusNotFound, "general.input"},
		{owner(identityaccess.ErrTenantAccessDenied), http.StatusUnauthorized, "identity.tenant_access_denied"},
		{owner(identityaccess.ErrAccountNoSchoolPortalRole), http.StatusForbidden, "school.no_school_portal_role"},
		{owner(errPortalNotBooked), http.StatusForbidden, "module.not_booked"},
	} {
		t.Run(tc.err.Error(), func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/auth/switch-school", strings.NewReader(`{"tenant_slug":"other-school"}`))
			request.Header.Set("Content-Type", "application/json")
			refusalRuntime(tc.err).switchSchool(recorder, request)
			status, code := decodeRefusal(t, recorder)
			assert.Equal(t, tc.status, status)
			assert.Equal(t, tc.code, code)
		})
	}
}

func TestSchoolSwitchUsesFamilyFromAuthenticatedClaims(t *testing.T) {
	t.Parallel()
	resource := refusalRuntime(nil)
	resource.AuthService.SwitchSchool = func(_ context.Context, _ int64, slug, _, _, family string) (string, string, error) {
		require.Equal(t, "target", slug)
		require.Equal(t, "signed-source-family", family)
		return "example-access", "example-refresh", nil
	}
	request := httptest.NewRequest(http.MethodPost, "/auth/switch-school", strings.NewReader(`{"tenant_slug":"target","family_id":"untrusted-family","mfa_verified":true}`))
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(context.WithValue(request.Context(), sessiontoken.CtxClaims,
		sessiontoken.AppClaims{FamilyID: "signed-source-family"}))
	response := httptest.NewRecorder()
	resource.switchSchool(response, request)
	require.Equal(t, http.StatusOK, response.Code)
}

// The refusal of a school without Unterricht am Vormittag names the module
// for the explanation on the login page (#4895).
func TestSchoolLoginPortalNotBookedNamesTheModule(t *testing.T) {
	t.Parallel()

	recorder := postLogin(refusalRuntime(owner(errPortalNotBooked)))

	var body struct {
		Details map[string]string `json:"details"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	assert.Equal(t, "unterricht_vormittag", body.Details["module"])
	assert.Equal(t, "Unterricht am Vormittag", body.Details["module_label"])
}
