package studentpresencehttp_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateStudent_WithSickStatus(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	student := testpkg.CreateTestStudent(t, tc.db, "Sick", "Status", "SS1")

	t.Run("mark_as_sick", func(t *testing.T) {
		body := map[string]interface{}{
			"sick": true,
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), `"sick":true`)
	})

	t.Run("mark_as_not_sick", func(t *testing.T) {
		body := map[string]interface{}{
			"sick": false,
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestUpdateStudent_SickStatusExtended(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("mark_student_as_sick_sets_sick_since", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "SickSince", "Student", "SS2")

		// Mark as sick
		body := map[string]interface{}{
			"sick": true,
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Should mark student as sick")
		assert.Contains(t, rr.Body.String(), `"sick":true`)
	})

	t.Run("clear_sick_status_clears_sick_since", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ClearSick", "Student", "CS1")

		// First mark as sick
		sickBody := map[string]interface{}{
			"sick": true,
		}
		sickReq := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), sickBody)
		sickRR := authExec(t, tc, sickReq, testutil.AdminTestClaims(1), []string{"admin:*"})
		assert.Equal(t, http.StatusOK, sickRR.Code)

		// Then clear sick status
		clearBody := map[string]interface{}{
			"sick": false,
		}
		clearReq := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), clearBody)
		clearRR := authExec(t, tc, clearReq, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, clearRR.Code, "Should clear sick status")
		assert.Contains(t, clearRR.Body.String(), `"sick":false`)

		var history testpkg.StudentStatusDayRow
		err := tc.db.NewSelect().
			Model(&history).
			ModelTableExpr(`active.student_status_days AS "student_status_day"`).
			Where(`"student_status_day".student_id = ?`, student.ID).
			Where(`"student_status_day".status = ?`, absencerecords.StudentStatusDaySick).
			Scan(context.Background())
		require.NoError(t, err)
		assert.NotNil(t, history.ClearedAt)
	})
}

// TestUpdateStudent_SickExcusedMutualExclusion encodes the business rule
// that a student cannot be both sick and excused at the same time.
func TestUpdateStudent_SickExcusedMutualExclusion(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("both_flags_true_in_same_request_is_rejected", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Mutex", "Student", "MS1")

		body := map[string]interface{}{"sick": true, "excused": true}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusConflict, rr.Code,
			"setting both sick and excused to true must return 409")
		assert.Contains(t, rr.Body.String(), common.CodeStudentsSickExcusedConflict,
			"response should carry the SICK_EXCUSED_CONFLICT code for the frontend")
	})

	t.Run("setting_sick_while_already_excused_is_rejected", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Mutex", "Excused", "ME1")

		// Pre-state: excused = true
		excReq := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID),
			map[string]interface{}{"excused": true})
		excRR := authExec(t, tc, excReq, testutil.AdminTestClaims(1), []string{"admin:*"})
		assert.Equal(t, http.StatusOK, excRR.Code)

		// Attempt: set sick = true without clearing excused
		sickReq := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID),
			map[string]interface{}{"sick": true})
		sickRR := authExec(t, tc, sickReq, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusConflict, sickRR.Code)
		assert.Contains(t, sickRR.Body.String(), common.CodeStudentsSickExcusedConflict)
	})

	t.Run("switch_from_sick_to_excused_in_one_request_succeeds", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Switch", "Student", "SW1")

		// Pre-state: sick = true
		sickReq := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID),
			map[string]interface{}{"sick": true})
		sickRR := authExec(t, tc, sickReq, testutil.AdminTestClaims(1), []string{"admin:*"})
		assert.Equal(t, http.StatusOK, sickRR.Code)

		// Switch: clear sick AND set excused in one request
		switchReq := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID),
			map[string]interface{}{"sick": false, "excused": true})
		switchRR := authExec(t, tc, switchReq, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, switchRR.Code,
			"simultaneous clear-one + set-other must be accepted")
		assert.Contains(t, switchRR.Body.String(), `"sick":false`)
		assert.Contains(t, switchRR.Body.String(), `"excused":true`)
	})
}

// The student PUT writes a status through the status-day path (#6959), so it
// refuses a day that already carries a class trip, as the status-day route
// does, instead of leaving two statuses standing on the same day.
func TestUpdateStudent_StatusOnClassTripDayIsRejected(t *testing.T) {
	t.Parallel()

	today := calendar.NewDate(2026, 9, 9)
	tc := setupStudentsRoute(t, func() time.Time { return today.BerlinMidnight().Add(10 * time.Hour) })

	for _, status := range []string{absencerecords.StudentStatusDaySick, absencerecords.StudentStatusDayExcused} {
		t.Run(status, func(t *testing.T) {
			student := testpkg.CreateTestStudent(t, tc.db, "ClassTrip", "Put"+status, "CT1")
			testpkg.CreateTestStudentStatusDay(t, tc.db, student.ID, today, absencerecords.StudentStatusDayClassTrip)

			rr := authExec(t, tc, testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID),
				map[string]any{status: true}), testutil.AdminTestClaims(1), []string{"admin:*"})

			require.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())
			assert.Contains(t, rr.Body.String(), common.CodeStudentsStatusDayConflict)

			var rows []testpkg.StudentStatusDayRow
			require.NoError(t, tc.db.NewSelect().Model(&rows).
				ModelTableExpr(`active.student_status_days AS "student_status_day"`).
				Where(`"student_status_day".student_id = ?`, student.ID).
				Where(`"student_status_day".cleared_at IS NULL`).Scan(context.Background()))
			require.Len(t, rows, 1, "only the class trip stands")
			assert.Equal(t, absencerecords.StudentStatusDayClassTrip, rows[0].Status)

			state, err := tc.resource.PeopleDirectory.FindStudentRecord(testpkg.Ctx(t), student.ID)
			require.NoError(t, err)
			assert.False(t, state.Sick != nil && *state.Sick, "the live flag stays unset")
			assert.False(t, state.Excused != nil && *state.Excused, "the live flag stays unset")
		})
	}
}

// The live flag the PUT derives carries the request's own instant, the one its
// status day is dated by, not a second wall-clock reading (#6959).
func TestUpdateStudent_StatusSinceUsesRequestInstant(t *testing.T) {
	t.Parallel()

	now := calendar.NewDate(2026, 9, 9).BerlinMidnight().Add(10 * time.Hour)
	tc := setupStudentsRoute(t, func() time.Time { return now })

	for _, status := range []string{absencerecords.StudentStatusDaySick, absencerecords.StudentStatusDayExcused} {
		t.Run(status, func(t *testing.T) {
			student := testpkg.CreateTestStudent(t, tc.db, "Since", "Put"+status, "SI1")

			rr := authExec(t, tc, testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID),
				map[string]any{status: true}), testutil.AdminTestClaims(1), []string{"admin:*"})
			require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

			state, err := tc.resource.PeopleDirectory.FindStudentRecord(testpkg.Ctx(t), student.ID)
			require.NoError(t, err)
			since := state.SickSince
			if status == absencerecords.StudentStatusDayExcused {
				since = state.ExcusedSince
			}
			require.NotNil(t, since)
			assert.True(t, since.Equal(now), "since %s, want %s", since, now)
		})
	}
}
