package careplanhttp_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
)

func TestStudentAuthorization_NonAdminAccess(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	// Create teacher, group, and student
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Auth", "Teacher")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "AuthTestGroup")
	student := testpkg.CreateTestStudent(t, tc.db, "Auth", "Student", "AT1")

	// Assign student to group
	testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)

	// Assign teacher to group
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	t.Run("teacher_can_view_student_in_supervised_group", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)

		claims := testutil.TeacherTestClaims(int(account.ID))
		rr := authExec(t, tc, req, claims, []string{"users:read"})

		assert.Equal(t, http.StatusOK, rr.Code, "Teacher should view supervised student. Body: %s", rr.Body.String())
	})

	t.Run("staff_outside_the_group_can_update", func(t *testing.T) {
		// #2329: a staff member who does not supervise the child's group is a
		// full writer — the route permission decides, not the group.
		_, otherAccount := testpkg.CreateTestStaffWithAccount(t, tc.db, "Other", "Staff")

		body := map[string]interface{}{
			"first_name": "Updated",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		claims := testutil.TeacherTestClaims(int(otherAccount.ID))
		rr := authExec(t, tc, req, claims, []string{"users:update"})

		assert.Equal(t, http.StatusOK, rr.Code, "Staff should update any student. Body: %s", rr.Body.String())
	})

	t.Run("account_without_staff_record_cannot_update", func(t *testing.T) {
		// Guests and guardians authenticate against the same portal; holding
		// users:update is not enough without a staff record in this tenant.
		guest := testpkg.CreateTestAccount(t, tc.db, "students-auth-guest@example.com")

		body := map[string]interface{}{
			"first_name": "GuestUpdated",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		claims := testutil.TeacherTestClaims(int(guest.ID))
		rr := authExec(t, tc, req, claims, []string{"users:update"})

		testutil.AssertForbidden(t, rr)
		assert.Contains(t, rr.Body.String(), "only staff members can update")
	})

	t.Run("account_without_staff_record_cannot_delete", func(t *testing.T) {
		guest := testpkg.CreateTestAccount(t, tc.db, "students-auth-guest-delete@example.com")

		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d", student.ID), nil)

		claims := testutil.TeacherTestClaims(int(guest.ID))
		rr := authExec(t, tc, req, claims, []string{"users:delete"})

		testutil.AssertForbidden(t, rr)
		assert.Contains(t, rr.Body.String(), "only staff members can delete")
	})
}

func TestStudentAuthorization_StudentWithoutGroup(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	// Create student without group assignment
	student := testpkg.CreateTestStudent(t, tc.db, "NoGroup", "Student", "NG1")

	_, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "NoGroup", "Staff")

	t.Run("staff_can_update_student_without_group", func(t *testing.T) {
		// #2329: a child without a group is an ordinary child, no longer
		// admin-only.
		body := map[string]interface{}{
			"first_name": "Updated",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		claims := testutil.TeacherTestClaims(int(account.ID))
		rr := authExec(t, tc, req, claims, []string{"users:update"})

		assert.Equal(t, http.StatusOK, rr.Code, "Staff should update groupless student. Body: %s", rr.Body.String())
	})

	t.Run("admin_can_update_student_without_group", func(t *testing.T) {
		body := map[string]interface{}{
			"first_name": "AdminUpdated",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Admin should update groupless student. Body: %s", rr.Body.String())
	})
}
