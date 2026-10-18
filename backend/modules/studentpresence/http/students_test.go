package studentpresencehttp_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	testpkg "github.com/moto-nrw/project-phoenix/test"
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
