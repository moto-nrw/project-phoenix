package parent

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A successful sign-in hands the address to the workflow, which claims the
// holiday registrations of that address (ADR 0216 point 3); a refused one
// claims nothing.
func TestParentLoginClaimsHolidayRegistrationsOnlyAfterSuccess(t *testing.T) {
	t.Parallel()
	refuse := false
	portal := &fakeParentService{}
	rs := &Resource{ParentService: portal, Auth: LoginRuntime{
		Login: func(context.Context, string, string, string, string) (string, string, error) {
			if refuse {
				return "", "", errors.New("stub: refused")
			}
			return "access", "refresh", nil
		},
	}}
	login := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/parent/auth/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		rs.login(rr, req)
		return rr
	}

	rr := login(`{"email": "  Eva.Muster@Example.org ", "password": "example-test-password"}`)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Equal(t, []string{"eva.muster@example.org"}, portal.claimedEmails)

	refuse = true
	rr = login(`{"email": "eva.muster@example.org", "password": "example-wrong-password"}`)
	require.NotEqual(t, http.StatusOK, rr.Code)
	assert.Len(t, portal.claimedEmails, 1, "a refused sign-in claims nothing")
}
