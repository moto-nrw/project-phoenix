package inbound

import (
	"encoding/json"
	"fmt"
	"log/slog"
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
		"name": "Regenerationstag", "allowance_enabled": true,
	}, testutil.WithJWTBearer(token))
	created := testutil.ExecuteRequest(router, create)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	var typeEnvelope struct {
		Data struct {
			ID               string  `json:"id"`
			AllowanceEnabled bool    `json:"allowance_enabled"`
			OverrunPolicy    *string `json:"overrun_policy"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &typeEnvelope))
	assert.True(t, typeEnvelope.Data.AllowanceEnabled)
	assert.Nil(t, typeEnvelope.Data.OverrunPolicy, "the overrun choice is gone from the wire (#3256)")

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

// #3257: the carryover rule travels on the type, and the preview shows the
// yearly split of a booking. An exceeded allowance is an answer, not an error.
func TestAbsenceTypesRouteCarryoverAndPreview(t *testing.T) {
	t.Parallel()

	db, router := setupAbsenceTypesRoute(t)
	_, account := testpkg.CreateTestStaffWithAccount(t, db, "Swantje", "Leitung")
	staff := testpkg.CreateTestStaff(t, db, "Rena", "Übertrag")
	claims := testutil.DefaultTestClaims()
	claims.ID = int(account.ID)
	claims.TenantID = testpkg.Tenant(t)
	claims.Permissions = []string{permissions.TimeTrackingManage}
	claims.IsAdmin = false
	token := testutil.MintTestJWT(t, claims)

	type typeData struct {
		ID             string  `json:"id"`
		CarryoverUntil *string `json:"carryover_until"`
	}
	decodeType := func(body []byte) typeData {
		var envelope struct {
			Data typeData `json:"data"`
		}
		require.NoError(t, json.Unmarshal(body, &envelope))
		return envelope.Data
	}
	created := testutil.ExecuteRequest(router, testutil.NewAuthenticatedRequest(t, "POST", "/", map[string]any{
		"name": "Krank-Urlaubstag", "allowance_enabled": true, "carryover_until": "03-31",
	}, testutil.WithJWTBearer(token)))
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	typ := decodeType(created.Body.Bytes())
	require.NotNil(t, typ.CarryoverUntil)
	assert.Equal(t, "03-31", *typ.CarryoverUntil)

	invalid := testutil.ExecuteRequest(router, testutil.NewAuthenticatedRequest(t, "PUT", "/"+typ.ID, map[string]any{
		"carryover_until": "02-29",
	}, testutil.WithJWTBearer(token)))
	assert.Equal(t, http.StatusBadRequest, invalid.Code, invalid.Body.String())

	path := fmt.Sprintf("/%s/allowances/%d", typ.ID, staff.ID)
	for year, days := range map[int]float64{2030: 1, 2031: 1} {
		saved := testutil.ExecuteRequest(router, testutil.NewAuthenticatedRequest(t, "PUT", path, map[string]any{
			"year": year, "entitled_days": days, "reason": "Anspruch",
		}, testutil.WithJWTBearer(token)))
		require.Equal(t, http.StatusOK, saved.Code, saved.Body.String())
	}

	type previewYear struct {
		Year          int     `json:"year"`
		BookingDays   float64 `json:"booking_days"`
		RemainingDays float64 `json:"remaining_days"`
		ExpiresOn     string  `json:"expires_on"`
		CarriedIn     *struct {
			Year          int     `json:"year"`
			RemainingDays float64 `json:"remaining_days"`
		} `json:"carried_in"`
	}
	preview := func(query string) (int, []previewYear, bool) {
		response := testutil.ExecuteRequest(router, testutil.NewAuthenticatedRequest(t, "GET", path+"/preview?"+query, nil, testutil.WithJWTBearer(token)))
		var envelope struct {
			Data struct {
				Years   []previewYear `json:"years"`
				Blocked bool          `json:"blocked"`
			} `json:"data"`
		}
		if response.Code == http.StatusOK {
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
		}
		return response.Code, envelope.Data.Years, envelope.Data.Blocked
	}

	// Mo 13.01. und Di 14.01.2031: ein Tag aus 2030, einer aus 2031.
	code, years, blocked := preview("date_start=2031-01-13&date_end=2031-01-14")
	require.Equal(t, http.StatusOK, code)
	assert.False(t, blocked)
	require.Len(t, years, 2)
	assert.Equal(t, previewYear{Year: 2030, BookingDays: 1, ExpiresOn: "2031-03-31"}, years[0])
	assert.Equal(t, 2031, years[1].Year)
	assert.Equal(t, 1.0, years[1].BookingDays)
	require.NotNil(t, years[1].CarriedIn)
	assert.Equal(t, 2030, years[1].CarriedIn.Year)

	code, years, blocked = preview("date_start=2031-01-13&date_end=2031-01-15")
	require.Equal(t, http.StatusOK, code)
	assert.True(t, blocked, "three days do not fit into two")
	require.Len(t, years, 2)
	assert.Equal(t, -1.0, years[1].RemainingDays)

	code, _, _ = preview("date_start=2031-01-15&date_end=2031-01-13")
	assert.Equal(t, http.StatusBadRequest, code)

	ownClaims := claims
	ownClaims.Permissions = []string{permissions.TimeTrackingOwn}
	denied := testutil.NewAuthenticatedRequest(t, "GET", path+"/preview?date_start=2031-01-13&date_end=2031-01-13", nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, ownClaims)))
	assert.Equal(t, http.StatusForbidden, testutil.ExecuteRequest(router, denied).Code)
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
	resource := NewAbsenceTypesResource(services.AbsenceTypeAdministration(svc.Catalog, slog.Default()), db, svc.UserContext.Caller().StaffID)
	return db, resource.Router()
}
