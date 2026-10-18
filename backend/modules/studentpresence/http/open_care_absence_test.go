package studentpresencehttp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAbsenceWriter_GrouplessStudent(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	// A staff member (the Sekretariat case) who supervises nothing, and a child
	// with no group at all.
	_, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Sekretariat", "Barnstorf")
	student := testpkg.CreateTestStudent(t, tc.db, "Gruppenlos", "Kind", "1a")

	claims := testutil.TeacherTestClaims(int(account.ID))
	absencePerms := []string{"users:read", "users:absence"}
	today := calendar.TodayDate().String()

	var createdStatusDayID int64

	t.Run("can_report_planned_sick_day", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
			"status": "sick",
			"dates":  []string{today},
			"reason": "Fieber",
		})
		rr := authExec(t, tc, req, claims, absencePerms)
		require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

		var env struct {
			Data []struct {
				ID     int64  `json:"id"`
				Status string `json:"status"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
		require.NotEmpty(t, env.Data)
		assert.Equal(t, "sick", env.Data[0].Status)
		createdStatusDayID = env.Data[0].ID
	})

	t.Run("can_read_the_planned_status_days", func(t *testing.T) {
		// The planning dialog refuses to save until it has loaded the existing
		// status days, so read access to them has to follow the write gate.
		rr := authExec(t, tc, testutil.NewRequest("GET",
			fmt.Sprintf("/%d/status-days?from=%s&to=%s", student.ID, today, today), nil), claims, absencePerms)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

		var env struct {
			Data []struct {
				Status string `json:"status"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
		require.NotEmpty(t, env.Data, "the day reported above must be visible to its author")
	})

	t.Run("can_clear_the_sick_day_again", func(t *testing.T) {
		require.NotZero(t, createdStatusDayID, "depends on the create sub-test")
		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/status-days/%d", student.ID, createdStatusDayID), nil)
		rr := authExec(t, tc, req, claims, absencePerms)
		assert.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	})

	t.Run("can_report_today_via_student_update", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), map[string]any{
			"sick": true,
		})
		rr := authExec(t, tc, req, claims, absencePerms)
		assert.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	})

	t.Run("can_clear_today_via_student_update", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), map[string]any{
			"sick": false,
		})
		assert.Equal(t, http.StatusOK, authExec(t, tc, req, claims, absencePerms).Code)
	})

	t.Run("cannot_edit_stammdaten", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), map[string]any{
			"school_class": "2b",
		})
		testutil.AssertForbidden(t, authExec(t, tc, req, claims, absencePerms))
	})

	t.Run("cannot_smuggle_stammdaten_alongside_the_absence", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), map[string]any{
			"sick":        true,
			"health_info": "smuggled",
		})
		testutil.AssertForbidden(t, authExec(t, tc, req, claims, absencePerms))
	})

	t.Run("users_update_reports_the_absence_too", func(t *testing.T) {
		// users:update is the permission that gated these writes before #2232 and
		// still does: the route admits either, and the per-child gate asks only
		// for a staff record (#2329).
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
			"status": "sick",
			"dates":  []string{today},
		})
		rr := authExec(t, tc, req, claims, []string{"users:read", "users:update"})
		require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

		var env struct {
			Data []struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
		require.NotEmpty(t, env.Data)

		// Clear it again so the shared child stays neutral for the sub-tests
		// that follow.
		del := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/status-days/%d", student.ID, env.Data[0].ID), nil)
		require.Equal(t, http.StatusOK, authExec(t, tc, del, claims, []string{"users:read", "users:update"}).Code)
	})

	// users:absence is a write scope on top of what a caller may already see.
	// Without users:read it grants nothing — not the write, and not the read
	// the planning dialog needs — instead of half a feature for a child whose
	// list entry and detail page stay closed.
	t.Run("the_absence_permission_alone_grants_nothing", func(t *testing.T) {
		absenceOnly := []string{"users:absence"}

		write := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
			"status": "sick",
			"dates":  []string{today},
		})
		testutil.AssertForbidden(t, authExec(t, tc, write, claims, absenceOnly))

		read := testutil.NewRequest("GET", fmt.Sprintf("/%d/status-days", student.ID), nil)
		testutil.AssertForbidden(t, authExec(t, tc, read, claims, absenceOnly))
	})
}

// TestAbsenceWithoutReadPermissionRefused pins the read prerequisite for a
// caller holding users:absence alone. The route gate admits that permission on
// its own, so without this check the caller would inherit the absence write —
// and the guardian requests behind it — from merely being staff, in a school
// that never granted the pair.
func TestAbsenceWithoutReadPermissionRefused(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "NoRead", "Supervisor")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "NoReadGroup")
	student := testpkg.CreateTestStudent(t, tc.db, "NoRead", "Kind", "1a")
	testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
		"status": "sick",
		"dates":  []string{calendar.TodayDate().String()},
	})
	testutil.AssertForbidden(t, authExec(t, tc, req, claims, []string{"users:absence"}))

	// The same caller WITH users:read keeps working — what was missing is the
	// permission, nothing else.
	ok := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
		"status": "sick",
		"dates":  []string{calendar.TodayDate().String()},
	})
	rr := authExec(t, tc, ok, claims, []string{"users:read", "users:absence"})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
}
