package portalauth

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// The sign-in, code and switch forms show a text per code (#2517); these
// tests pin the status and code of each refusal per operation.

type answer struct {
	status int
	code   string
}

// renderer answers one refusal through the tables of one operation.
type renderer func(w http.ResponseWriter, r *http.Request, err error)

func rendered(t *testing.T, err error, render renderer) answer {
	t.Helper()
	rr := httptest.NewRecorder()
	render(rr, httptest.NewRequest(http.MethodPost, "/", nil), err)
	var body struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body), rr.Body.String())
	return answer{rr.Code, body.Code}
}

func wrapped(err error) error { return &identityaccess.AuthenticationError{Op: "login", Err: err} }

func assertAnswers(t *testing.T, render renderer, cases map[error]answer) {
	t.Helper()
	for err, want := range cases {
		t.Run(err.Error(), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, want, rendered(t, wrapped(err), render))
		})
	}
}

func TestLoginRefusals(t *testing.T) {
	t.Parallel()
	assertAnswers(t, func(w http.ResponseWriter, r *http.Request, err error) { Render(w, r, err, LoginRules) }, map[error]answer{
		// One answer for an unknown address, a wrong password and a
		// membership at another school: no address oracle.
		identityaccess.ErrInvalidCredentials:        {http.StatusUnauthorized, "identity.invalid_credentials"},
		identityaccess.ErrAccountNotFound:           {http.StatusUnauthorized, "identity.invalid_credentials"},
		identityaccess.ErrTenantAccessDenied:        {http.StatusUnauthorized, "identity.invalid_credentials"},
		identityaccess.ErrAccountInactive:           {http.StatusUnauthorized, "identity.session_account_inactive"},
		identityaccess.ErrTenantNotFound:            {http.StatusNotFound, "general.input"},
		identityaccess.ErrParentMustUseParentPortal: {http.StatusForbidden, "identity.use_parent_portal"},
		identityaccess.ErrMustUseSchoolPortal:       {http.StatusForbidden, "identity.use_school_portal"},
		identityaccess.ErrAccountNoSchoolPortalRole: {http.StatusForbidden, "school.no_school_portal_role"},
		identityaccess.ErrAccountNoGuardianRole:     {http.StatusForbidden, "care.not_a_guardian"},
		identityaccess.ErrNoTraegerMembership:       {http.StatusForbidden, "traeger.no_membership"},
		identityaccess.ErrMFARateLimited:            {http.StatusTooManyRequests, "identity.mfa_blocked"},
		identityaccess.ErrMFALocked:                 {http.StatusTooManyRequests, "identity.mfa_blocked"},
		identityaccess.ErrMFAStatusUnavailable:      {http.StatusServiceUnavailable, "identity.mfa_status_unavailable"},
	})
}

func TestLoginRefusalOfAMissingModuleNamesTheModule(t *testing.T) {
	t.Parallel()
	err := wrapped(&identityaccess.PortalNotBookedError{Module: "eltern_app", ModuleLabel: "Eltern-App"})
	assert.Equal(t, answer{http.StatusForbidden, "module.not_booked"}, rendered(t, err, func(w http.ResponseWriter, r *http.Request, err error) { Render(w, r, err, LoginRules) }))
}

func TestExchangeRefusals(t *testing.T) {
	t.Parallel()
	assertAnswers(t, func(w http.ResponseWriter, r *http.Request, err error) { Render(w, r, err, ExchangeRules) }, map[error]answer{
		// A proof an MFA reset revoked answers like a wrong code.
		identityaccess.ErrInvalidToken:              {http.StatusUnauthorized, "identity.mfa_code_invalid"},
		identityaccess.ErrAccountNotFound:           {http.StatusUnauthorized, "identity.invalid_credentials"},
		identityaccess.ErrAccountInactive:           {http.StatusUnauthorized, "identity.session_account_inactive"},
		identityaccess.ErrTenantNotFound:            {http.StatusNotFound, "general.input"},
		identityaccess.ErrTenantAccessDenied:        {http.StatusUnauthorized, "identity.invalid_credentials"},
		identityaccess.ErrParentMustUseParentPortal: {http.StatusForbidden, "identity.use_parent_portal"},
		identityaccess.ErrMustUseSchoolPortal:       {http.StatusForbidden, "identity.use_school_portal"},
		identityaccess.ErrAccountNoSchoolPortalRole: {http.StatusForbidden, "school.no_school_portal_role"},
		identityaccess.ErrNoTraegerMembership:       {http.StatusForbidden, "traeger.no_membership"},
	})
}

