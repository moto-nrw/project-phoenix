package account_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rootAPI "github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// setupHolidayParentSetupRouter mounts the real auth Router() with the setup
// link of a parent account at a Träger wired (ADR 0216).
func setupHolidayParentSetupRouter(t *testing.T) (chi.Router, identityaccess.HolidayParentSetups, int64) {
	t.Helper()
	db, resource := setupAuthDependenciesRoute(t)
	setups, ok := resource.Sessions.(identityaccess.HolidayParentSetups)
	require.True(t, ok, "the composed owner serves the setup link")
	resource.HolidayParentSetups = setups
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Träger Ferienkonto", Slug: fmt.Sprintf("ferienkonto-%d", testpkg.UniqueTestTenantID(t)), Active: true,
	})
	router := testutil.NewTenantRouter(db)
	router.Mount("/auth", resource.Router(rootAPI.TenantSessionChain))
	t.Cleanup(func() {
		var accountIDs []int64
		require.NoError(t, db.NewRaw(`SELECT account_id FROM auth.traeger_parent_access WHERE organization_id = ?`, organization.ID).Scan(context.Background(), &accountIDs))
		for _, id := range accountIDs {
			testpkg.EnsureAccountTenant(t, db, id, testpkg.Tenant(t))
		}
	})
	return router, setups, organization.ID
}

func TestHolidayParentSetupRoutes(t *testing.T) {
	t.Parallel()
	router, setups, traeger := setupHolidayParentSetupRouter(t)
	email := fmt.Sprintf("ferienkonto-%d@test.local", testpkg.UniqueTestTenantID(t))
	token, err := setups.IssueHolidayParentSetupToken(email, traeger)
	require.NoError(t, err)
	password := testpkg.NewTestPassword()

	rr := testutil.ExecuteRequest(router, testutil.NewJSONRequest(t, http.MethodGet, "/auth/holiday-parent-setups/"+token, nil))
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	data := testutil.ParseJSONResponse(t, rr.Body.Bytes())["data"].(map[string]any)
	assert.Equal(t, map[string]any{"email": email, "traeger_name": "Träger Ferienkonto", "account_exists": false}, data)

	rr = testutil.ExecuteRequest(router, testutil.NewJSONRequest(t, http.MethodPost, "/auth/holiday-parent-setups/"+token, map[string]any{"password": "kurz", "confirm_password": "kurz"}))
	testutil.AssertBadRequest(t, rr)
	assert.Equal(t, "identity.password_too_weak", testutil.ParseJSONResponse(t, rr.Body.Bytes())["code"])
	rr = testutil.ExecuteRequest(router, testutil.NewJSONRequest(t, http.MethodPost, "/auth/holiday-parent-setups/"+token, map[string]any{"password": password, "confirm_password": password + "x"}))
	testutil.AssertBadRequest(t, rr)

	rr = testutil.ExecuteRequest(router, testutil.NewJSONRequest(t, http.MethodPost, "/auth/holiday-parent-setups/"+token, map[string]any{"password": password, "confirm_password": password}))
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	assert.Equal(t, map[string]any{"account_exists": false}, testutil.ParseJSONResponse(t, rr.Body.Bytes())["data"])

	rr = testutil.ExecuteRequest(router, testutil.NewJSONRequest(t, http.MethodPost, "/auth/holiday-parent-setups/"+token, map[string]any{}))
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	assert.Equal(t, map[string]any{"account_exists": true}, testutil.ParseJSONResponse(t, rr.Body.Bytes())["data"], "an existing account needs no password")
}

func TestHolidayParentSetupRoutesRefuseAnInvalidLink(t *testing.T) {
	t.Parallel()
	router, _, _ := setupHolidayParentSetupRouter(t)
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		rr := testutil.ExecuteRequest(router, testutil.NewJSONRequest(t, method, "/auth/holiday-parent-setups/not-a-token", map[string]any{}))
		testutil.AssertNotFound(t, rr)
		assert.Equal(t, "identity.holiday_parent_setup_invalid", testutil.ParseJSONResponse(t, rr.Body.Bytes())["code"], method)
	}
}

// The page sends an empty body when the address already has an account. For
// a new address that body is no password: the route refuses it and writes
// nothing.
func TestHolidayParentSetupRoutesNeedAPasswordForANewAccount(t *testing.T) {
	t.Parallel()
	router, setups, traeger := setupHolidayParentSetupRouter(t)
	email := fmt.Sprintf("ferienkonto-leer-%d@test.local", testpkg.UniqueTestTenantID(t))
	token, err := setups.IssueHolidayParentSetupToken(email, traeger)
	require.NoError(t, err)

	rr := testutil.ExecuteRequest(router, testutil.NewJSONRequest(t, http.MethodPost, "/auth/holiday-parent-setups/"+token, map[string]any{}))

	testutil.AssertBadRequest(t, rr)
	assert.Equal(t, "identity.password_too_weak", testutil.ParseJSONResponse(t, rr.Body.Bytes())["code"])
	rr = testutil.ExecuteRequest(router, testutil.NewJSONRequest(t, http.MethodGet, "/auth/holiday-parent-setups/"+token, nil))
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	assert.Equal(t, false, testutil.ParseJSONResponse(t, rr.Body.Bytes())["data"].(map[string]any)["account_exists"], "no account was created")
}
