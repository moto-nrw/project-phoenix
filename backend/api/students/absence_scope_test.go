package students_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestDirectAbsenceScope(t *testing.T) {
	t.Parallel()
	for _, scope := range []string{configModel.StudentAbsenceEditScopeAdmins, configModel.StudentAbsenceEditScopeAllStaff} {
		t.Run(scope, func(t *testing.T) {
			t.Parallel()
			_ = testpkg.OwnCtx(t)
			tc := setupStudentsRoute(t, fixedCalendarClock)
			_, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Absence", "Writer")
			claims := testutil.TeacherTestClaims(int(account.ID))
			perms := []string{"users:read", "users:update", "users:absence"}
			require.NoError(t, tc.resource.SettingsService.SetValue(testpkg.Ctx(t), configModel.KeyStudentAbsenceEditScope, scope, nil, nil))
			child := testpkg.CreateTestStudent(t, tc.db, "Detail", "Rights", "1a")
			detail := authExec(t, tc, testutil.NewRequest("GET", fmt.Sprintf("/%d", child.ID), nil), claims, perms)
			require.Equal(t, http.StatusOK, detail.Code, detail.Body.String())
			var capability struct {
				Data struct {
					HasAbsenceWriteAccess     bool `json:"has_absence_write_access"`
					HasSickExcusedWriteAccess bool `json:"has_sick_excused_write_access"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal(detail.Body.Bytes(), &capability))
			require.True(t, capability.Data.HasAbsenceWriteAccess, "class-trip authority is unchanged")
			require.Equal(t, scope == configModel.StudentAbsenceEditScopeAllStaff, capability.Data.HasSickExcusedWriteAccess)
			for _, status := range []string{"sick", "excused", "class_trip"} {
				t.Run(status, func(t *testing.T) {
					student := testpkg.CreateTestStudent(t, tc.db, "Absence", status, "1a")
					want := http.StatusCreated
					if scope == configModel.StudentAbsenceEditScopeAdmins && status != "class_trip" {
						want = http.StatusForbidden
					}
					req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
						"status": status, "dates": []string{timezone.DateFromTime(fixedCalendarClock()).AddDays(1).String()},
					})
					rr := authExec(t, tc, req, claims, perms)
					require.Equal(t, want, rr.Code, rr.Body.String())
				})
			}
			for _, value := range []bool{true, false} {
				for _, payload := range []map[string]any{{"sick": value}, {"excused": value}, {"sick": value, "school_class": "2b"}} {
					student := testpkg.CreateTestStudent(t, tc.db, "Direct", "Update", "1a")
					want := http.StatusOK
					if scope == configModel.StudentAbsenceEditScopeAdmins {
						want = http.StatusForbidden
					}
					rr := authExec(t, tc, testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), payload), claims, perms)
					require.Equal(t, want, rr.Code, rr.Body.String())
				}
			}
		})
	}
}

func TestDirectAbsenceScopeRestrictsExistingReportsOnNextRequest(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	_, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Direct", "Writer")
	claims := testutil.TeacherTestClaims(int(account.ID))
	perms := []string{"users:read", "users:update"}
	student := testpkg.CreateTestStudent(t, tc.db, "Existing", "Absences", "1a")
	date := timezone.DateFromTime(fixedCalendarClock()).AddDays(2).String()
	statusIDs := make(map[string]int64)
	for i, status := range []string{"sick", "excused", "class_trip"} {
		rr := authExec(t, tc, testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
			"status": status, "dates": []string{timezone.DateFromTime(fixedCalendarClock()).AddDays(i + 3).String()},
		}), claims, perms)
		require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
		var response struct {
			Data []struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
		require.Len(t, response.Data, 1)
		statusIDs[status] = response.Data[0].ID
	}
	partialPayload := map[string]any{"date": date, "from_time": "13:00", "reason": "Arzttermin"}
	rr := authExec(t, tc, testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/partial-absences", student.ID), partialPayload), claims, perms)
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	partialID := partialAbsenceResponseID(t, rr.Body.Bytes())
	require.NoError(t, tc.resource.SettingsService.SetValue(testpkg.Ctx(t), configModel.KeyStudentAbsenceEditScope, configModel.StudentAbsenceEditScopeAdmins, nil, nil))

	for status, id := range statusIDs {
		want := http.StatusForbidden
		if status == "class_trip" {
			want = http.StatusOK
		}
		rr := authExec(t, tc, testutil.NewRequest("DELETE", fmt.Sprintf("/%d/status-days/%d", student.ID, id), nil), claims, perms)
		require.Equal(t, want, rr.Code, rr.Body.String())
	}
	for _, method := range []string{"PUT", "DELETE"} {
		rr := authExec(t, tc, testutil.NewAuthenticatedRequest(t, method, fmt.Sprintf("/%d/partial-absences/%d", student.ID, partialID), partialPayload), claims, perms)
		require.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
	}
	other := testpkg.CreateTestStudent(t, tc.db, "Bulk", "Absence", "1b")
	rr = authExec(t, tc, testutil.NewAuthenticatedRequest(t, "POST", "/status-days/bulk", map[string]any{
		"student_ids": []int64{student.ID, other.ID}, "status": "sick", "from": date, "to": date,
	}), claims, perms)
	require.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
	rows, err := tc.resource.StudentStatusDayService.GetActiveByStudentAndDateRange(testpkg.Ctx(t), other.ID, timezone.DateFromTime(fixedCalendarClock()), timezone.DateFromTime(fixedCalendarClock()).AddDays(5))
	require.NoError(t, err)
	require.Empty(t, rows, "denied bulk write must not partially commit")

	// Admin rights and ordinary child edits remain independent of direct reports.
	rr = authExec(t, tc, testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", other.ID), map[string]any{"school_class": "2b"}), claims, perms)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	rr = authExec(t, tc, testutil.NewRequest("DELETE", fmt.Sprintf("/%d/status-days/%d", student.ID, statusIDs["sick"]), nil), testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	// Reset restores the registered default without renewing the actor's claims.
	require.NoError(t, tc.resource.SettingsService.ResetValue(testpkg.Ctx(t), configModel.KeyStudentAbsenceEditScope, nil, nil))
	rr = authExec(t, tc, testutil.NewRequest("DELETE", fmt.Sprintf("/%d/status-days/%d", student.ID, statusIDs["excused"]), nil), claims, perms)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
}

func TestDirectAbsenceScopeFailsClosed(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"missing", "error", "unknown", "non-staff", "missing-read", "school", "parent", "operator"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			_ = testpkg.OwnCtx(t)
			tc := setupStudentsRoute(t, fixedCalendarClock)
			_, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Scope", "Actor")
			student := testpkg.CreateTestStudent(t, tc.db, "Scope", "Child", "1a")
			claims := testutil.TeacherTestClaims(int(account.ID))
			perms := []string{"users:read", "users:absence"}
			switch mode {
			case "missing":
				tc.resource.SettingsService = nil
			case "error", "unknown":
				tc.resource.SettingsService = &configtest.Mock{ResolveStringFn: func(context.Context, string) (string, error) {
					if mode == "error" {
						return "all_staff", errors.New("settings unavailable")
					}
					return "unexpected", nil
				}}
			case "non-staff":
				_, guest := testpkg.CreateTestPersonWithAccount(t, tc.db, "Guest", "Actor")
				claims.ID = int(guest.ID)
			case "missing-read":
				perms = []string{"users:absence"}
			default:
				claims.Scope = mode
			}
			rr := authExec(t, tc, testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
				"status": "sick", "dates": []string{timezone.DateFromTime(fixedCalendarClock()).AddDays(1).String()},
			}), claims, perms)
			require.Contains(t, []int{http.StatusForbidden, http.StatusUnauthorized}, rr.Code, rr.Body.String())
			rows, err := tc.resource.StudentStatusDayService.GetActiveByStudentAndDateRange(testpkg.Ctx(t), student.ID, timezone.DateFromTime(fixedCalendarClock()), timezone.DateFromTime(fixedCalendarClock()).AddDays(2))
			require.NoError(t, err)
			require.Empty(t, rows)
		})
	}
}
