package careplanhttp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/modules/careplan/excusedrequests"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// replaceDay lies far ahead so the approval path never sees a past day.
var replaceDay = calendar.NewDate(2030, 1, 15)

func createPartialAbsence(t *testing.T, tc *testContext, studentID, accountID int64, date calendar.Date) int64 {
	t.Helper()
	req := testutil.NewAuthenticatedRequest(t, http.MethodPost, fmt.Sprintf("/%d/partial-absences", studentID), map[string]any{
		"date": date.String(), "from_time": "13:00", "reason": "Arzttermin",
	})
	rr := authExec(t, tc, req, testutil.AdminTestClaims(int(accountID)), []string{"admin:*"})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	return partialAbsenceResponseID(t, rr.Body.Bytes())
}

func partialAbsenceCount(t *testing.T, tc *testContext, studentID, accountID int64, date calendar.Date) int {
	t.Helper()
	req := testutil.NewAuthenticatedRequest(t, http.MethodGet,
		fmt.Sprintf("/%d/partial-absences?from=%s&to=%s", studentID, date.String(), date.String()), nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(int(accountID)), []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var env struct {
		Data []json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
	return len(env.Data)
}

// Staff may replace a partial absence with a full-day status, but only when
// they confirm it for that day (#5698).
func TestStatusDayReplacesConfirmedPartialAbsence(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	student := testpkg.CreateTestStudent(t, tc.db, "Replace", "Partial", "RP1")
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Replace", "Teacher")
	free := replaceDay.AddDays(1)
	createPartialAbsence(t, tc, student.ID, account.ID, replaceDay)

	post := func(body map[string]any, perms []string) (int, string) {
		req := testutil.NewAuthenticatedRequest(t, http.MethodPost, fmt.Sprintf("/%d/status-days", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), perms)
		return rr.Code, rr.Body.String()
	}
	days := []string{replaceDay.String(), free.String()}

	code, body := post(map[string]any{"status": "sick", "dates": days}, []string{"admin:*"})
	require.Equal(t, http.StatusConflict, code, body)
	assert.Contains(t, body, "students.partial_absence_conflict", "without confirmation the partial absence still refuses")

	code, body = post(map[string]any{"status": "sick", "dates": days, "replace_partial_absence_dates": []string{"2030-02-01"}}, []string{"admin:*"})
	assert.Equal(t, http.StatusBadRequest, code, "a confirmed day must be one of the requested days: %s", body)

	code, body = post(map[string]any{"status": "sick", "dates": days, "replace_partial_absence_dates": days}, []string{"users:read"})
	assert.Equal(t, http.StatusForbidden, code, "no absence right, no replacement: %s", body)
	// An absence-only role may report the day but not remove the partial
	// absence: that needs the same right as deleting it (#2232).
	code, body = post(map[string]any{"status": "sick", "dates": days, "replace_partial_absence_dates": []string{replaceDay.String()}}, []string{"users:read", "users:absence"})
	assert.Equal(t, http.StatusForbidden, code, "absence-only role, no replacement: %s", body)
	assert.Equal(t, 1, partialAbsenceCount(t, tc, student.ID, account.ID, replaceDay))

	// A second partial absence the caller did not confirm still refuses the
	// write, and the confirmed one is not removed by the refused request.
	createPartialAbsence(t, tc, student.ID, account.ID, free)
	code, body = post(map[string]any{"status": "sick", "dates": days, "replace_partial_absence_dates": []string{replaceDay.String()}}, []string{"admin:*"})
	require.Equal(t, http.StatusConflict, code, body)
	assert.Equal(t, 1, partialAbsenceCount(t, tc, student.ID, account.ID, replaceDay), "a refused write keeps the confirmed partial absence")
	code, body = post(map[string]any{"status": "sick", "dates": days, "replace_partial_absence_dates": days}, []string{"admin:*"})
	require.Equal(t, http.StatusCreated, code, body)
	assert.Zero(t, partialAbsenceCount(t, tc, student.ID, account.ID, free))

	assert.Zero(t, partialAbsenceCount(t, tc, student.ID, account.ID, replaceDay), "the partial absence is gone")
	rows, err := tc.statusDays.FindActiveByStudentAndDateRange(testpkg.Ctx(t), student.ID, replaceDay, free)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	for _, row := range rows {
		assert.Equal(t, "sick", row.Status)
	}
}

// A child of another school does not exist here, so nothing of its partial
// absence can be replaced through this school.
func TestStatusDayReplacementStaysInTheTenant(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Tenant", "Teacher")
	otherTenant, _ := testpkg.CreateTestTenant(t, tc.db)
	foreign := testpkg.CreateTestStudentForTenant(t, tc.db, otherTenant, "Fremd", "Kind", "4b")

	req := testutil.NewAuthenticatedRequest(t, http.MethodPost, fmt.Sprintf("/%d/status-days", foreign.ID), map[string]any{
		"status": "sick", "dates": []string{replaceDay.String()}, "replace_partial_absence_dates": []string{replaceDay.String()},
	})
	rr := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})
	assert.Equal(t, http.StatusNotFound, rr.Code, rr.Body.String())
}

// A parent request on a day with a partial absence reaches the school. The
// review queue names the partial absence; approving needs the confirmation,
// which then replaces it (#5698).
func TestApproveExcusedRequestReplacesConfirmedPartialAbsence(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	chain := testpkg.CreateTestParentGuardianChain(t, tc.db)
	_, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Queue", "Staff")
	createPartialAbsence(t, tc, chain.StudentID, account.ID, replaceDay)

	var pending *excusedrequests.Request
	require.NoError(t, testpkg.WithTenantTx(t, adminTenantCtx(chain.TenantID), tc.db, chain.TenantID,
		func(txCtx context.Context, _ bun.Tx) error {
			var err error
			pending, err = tc.resource.ExcusedRequestService.CreateRequestForStatus(txCtx, chain.StudentID, chain.AccountID,
				[]excusedrequests.Date{excusedrequests.Date(replaceDay)}, "Fieber", "sick")
			return err
		}))
	claims := testutil.AdminTestClaims(int(account.ID))
	perms := []string{"users:read", "users:absence"}

	queue := authExec(t, tc, testutil.NewRequest(http.MethodGet, "/change-requests?types=excused", nil), claims, perms)
	require.Equal(t, http.StatusOK, queue.Code, queue.Body.String())
	assert.Contains(t, queue.Body.String(), fmt.Sprintf(`"partial_absence_from_by_date":{"%s":"13:00"}`, replaceDay))

	decide := func(replace bool) (int, string) {
		req := testutil.NewAuthenticatedRequest(t, http.MethodPost, fmt.Sprintf("/excused-absence-requests/%d/decide", pending.ID),
			map[string]any{"approve": true, "reason": "Passt so", "replace_partial_absences": replace})
		rr := authExec(t, tc, req, claims, perms)
		return rr.Code, rr.Body.String()
	}
	code, body := decide(false)
	require.Equal(t, http.StatusConflict, code, body)
	assert.Equal(t, 1, partialAbsenceCount(t, tc, chain.StudentID, account.ID, replaceDay))

	code, body = decide(true)
	require.Equal(t, http.StatusOK, code, body)
	assert.Zero(t, partialAbsenceCount(t, tc, chain.StudentID, account.ID, replaceDay))
	rows, err := tc.statusDays.FindActiveByStudentAndDateRange(testpkg.Ctx(t), chain.StudentID, replaceDay, replaceDay)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "sick", rows[0].Status)
}
