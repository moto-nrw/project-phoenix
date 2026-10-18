package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	substitutionsHTTP "github.com/moto-nrw/project-phoenix/modules/workforce/http/substitutions"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// The substitution contract renders status, code and message the adapter
// classified and never the underlying cause; the shared envelope is what the
// frontend parses, so it is asserted byte for byte here.
func TestRenderSubstitutionsFailureUsesSharedEnvelope(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/substitutions", nil)
	renderSubstitutionsFailure(recorder, request, substitutionsHTTP.Failure{
		Status: http.StatusConflict, Code: "substitutions.already_assigned", Message: "Diese Gruppenübergabe besteht bereits.",
		Err: errors.New("postgres password leaked"),
	})

	require.Equal(t, http.StatusConflict, recorder.Code)
	var body struct {
		Status int    `json:"status"`
		Detail string `json:"detail"`
		Code   string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, http.StatusConflict, body.Status)
	require.Equal(t, "Diese Gruppenübergabe besteht bereits.", body.Detail)
	require.Equal(t, "substitutions.already_assigned", body.Code)
	require.NotContains(t, recorder.Body.String(), "postgres")
}

// Without an authenticated principal in the request there is no caller, and
// the operation is forbidden rather than attributed to nobody.
func TestSubstitutionCallerRequiresPrincipal(t *testing.T) {
	t.Parallel()

	_, err := substitutionsHTTP.RequestCaller(testpkg.TenantContext(testpkg.Tenant(t)))
	require.ErrorIs(t, err, workforce.ErrSubstitutionForbidden)
	_, err = substitutionsHTTP.RequestCaller(context.Background())
	require.ErrorIs(t, err, workforce.ErrSubstitutionForbidden)
}