func TestMFARefusals(t *testing.T) {
	t.Parallel()
	assertAnswers(t, func(w http.ResponseWriter, r *http.Request, err error) { Render(w, r, err, MFARules) }, map[error]answer{
		// A wrong code, an unusable challenge and another portal's
		// challenge answer alike.
		identityaccess.ErrMFAChallengeTokenInvalid: {http.StatusUnauthorized, "identity.mfa_code_invalid"},
		identityaccess.ErrMFACodeInvalid:           {http.StatusUnauthorized, "identity.mfa_code_invalid"},
		identityaccess.ErrMFAUnsupportedScope:      {http.StatusUnauthorized, "identity.mfa_code_invalid"},
		identityaccess.ErrMFALocked:                {http.StatusTooManyRequests, "identity.mfa_blocked"},
		identityaccess.ErrMFARateLimited:           {http.StatusTooManyRequests, "identity.mfa_blocked"},
		identityaccess.ErrMFAStatusUnavailable:     {http.StatusServiceUnavailable, "identity.mfa_status_unavailable"},
		identityaccess.ErrMFANotEnrolled:           {http.StatusForbidden, "general.permission"},
		identityaccess.ErrMFAAlreadyEnrolled:       {http.StatusConflict, "general.business_rejection"},
	})
}

func TestSwitchRefusals(t *testing.T) {
	t.Parallel()
	assertAnswers(t, func(w http.ResponseWriter, r *http.Request, err error) { Render(w, r, err, SwitchRules) }, map[error]answer{
		identityaccess.ErrInvalidToken:              {http.StatusUnauthorized, "identity.mfa_session_required"},
		identityaccess.ErrMFAStatusUnavailable:      {http.StatusServiceUnavailable, "identity.mfa_status_unavailable"},
		identityaccess.ErrAccountNotFound:           {http.StatusUnauthorized, "identity.account_not_found"},
		identityaccess.ErrAccountInactive:           {http.StatusUnauthorized, "identity.session_account_inactive"},
		identityaccess.ErrTenantNotFound:            {http.StatusNotFound, "general.input"},
		identityaccess.ErrTenantAccessDenied:        {http.StatusUnauthorized, "identity.tenant_access_denied"},
		identityaccess.ErrMustUseSchoolPortal:       {http.StatusForbidden, "identity.use_school_portal"},
		identityaccess.ErrAccountNoSchoolPortalRole: {http.StatusForbidden, "school.no_school_portal_role"},
		identityaccess.ErrDemoSessionTenantLocked:   {http.StatusForbidden, "identity.demo_session"},
	})
}

func TestRefreshRefusalsEndTheSession(t *testing.T) {
	t.Parallel()
	cases := map[error]answer{identityaccess.ErrMFAStatusUnavailable: {http.StatusServiceUnavailable, "identity.mfa_status_unavailable"}}
	for _, err := range []error{
		identityaccess.ErrInvalidToken, identityaccess.ErrTokenExpired, identityaccess.ErrTokenNotFound,
		identityaccess.ErrAccountNotFound, identityaccess.ErrAccountInactive, identityaccess.ErrTenantNotFound,
		identityaccess.ErrTenantAccessDenied, identityaccess.ErrNoTraegerMembership, identityaccess.ErrMustUseSchoolPortal,
	} {
		cases[err] = answer{http.StatusUnauthorized, "general.permission"}
	}
	assertAnswers(t, func(w http.ResponseWriter, r *http.Request, err error) { Render(w, r, err, RefreshRules) }, cases)
}

func TestPasskeyLoginRefusalsShareOneCode(t *testing.T) {
	t.Parallel()
	cases := map[error]answer{}
	for _, err := range []error{
		identityaccess.ErrInvalidCredentials, identityaccess.ErrPasskeySessionInvalid, identityaccess.ErrAccountNotFound,
		identityaccess.ErrTenantAccessDenied, identityaccess.ErrPasskeyOriginInvalid,
	} {
		cases[err] = answer{http.StatusUnauthorized, "identity.passkey_login_failed"}
	}
	assertAnswers(t, func(w http.ResponseWriter, r *http.Request, err error) {
		Render(w, r, err, PasskeyLoginRules, PasskeyRules)
	}, cases)
}

func TestUnclassifiedErrorsAreServerErrors(t *testing.T) {
	t.Parallel()
	err := wrapped(errAssertionUnknown)
	for _, table := range []renderer{
		func(w http.ResponseWriter, r *http.Request, err error) { Render(w, r, err, LoginRules) },
		func(w http.ResponseWriter, r *http.Request, err error) { Render(w, r, err, ExchangeRules) },
		func(w http.ResponseWriter, r *http.Request, err error) { Render(w, r, err, MFARules) },
		func(w http.ResponseWriter, r *http.Request, err error) { Render(w, r, err, SwitchRules) },
		func(w http.ResponseWriter, r *http.Request, err error) { Render(w, r, err, RefreshRules) },
	} {
		assert.Equal(t, answer{http.StatusInternalServerError, "general.server"}, rendered(t, err, table))
	}
}

var errAssertionUnknown = errors.New("store unavailable")
