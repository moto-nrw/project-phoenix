package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	parentHTTP "github.com/moto-ogs/project-phoenix/workflows/parentportal/http"
)

// The parents login page branches on these codes, so each refusal of the
// owner keeps its answer.
func TestParentLoginRefusals(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		cause  error
		status int
		code   string
	}{
		{"wrong password", identityaccess.ErrInvalidCredentials, http.StatusUnauthorized, "care.invalid_credentials"},
		// An unknown address answers like a wrong password; a distinct
		// answer would tell a caller which addresses exist.
		{"unknown address", identityaccess.ErrAccountNotFound, http.StatusUnauthorized, "care.invalid_credentials"},
		// The page asks a deactivated guardian to contact the school.
		{"account switched off", identityaccess.ErrAccountInactive, http.StatusUnauthorized, "identity.session_account_inactive"},
		// A staff account at the parents login; the page masks it but keeps
		// the hint to the staff login.
		{"no guardian", identityaccess.ErrAccountNoGuardianRole, http.StatusForbidden, "care.not_a_guardian"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rr := postParentLogin(t, &identityaccess.AuthenticationError{Op: "login", Err: tc.cause})
			assert.Equal(t, tc.status, rr.Code)
			assert.Equal(t, tc.code, parentLoginRefusal(t, rr).Code)
		})
	}
}

// None of the guardian's schools booked Eltern-App (#4895): the page explains
// the missing module.
func TestParentLoginWithoutTheModuleNamesTheModule(t *testing.T) {
	t.Parallel()

	rr := postParentLogin(t, &identityaccess.PortalNotBookedError{Module: "eltern_app", ModuleLabel: "Eltern-App"})
	assert.Equal(t, http.StatusForbidden, rr.Code)
	body := parentLoginRefusal(t, rr)
	assert.Equal(t, "module.not_booked", body.Code)
	assert.Equal(t, "eltern_app", body.Details["module"])
	assert.Equal(t, "Eltern-App", body.Details["module_label"])
}

type parentRefusalBody struct {
	Code    string            `json:"code"`
	Details map[string]string `json:"details"`
}

func parentLoginRefusal(t *testing.T, rr *httptest.ResponseRecorder) parentRefusalBody {
	t.Helper()
	var body parentRefusalBody
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body), rr.Body.String())
	return body
}

func postParentLogin(t *testing.T, refusal error) *httptest.ResponseRecorder {
	t.Helper()
	resource := parentHTTP.NewResource(parentHTTP.ResourceConfig{Auth: parentHTTP.LoginRuntime{
		Login: func(context.Context, string, string, string, string) (string, string, error) {
			return "", "", refusal
		},
	}})
	body, err := json.Marshal(map[string]string{"email": "parent@example.com", "password": "secret"})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	resource.Router(ParentSessionChain).ServeHTTP(rr, req)
	return rr
}
