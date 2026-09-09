package shiftplanning

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The shift-type route is composed here, so the permission gate, the
// Kategorie↔Schichtart sync and the rollback of a rejected mapping are
// asserted against the real runtime, the real services and the database.

type shiftTypeRoute struct {
	db     *bun.DB
	router chi.Router
	token  string
	ctx    context.Context
	module services.ShiftTypeTestModule
}

func setupShiftTypeRoute(t *testing.T) *shiftTypeRoute {
	t.Helper()
	db, module := testutil.SetupShiftTypeModule(t)
	resource := NewShiftTypesResource(NewShiftTypeAdministration(module.ShiftTypes, module.Activities.SetCategoryShiftTypeLinks), db)

	claims := testutil.DefaultTestClaims()
	claims.TenantID = testpkg.Tenant(t)
	claims.Permissions = []string{permissions.TimeTrackingManage}
	claims.IsAdmin = false
	return &shiftTypeRoute{db: db, router: resource.Router(), token: testutil.MintTestJWT(t, claims), ctx: testpkg.Ctx(t), module: module}
}

func (s *shiftTypeRoute) do(t *testing.T, method, path string, body any) *http.Response {
	t.Helper()
	request := testutil.NewAuthenticatedRequest(t, method, path, body, testutil.WithJWTBearer(s.token))
	return testutil.ExecuteRequest(s.router, request).Result()
}

func (s *shiftTypeRoute) shiftTypeIDOf(t *testing.T, categoryID int64) *int64 {
	t.Helper()
	category, err := s.module.Repositories.Categories.FindByID(s.ctx, categoryID)
	require.NoError(t, err)
	return category.ShiftTypeID
}

func decodeShiftTypeID(t *testing.T, response *http.Response) int64 {
	t.Helper()
	defer func() { _ = response.Body.Close() }()
	var envelope struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&envelope))
	require.NotZero(t, envelope.Data.ID)
	return envelope.Data.ID
}

func TestShiftTypesRouteCreatesWithCategoryLinks(t *testing.T) {
	t.Parallel()
	route := setupShiftTypeRoute(t)

	first := testpkg.CreateTestActivityCategory(t, route.db, "st-create-1")
	second := testpkg.CreateTestActivityCategory(t, route.db, "st-create-2")

	created := route.do(t, http.MethodPost, "/", map[string]any{
		"name": fmt.Sprintf("Betreuung-%d", time.Now().UnixNano()), "color": "#83CD2D", "category_ids": []int64{first.ID, second.ID},
	})
	require.Equal(t, http.StatusCreated, created.StatusCode)
	typeID := decodeShiftTypeID(t, created)

	require.NotNil(t, route.shiftTypeIDOf(t, first.ID))
	assert.Equal(t, typeID, *route.shiftTypeIDOf(t, first.ID))
	require.NotNil(t, route.shiftTypeIDOf(t, second.ID))
	assert.Equal(t, typeID, *route.shiftTypeIDOf(t, second.ID))
}

