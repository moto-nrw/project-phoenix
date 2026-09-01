package worktimemodels

import (
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/stretchr/testify/require"
)

func init() { testutil.SeedTestJWTConfig() }

func TestRouter_RejectsUsersRead(t *testing.T) {
	t.Parallel()
	resource := &Resource{}
	router := resource.Router()
	claims := testutil.DefaultTestClaims()
	claims.Roles = []string{"user"}
	claims.Permissions = []string{"users:read"}
	claims.IsAdmin = false
	token := testutil.MintTestJWT(t, claims)

	for _, tc := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/"},
		{method: http.MethodGet, path: "/123"},
		{method: http.MethodPost, path: "/"},
		{method: http.MethodPut, path: "/123"},
		{method: http.MethodDelete, path: "/123"},
	} {
		req := testutil.NewAuthenticatedRequest(t, tc.method, tc.path, nil, testutil.WithJWTBearer(token))
		rr := testutil.ExecuteRequest(router, req)

		require.Equal(t, http.StatusForbidden, rr.Code)
	}
}
