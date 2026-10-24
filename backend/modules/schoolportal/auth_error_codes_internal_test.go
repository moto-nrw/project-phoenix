package schoolportal

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	errFakeInvalidCredentials = errors.New("fake: invalid credentials")
	errFakeAccountInactive    = errors.New("fake: account inactive")
	errFakeNoPortalRole       = errors.New("fake: no school portal role")
	errFakeSchoolNotFound     = errors.New("fake: school not found")
	errFakeSchoolAccessDenied = errors.New("fake: school access denied")
	errFakeMFABlocked         = errors.New("fake: mfa blocked")
	errFakeMFAUnavailable     = errors.New("fake: mfa unavailable")
	errFakeInvalidSession     = errors.New("fake: missing mfa proof")
)

// refusalRuntime classifies the fake sentinels like the owner's predicates
// and refuses every token issue and school switch with err.
func refusalRuntime(err error) *Resource {
	is := func(target error) func(error) bool {
		return func(got error) bool { return errors.Is(got, target) }
	}
	return &Resource{AuthService: AuthRuntime{
		IssueTokens: func(context.Context, int64, int64, string, string) (string, string, error) {
			return "", "", err
		},
		SwitchSchool: func(context.Context, int64, string, string, string, string) (string, string, error) {
			return "", "", err
		},
		Cause:              func(err error) error { return err },
		InvalidCredentials: is(errFakeInvalidCredentials),
		AccountInactive:    is(errFakeAccountInactive),
		NoSchoolPortalRole: is(errFakeNoPortalRole),
		SchoolNotFound:     is(errFakeSchoolNotFound),
		SchoolAccessDenied: is(errFakeSchoolAccessDenied),
		MFABlocked:         is(errFakeMFABlocked),
		MFAUnavailable:     is(errFakeMFAUnavailable),
		InvalidSession:     is(errFakeInvalidSession),
	}}
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
		{errFakeInvalidCredentials, http.StatusUnauthorized, "identity.invalid_credentials"},
		{errFakeAccountInactive, http.StatusUnauthorized, "identity.session_account_inactive"},
		{errFakeNoPortalRole, http.StatusForbidden, "school.no_school_portal_role"},
		{errFakeSchoolNotFound, http.StatusNotFound, "general.input"},
		{errFakeSchoolAccessDenied, http.StatusUnauthorized, "identity.invalid_credentials"},
		{errFakeMFABlocked, http.StatusTooManyRequests, "identity.mfa_blocked"},
		{errFakeMFAUnavailable, http.StatusServiceUnavailable, "general.unavailable"},
	} {
		t.Run(tc.err.Error(), func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			refusalRuntime(tc.err).handleLoginError(recorder, httptest.NewRequest(http.MethodPost, "/auth/login", nil), tc.err)
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
		{errFakeInvalidCredentials, http.StatusUnauthorized, "identity.invalid_credentials"},
		{errFakeAccountInactive, http.StatusUnauthorized, "identity.session_account_inactive"},
		{errFakeNoPortalRole, http.StatusForbidden, "school.no_school_portal_role"},
		{errFakeSchoolNotFound, http.StatusNotFound, "general.input"},
		{errFakeSchoolAccessDenied, http.StatusUnauthorized, "identity.invalid_credentials"},
	} {
		t.Run(tc.err.Error(), func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			refusalRuntime(tc.err).completeSchoolExchange(recorder, httptest.NewRequest(http.MethodPost, "/auth/mfa/verify", nil), 1, 2, false)
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
		{errFakeInvalidSession, http.StatusUnauthorized, "identity.mfa_session_required"},
		{errFakeMFAUnavailable, http.StatusServiceUnavailable, "identity.mfa_status_unavailable"},
		{errFakeInvalidCredentials, http.StatusUnauthorized, "identity.account_not_found"},
		{errFakeAccountInactive, http.StatusUnauthorized, "identity.session_account_inactive"},
		{errFakeSchoolNotFound, http.StatusNotFound, "general.input"},
		{errFakeSchoolAccessDenied, http.StatusUnauthorized, "identity.tenant_access_denied"},
		{errFakeNoPortalRole, http.StatusForbidden, "school.no_school_portal_role"},
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
