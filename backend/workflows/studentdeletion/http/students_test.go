package studentdeletionhttp_test

import (
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
)

func TestDeleteStudent(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("success_deletes_student", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Delete", "Me", "DM1")
		// No cleanup needed - we're deleting

		rr := deleteStudentConfirmed(t, tc, testutil.AdminTestClaims(1), student.ID)

		// Handler returns 200 OK with success message (not 204 NoContent)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "deleted successfully")
	})

	t.Run("not_found_for_nonexistent_student", func(t *testing.T) {
		req := testutil.NewRequest("DELETE", "/999999", nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("bad_request_for_invalid_id", func(t *testing.T) {
		req := testutil.NewRequest("DELETE", "/invalid", nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})
}