func TestShiftTypesRouteUpdateSyncsAndOmittedLeavesUntouched(t *testing.T) {
	t.Parallel()
	route := setupShiftTypeRoute(t)

	first := testpkg.CreateTestActivityCategory(t, route.db, "st-upd-1")
	second := testpkg.CreateTestActivityCategory(t, route.db, "st-upd-2")

	name := fmt.Sprintf("Vorbereitung-%d", time.Now().UnixNano())
	created := route.do(t, http.MethodPost, "/", map[string]any{
		"name": name, "color": "#5080D8", "category_ids": []int64{first.ID, second.ID},
	})
	require.Equal(t, http.StatusCreated, created.StatusCode)
	typeID := decodeShiftTypeID(t, created)

	// Narrowing the set to the first category unlinks the second.
	updated := route.do(t, http.MethodPut, fmt.Sprintf("/%d", typeID), map[string]any{
		"name": name, "color": "#5080D8", "is_active": true, "category_ids": []int64{first.ID},
	})
	require.Equal(t, http.StatusOK, updated.StatusCode)
	require.NotNil(t, route.shiftTypeIDOf(t, first.ID))
	assert.Equal(t, typeID, *route.shiftTypeIDOf(t, first.ID))
	assert.Nil(t, route.shiftTypeIDOf(t, second.ID), "the de-selected category is unlinked on update")

	// An update without category_ids leaves the mapping untouched, and an
	// omitted is_active keeps the stored flag.
	untouched := route.do(t, http.MethodPut, fmt.Sprintf("/%d", typeID), map[string]any{"name": name, "color": "#5080D8"})
	require.Equal(t, http.StatusOK, untouched.StatusCode)
	require.NotNil(t, route.shiftTypeIDOf(t, first.ID), "omitted category_ids must not clear the mapping")
	var envelope struct {
		Data struct {
			IsActive bool `json:"is_active"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(untouched.Body).Decode(&envelope))
	assert.True(t, envelope.Data.IsActive)
}

func TestShiftTypesRouteRejectsUnknownCategoryIDsAndRollsBack(t *testing.T) {
	t.Parallel()
	route := setupShiftTypeRoute(t)

	first := testpkg.CreateTestActivityCategory(t, route.db, "st-unknown-1")
	name := fmt.Sprintf("Betreuung-%d", time.Now().UnixNano())
	created := route.do(t, http.MethodPost, "/", map[string]any{
		"name": name, "color": "#83CD2D", "category_ids": []int64{first.ID},
	})
	require.Equal(t, http.StatusCreated, created.StatusCode)
	typeID := decodeShiftTypeID(t, created)
	require.NotNil(t, route.shiftTypeIDOf(t, first.ID))

	// An update carrying an unknown category id is a 400 and must NOT clear
	// the existing mapping nor persist the renamed type.
	rejected := route.do(t, http.MethodPut, fmt.Sprintf("/%d", typeID), map[string]any{
		"name": name + " neu", "color": "#83CD2D", "is_active": true, "category_ids": []int64{first.ID, 999999999},
	})
	require.Equal(t, http.StatusBadRequest, rejected.StatusCode)
	require.NotNil(t, route.shiftTypeIDOf(t, first.ID), "existing mapping must survive a rejected update")
	assert.Equal(t, typeID, *route.shiftTypeIDOf(t, first.ID))
	listed := route.do(t, http.MethodGet, "/", nil)
	require.Equal(t, http.StatusOK, listed.StatusCode)
	var envelope struct {
		Data []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(listed.Body).Decode(&envelope))
	for _, entry := range envelope.Data {
		if entry.ID == typeID {
			assert.Equal(t, name, entry.Name, "the rejected rename must roll back with the mapping")
		}
	}

	// A duplicate name is a conflict; an unknown id is not found.
	duplicate := route.do(t, http.MethodPost, "/", map[string]any{"name": name, "color": "#000000"})
	assert.Equal(t, http.StatusConflict, duplicate.StatusCode)
	missing := route.do(t, http.MethodPut, "/999999999", map[string]any{"name": "Neu", "color": "#000000"})
	assert.Equal(t, http.StatusNotFound, missing.StatusCode)
}

func TestShiftTypesRouteRequiresManagePermission(t *testing.T) {
	t.Parallel()
	route := setupShiftTypeRoute(t)

	claims := testutil.DefaultTestClaims()
	claims.TenantID = testpkg.Tenant(t)
	claims.Permissions = []string{permissions.TimeTrackingOwn}
	claims.IsAdmin = false
	token := testutil.MintTestJWT(t, claims)
	for _, test := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/"},
		{method: http.MethodPost, path: "/"},
		{method: http.MethodPost, path: "/defaults"},
		{method: http.MethodPut, path: "/1"},
		{method: http.MethodDelete, path: "/1"},
	} {
		request := testutil.NewAuthenticatedRequest(t, test.method, test.path, map[string]any{"name": "Neu"}, testutil.WithJWTBearer(token))
		assert.Equal(t, http.StatusForbidden, testutil.ExecuteRequest(route.router, request).Code, test.method+" "+test.path)
	}
}
