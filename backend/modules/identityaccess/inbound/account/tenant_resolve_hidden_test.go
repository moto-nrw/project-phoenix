package account_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	authAPI "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/account"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestResolveTenant_HiddenSchoolReturnsHiddenFlag(t *testing.T) {
	t.Parallel()

	db, authRoute := setupAuthDependenciesRoute(t)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	_, err := db.ExecContext(context.Background(),
		`UPDATE platform.schools SET hidden = true WHERE id = ?`, tenantID)
	require.NoError(t, err)

	resource := authAPI.NewResource(authRoute.AuthService, authRoute.Invitations, authRoute.SchoolService, authRoute.Sessions, testutil.AccountRouteTenantRuntime())
	resource.SettingsService = authRoute.SettingsService

	router := chi.NewRouter()
	router.Mount("/auth", resource.Router())

	req := httptest.NewRequest("GET",
		fmt.Sprintf("/auth/tenant/resolve?slug=t%d", tenantID), nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var resp struct {
		Data struct {
			Hidden bool `json:"hidden"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.True(t, resp.Data.Hidden)
}
