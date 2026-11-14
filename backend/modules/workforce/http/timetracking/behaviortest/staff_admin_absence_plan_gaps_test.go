package behaviortest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/modules/workforce"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
)

const planGapsPath = "/staff/absences/plan-gaps"

// planGapClock lies a week before the absence days, inside the cascade
// window and after every live-clock past-day guard.
func planGapClock() time.Time { return time.Date(2098, 12, 30, 12, 0, 0, 0, time.UTC) }

// planGapsAPI is the staff route over the real shift-plan-sync entry, with a
// manager whose account resolves to a staff member.
type planGapsAPI struct {
	tc        *testContext
	accountID int64
}

func setupPlanGapsAPI(t *testing.T) planGapsAPI {
	t.Helper()
	tc := setupStaffRouteWith(t, nil, planGapClock)
	person, account := testpkg.CreateTestPersonWithAccount(t, tc.db, "Plan", fmt.Sprintf("Leitung-%d", time.Now().UnixNano()))
	testpkg.CreateTestStaffForPerson(t, tc.db, person.ID)
	return planGapsAPI{tc: tc, accountID: account.ID}
}

func (a planGapsAPI) request(t *testing.T, method, path string, counter *testpkg.QueryCounter, perms ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	claims := testutil.DefaultTestClaims()
	claims.ID = int(a.accountID)
	claims.Permissions = perms
	req.Header.Set("Authorization", "Bearer "+testutil.MintTestJWT(t, claims))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	if counter == nil {
		a.tc.router.ServeHTTP(rec, req)
		return rec
	}
	req = req.WithContext(counter.Context(context.Background()))
	testpkg.MeasuredHTTPRequest(counter, testpkg.MountedQueryBudgetRouter("/api", a.tc.router), rec, req)
	return rec
}

func (a planGapsAPI) gaps(t *testing.T) []workforce.AbsencePlanGap {
	t.Helper()
	rec := a.request(t, http.MethodGet, planGapsPath, nil, "vacation:approve")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body struct {
		Data []workforce.AbsencePlanGap `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body.Data
}

func dataOf(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return string(body.Data)
}

// preCascadeVacation is an approved vacation stored the way the plans saw it
// before #6847, with a planned shift and a staffed block on its day.
func (a planGapsAPI) preCascadeVacation(t *testing.T, tenantID int64, day isoDay) int64 {
	t.Helper()
	db := a.tc.db
	staff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Urlaub", fmt.Sprintf("Vor-%d", time.Now().UnixNano()))
	if tenantID == testpkg.Tenant(t) {
		testpkg.CreateTestStaffShift(t, db, staff.ID, day, testpkg.StaffShiftOpts{})
		room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Plan-Raum-%d", staff.ID))
		instance := testpkg.CreateTestActivityInstance(t, db, day, room.ID, testpkg.ActivityInstanceOpts{})
		testpkg.CreateTestInstanceStaff(t, db, instance.ID, staff.ID, testpkg.InstanceStaffOpts{})
	}
	var id int64
	require.NoError(t, db.NewRaw(
		`INSERT INTO active.staff_absences (tenant_id, staff_id, absence_type, date_start, date_end, status, created_by, working_days, requested_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 1, NOW()) RETURNING id`,
		tenantID, staff.ID, workforce.AbsenceTypeVacation, day.String(), day.String(), workforce.AbsenceStatusApproved, staff.ID,
	).Scan(testpkg.Ctx(t), &id))
	return id
}

const planGapDay isoDay = "2099-01-06"

// #7021: the Leitung sees the absences in force whose plans predate the
// cascade, takes them over, and they leave the list; another school's
// absence never appears. The take-over needs a manager permission.
func TestAbsencePlanGapsAPI_ListAndTakeOver(t *testing.T) {
	t.Parallel()
	api := setupPlanGapsAPI(t)
	foreign := testpkg.NewTenantScope(t, api.tc.db)
	foreignID := api.preCascadeVacation(t, foreign.TenantID, planGapDay)
	first := api.preCascadeVacation(t, testpkg.Tenant(t), planGapDay)
	second := api.preCascadeVacation(t, testpkg.Tenant(t), planGapDay)

	gaps := api.gaps(t)
	require.Len(t, gaps, 2, "only this school's absences")
	for _, gap := range gaps {
		assert.Equal(t, 1, gap.OpenShifts)
		assert.Equal(t, 1, gap.OpenBlocks)
	}

	syncPath := func(id int64) string { return fmt.Sprintf("/staff/absences/%d/plan-sync", id) }
	rec := api.request(t, http.MethodPost, syncPath(first), nil, "users:read")
	assert.Equal(t, http.StatusForbidden, rec.Code, "the take-over needs vacation:approve or time_tracking:manage")
	rec = api.request(t, http.MethodPost, syncPath(foreignID), nil, "time_tracking:manage")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"applied":false}`, dataOf(t, rec), "another school's absence is out of reach")

	for _, id := range []int64{first, second} {
		rec = api.request(t, http.MethodPost, syncPath(id), nil, "time_tracking:manage")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.JSONEq(t, `{"applied":true}`, dataOf(t, rec))
	}
	assert.Empty(t, api.gaps(t), "taken-over absences are no gaps any more")
}

// Rule 15 (#2940): the list reads the absences, shifts, assignments and
// blocks once each, however many absences are in force.
func TestAbsencePlanGapsAPI_ListQueryBudget(t *testing.T) {
	t.Parallel()
	api := setupPlanGapsAPI(t)
	measure := func() testpkg.QueryMeasurement {
		counter := testpkg.CaptureQueriesForContext(t, api.tc.db)
		rec := api.request(t, http.MethodGet, planGapsPath, counter, "vacation:approve")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		queries := counter.Queries()
		counter.Stop()
		return counter.Measurement(queries)
	}
	for range 2 {
		api.preCascadeVacation(t, testpkg.Tenant(t), planGapDay)
	}
	small := measure()
	for range 3 {
		api.preCascadeVacation(t, testpkg.Tenant(t), planGapDay)
	}
	large := measure()

	assert.Equal(t, len(small.Statements), len(large.Statements), "the list must not issue a statement per absence")
	querybudgets.AssertQueryBudget(t, "workforce.absence_plan_gaps.list", large)
}
