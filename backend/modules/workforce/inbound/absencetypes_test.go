package inbound

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The absence-type route is composed here, so the permission gate, the actor
// lookup and the error envelope are asserted against the real runtime.
func TestAbsenceTypesRouteManagesCustomTypeAllowance(t *testing.T) {
	t.Parallel()

	db, router := setupAbsenceTypesRoute(t)
	admin, account := testpkg.CreateTestStaffWithAccount(t, db, "Lea", "Leitung")
	staff := testpkg.CreateTestStaff(t, db, "Rena", "Generation")

	claims := testutil.DefaultTestClaims()
	claims.ID = int(account.ID)
	claims.TenantID = testpkg.Tenant(t)
	claims.Permissions = []string{permissions.TimeTrackingManage}
	claims.IsAdmin = false
	token := testutil.MintTestJWT(t, claims)
	create := testutil.NewAuthenticatedRequest(t, "POST", "/", map[string]any{
		"name": "Regenerationstag", "allowance_enabled": true, "overrun_policy": "block",
	}, testutil.WithJWTBearer(token))
	created := testutil.ExecuteRequest(router, create)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	var typeEnvelope struct {
		Data struct {
			ID               string `json:"id"`
			AllowanceEnabled bool   `json:"allowance_enabled"`
			OverrunPolicy    string `json:"overrun_policy"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &typeEnvelope))
	assert.True(t, typeEnvelope.Data.AllowanceEnabled)
	assert.Equal(t, "block", typeEnvelope.Data.OverrunPolicy)

	// A second art with the same name is a conflict that keeps the German
	// wording of the retained service on the wire.
	duplicate := testutil.NewAuthenticatedRequest(t, "POST", "/", map[string]any{"name": "regenerationstag"}, testutil.WithJWTBearer(token))
	conflict := testutil.ExecuteRequest(router, duplicate)
	require.Equal(t, http.StatusConflict, conflict.Code, conflict.Body.String())
	var conflictEnvelope struct {
		Error string `json:"error"`
	}
	require.NoError(t, json.Unmarshal(conflict.Body.Bytes(), &conflictEnvelope))
	assert.Equal(t, "eine Abwesenheitsart mit diesem Namen gibt es bereits", conflictEnvelope.Error)

	path := fmt.Sprintf("/%s/allowances/%d", typeEnvelope.Data.ID, staff.ID)
	ownClaims := claims
	ownClaims.Permissions = []string{permissions.TimeTrackingOwn}
	ownToken := testutil.MintTestJWT(t, ownClaims)
	denied := testutil.NewAuthenticatedRequest(t, "GET", path+"?year=2026", nil, testutil.WithJWTBearer(ownToken))
	assert.Equal(t, http.StatusForbidden, testutil.ExecuteRequest(router, denied).Code)

	set := testutil.NewAuthenticatedRequest(t, "PUT", path, map[string]any{
		"year": 2026, "entitled_days": 2.5, "reason": "Tariflicher Anspruch",
	}, testutil.WithJWTBearer(token))
	saved := testutil.ExecuteRequest(router, set)
	require.Equal(t, http.StatusOK, saved.Code, saved.Body.String())
	var allowanceEnvelope struct {
		Data struct {
			StaffID       string  `json:"staff_id"`
			EntitledDays  float64 `json:"entitled_days"`
			RemainingDays float64 `json:"remaining_days"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(saved.Body.Bytes(), &allowanceEnvelope))
	assert.Equal(t, fmt.Sprint(staff.ID), allowanceEnvelope.Data.StaffID)
	assert.Equal(t, 2.5, allowanceEnvelope.Data.EntitledDays)
	assert.Equal(t, 2.5, allowanceEnvelope.Data.RemainingDays)
	assert.NotEqual(t, staff.ID, admin.ID)

	unknown := testutil.NewAuthenticatedRequest(t, "PUT", "/999999999", map[string]any{"name": "Neu"}, testutil.WithJWTBearer(token))
	assert.Equal(t, http.StatusNotFound, testutil.ExecuteRequest(router, unknown).Code)
}

// Reading is open to time_tracking:own so the Leitung's approval views can
// render the school's wording; writing stays with time_tracking:manage.
func TestAbsenceTypesRouteRejectsWritesWithoutManage(t *testing.T) {
	t.Parallel()

	db, router := setupAbsenceTypesRoute(t)
	_, account := testpkg.CreateTestStaffWithAccount(t, db, "Ohne", "Verwaltung")

	claims := testutil.DefaultTestClaims()
	claims.ID = int(account.ID)
	claims.TenantID = testpkg.Tenant(t)
	claims.Permissions = []string{permissions.TimeTrackingOwn}
	claims.IsAdmin = false
	token := testutil.MintTestJWT(t, claims)

	list := testutil.NewAuthenticatedRequest(t, "GET", "/", nil, testutil.WithJWTBearer(token))
	require.Equal(t, http.StatusOK, testutil.ExecuteRequest(router, list).Code)

	for _, test := range []struct {
		method string
		path   string
	}{
		{method: http.MethodPost, path: "/"},
		{method: http.MethodPut, path: "/1"},
		{method: http.MethodPut, path: "/1/allowances/1"},
	} {
		request := testutil.NewAuthenticatedRequest(t, test.method, test.path, map[string]any{"name": "Neu"}, testutil.WithJWTBearer(token))
		assert.Equal(t, http.StatusForbidden, testutil.ExecuteRequest(router, request).Code, test.method+" "+test.path)
	}
}

func setupAbsenceTypesRoute(t *testing.T) (*bun.DB, chi.Router) {
	t.Helper()

	db, svc := testutil.SetupAbsenceTypeModule(t)
	resource := NewAbsenceTypesResource(services.AbsenceTypeAdministration(svc.StaffAbsenceType), db, func(ctx context.Context) (int64, error) {
		current, err := svc.UserContext.GetCurrentStaff(ctx)
		if err != nil {
			return 0, err
		}
		return current.ID, nil
	})
	return db, resource.Router()
}
