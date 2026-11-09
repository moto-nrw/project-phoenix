package catererauth

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

// The caterer's login and code forms show a text per code (#4497); these
// tests pin the status and code of each refusal the caterer flows return.
// They are the status and code the caterer portal answered with before it
// shared the tables (#6172); the detail is diagnostics and follows the
// shared tables.

type answer struct {
	status int
	code   string
	detail string
}

type renderer func(w http.ResponseWriter, r *http.Request, err error)

func rendered(t *testing.T, err error, render renderer) answer {
	t.Helper()
	rr := httptest.NewRecorder()
	render(rr, httptest.NewRequest(http.MethodPost, "/", nil), err)
	var body struct {
		Code   string `json:"code"`
		Detail string `json:"detail"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body), rr.Body.String())
	return answer{rr.Code, body.Code, body.Detail}
}

// assertAnswers compares status and code, and the detail where the case
// names one.
func assertAnswers(t *testing.T, render renderer, cases map[error]answer) {
	t.Helper()
	for err, want := range cases {
		t.Run(err.Error(), func(t *testing.T) {
			t.Parallel()
			got := rendered(t, &identityaccess.AuthenticationError{Op: "caterer login", Err: err}, render)
			if want.detail == "" {
				got.detail = ""
			}
			assert.Equal(t, want, got)
		})
	}
}

var (
	invalidCredentials = answer{status: http.StatusUnauthorized, code: "identity.invalid_credentials"}
	mfaCodeInvalid     = answer{status: http.StatusUnauthorized, code: "identity.mfa_code_invalid"}
	mfaBlocked         = answer{status: http.StatusTooManyRequests, code: "identity.mfa_blocked"}
	passkeyLoginFailed = answer{status: http.StatusUnauthorized, code: "identity.passkey_login_failed"}
)

func TestSignInAndSecondFactorRefusals(t *testing.T) {
	t.Parallel()
	assertAnswers(t, renderFailure, map[error]answer{
		// An unknown address, a wrong password and an account that is no
		// caterer's answer alike: no address oracle.
		identityaccess.ErrInvalidCredentials: invalidCredentials,
		identityaccess.ErrAccountNotFound:    invalidCredentials,
		identityaccess.ErrAccountNotCaterer:  invalidCredentials,
		identityaccess.ErrAccountInactive:    {status: http.StatusUnauthorized, code: "identity.session_account_inactive"},
		identityaccess.ErrCatererWithoutSchool: {
			status: http.StatusForbidden, code: "identity.caterer_without_school", detail: identityaccess.ErrCatererWithoutSchool.Error(),
		},
		// A wrong code, an unusable challenge, another portal's challenge
		// and a proof an operator's reset revoked answer alike.
		identityaccess.ErrMFACodeInvalid:           mfaCodeInvalid,
		identityaccess.ErrMFAChallengeTokenInvalid: mfaCodeInvalid,
		identityaccess.ErrMFAUnsupportedScope:      mfaCodeInvalid,
		identityaccess.ErrInvalidToken:             mfaCodeInvalid,
		identityaccess.ErrMFALocked:                mfaBlocked,
		identityaccess.ErrMFARateLimited:           mfaBlocked,
		identityaccess.ErrMFAStatusUnavailable:     {status: http.StatusServiceUnavailable, code: "identity.mfa_status_unavailable"},
		identityaccess.ErrAccountMFAUnavailable:    {status: http.StatusServiceUnavailable, code: "general.unavailable"},
		identityaccess.ErrPasskeyFlowsUnavailable:  {status: http.StatusServiceUnavailable, code: "general.unavailable"},
		identityaccess.ErrMFAAlreadyEnrolled:       {status: http.StatusConflict, code: "general.business_rejection"},
		identityaccess.ErrCatererMFANotEnabled:     {status: http.StatusConflict, code: "general.business_rejection", detail: identityaccess.ErrCatererMFANotEnabled.Error()},
		identityaccess.ErrMFAPermissionDenied:      {status: http.StatusForbidden, code: "general.permission"},
		identityaccess.ErrPasskeyNotFound:          {status: http.StatusNotFound, code: "general.input"},
		identityaccess.ErrPasskeySessionInvalid:    {status: http.StatusUnauthorized, code: "general.permission"},
		identityaccess.ErrPasskeyOriginInvalid:     {status: http.StatusUnauthorized, code: "general.permission"},
	})
}

func TestPasskeyLoginRefusalsShareOneCode(t *testing.T) {
	t.Parallel()
	cases := map[error]answer{
		identityaccess.ErrAccountInactive:      {status: http.StatusUnauthorized, code: "identity.session_account_inactive"},
		identityaccess.ErrCatererWithoutSchool: {status: http.StatusForbidden, code: "identity.caterer_without_school"},
		identityaccess.ErrMFALocked:            mfaBlocked,
		identityaccess.ErrMFAStatusUnavailable: {status: http.StatusServiceUnavailable, code: "identity.mfa_status_unavailable"},
		identityaccess.ErrPasskeyFlowsUnavailable: {
			status: http.StatusServiceUnavailable, code: "general.unavailable",
		},
	}
	for _, err := range []error{
		identityaccess.ErrInvalidCredentials, identityaccess.ErrPasskeySessionInvalid, identityaccess.ErrAccountNotFound,
		identityaccess.ErrAccountNotCaterer, identityaccess.ErrPasskeyOriginInvalid,
	} {
		cases[err] = passkeyLoginFailed
	}
	assertAnswers(t, renderPasskeyLoginFailure, cases)
}

// A refusal of an account of another portal names the same refusal in its
// detail as the wrong password does, so the body tells nothing either.
func TestNoCatererAccountReadsLikeAWrongPassword(t *testing.T) {
	t.Parallel()
	for _, render := range []renderer{renderFailure, renderPasskeyLoginFailure} {
		assert.Equal(t,
			rendered(t, identityaccess.ErrInvalidCredentials, render),
			rendered(t, identityaccess.ErrAccountNotCaterer, render))
	}
}

func TestUnclassifiedErrorsKeepTheirCauseOutOfTheBody(t *testing.T) {
	t.Parallel()
	err := errors.New("store unavailable at 10.0.0.7")
	assert.Equal(t, answer{http.StatusInternalServerError, "general.server", "caterer second factor failed"}, rendered(t, err, renderFailure))
	assert.Equal(t, answer{http.StatusInternalServerError, "general.server", "caterer passkey sign-in failed"}, rendered(t, err, renderPasskeyLoginFailure))
}
