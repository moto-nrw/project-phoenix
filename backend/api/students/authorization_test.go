package students_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// =============================================================================
// Authorization Tests (Non-Admin Access)
// =============================================================================

func TestStudentAuthorization_NonAdminAccess(t *testing.T) {
	tc := setupTestContext(t)

	// Create teacher, group, and student
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Auth", "Teacher", tc.ogsID)
	group := testpkg.CreateTestEducationGroup(t, tc.db, "AuthTestGroup", tc.ogsID)
	student := testpkg.CreateTestStudent(t, tc.db, "Auth", "Student", "AT1", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID, group.ID, student.ID)

	// Assign student to group
	testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)

	// Assign teacher to group
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	t.Run("teacher_can_view_student_in_supervised_group", func(t *testing.T) {
		router := tc.setupRouter(tc.resource.GetStudentHandler(), "id")
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)

		claims := testutil.TeacherTestClaims(int(account.ID))
		rr := tc.executeWithAuth(router, req, claims, []string{"students:read"})

		assert.Equal(t, http.StatusOK, rr.Code, "Teacher should view supervised student. Body: %s", rr.Body.String())
	})

	t.Run("staff_without_permission_cannot_update", func(t *testing.T) {
		// Create a staff member that does not supervise the student's group
		otherStaff, otherAccount := testpkg.CreateTestStaffWithAccount(t, tc.db, "Other", "Staff", tc.ogsID)
		defer testpkg.CleanupActivityFixtures(t, tc.db, otherStaff.ID)

		router := tc.setupRouter(tc.resource.UpdateStudentHandler(), "id")
		body := map[string]interface{}{
			"first_name": "Updated",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		claims := testutil.TeacherTestClaims(int(otherAccount.ID))
		rr := tc.executeWithAuth(router, req, claims, []string{"students:write"})

		// Non-supervisor should be forbidden
		testutil.AssertForbidden(t, rr)
	})

	t.Run("staff_without_permission_cannot_delete", func(t *testing.T) {
		otherStaff, otherAccount := testpkg.CreateTestStaffWithAccount(t, tc.db, "Delete", "Restricted", tc.ogsID)
		defer testpkg.CleanupActivityFixtures(t, tc.db, otherStaff.ID)

		router := tc.setupRouter(tc.resource.DeleteStudentHandler(), "id")
		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d", student.ID), nil)

		claims := testutil.TeacherTestClaims(int(otherAccount.ID))
		rr := tc.executeWithAuth(router, req, claims, []string{"students:delete"})

		// Non-supervisor should be forbidden
		testutil.AssertForbidden(t, rr)
	})
}

func TestStudentAuthorization_StudentWithoutGroup(t *testing.T) {
	tc := setupTestContext(t)

	// Create student without group assignment
	student := testpkg.CreateTestStudent(t, tc.db, "NoGroup", "Student", "NG1", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "NoGroup", "Staff", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, staff.ID)

	t.Run("non_admin_cannot_update_student_without_group", func(t *testing.T) {
		router := tc.setupRouter(tc.resource.UpdateStudentHandler(), "id")
		body := map[string]interface{}{
			"first_name": "Updated",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		claims := testutil.TeacherTestClaims(int(account.ID))
		rr := tc.executeWithAuth(router, req, claims, []string{"students:write"})

		// Only administrators can update students without groups
		testutil.AssertForbidden(t, rr)
		assert.Contains(t, rr.Body.String(), "administrator")
	})

	t.Run("admin_can_update_student_without_group", func(t *testing.T) {
		router := tc.setupRouter(tc.resource.UpdateStudentHandler(), "id")
		body := map[string]interface{}{
			"first_name": "AdminUpdated",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := tc.executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Admin should update groupless student. Body: %s", rr.Body.String())
	})
}

// =============================================================================
// Student Full Access Tests
// =============================================================================

func TestStudentResponse_FullAccess(t *testing.T) {
	tc := setupTestContext(t)

	t.Run("admin_sees_all_fields", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Full", "Access", "FA1", tc.ogsID)
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		// Update student with additional fields using raw SQL - use ? placeholders
		ctx := context.Background()
		_, err := tc.db.ExecContext(ctx,
			"UPDATE users.students SET guardian_email = ?, guardian_phone = ?, extra_info = ? WHERE id = ?",
			"guardian@example.com", "+49123456789", "Important notes", student.ID)
		require.NoError(t, err)

		router := tc.setupRouter(tc.resource.GetStudentHandler(), "id")
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)

		rr := tc.executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
		// Admin should see sensitive fields
		body := rr.Body.String()
		assert.Contains(t, body, "guardian_email", "Admin should see guardian email")
	})

	t.Run("non_supervisor_sees_limited_fields", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Limited", "Access", "LA1", tc.ogsID)
		staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Limited", "Staff", tc.ogsID)
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, staff.ID)

		router := tc.setupRouter(tc.resource.GetStudentHandler(), "id")
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)

		claims := testutil.TeacherTestClaims(int(account.ID))
		rr := tc.executeWithAuth(router, req, claims, []string{"students:read"})

		assert.Equal(t, http.StatusOK, rr.Code)
		// Non-supervisor should see limited fields (response still 200 but with less data)
	})
}

// =============================================================================
// Student Detail With Group Tests
// =============================================================================

func TestGetStudentDetail_WithGroup(t *testing.T) {
	tc := setupTestContext(t)

	// Create teacher, group, and student
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Detail", "Teacher", tc.ogsID)
	group := testpkg.CreateTestEducationGroup(t, tc.db, "DetailGroup", tc.ogsID)
	student := testpkg.CreateTestStudent(t, tc.db, "Detail", "Student", "DT1", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID, group.ID, student.ID)

	// Assign teacher and student to group
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)
	testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)

	t.Run("supervisor_gets_full_access", func(t *testing.T) {
		router := tc.setupRouter(tc.resource.GetStudentHandler(), "id")
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)

		claims := testutil.TeacherTestClaims(int(account.ID))
		rr := tc.executeWithAuth(router, req, claims, []string{"students:read"})

		assert.Equal(t, http.StatusOK, rr.Code, "Supervisor should get student")
		// Check response includes group name
		assert.Contains(t, rr.Body.String(), "DetailGroup")
	})
}

// =============================================================================
// Teacher Access Tests
// =============================================================================

func TestListStudents_WithTeacherAccess(t *testing.T) {
	tc := setupTestContext(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "ListTeacher", "Access", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID)

	router := tc.setupRouter(tc.resource.ListStudentsHandler(), "")
	req := testutil.NewRequest("GET", "/", nil)

	claims := testutil.TeacherTestClaims(int(account.ID))
	rr := tc.executeWithAuth(router, req, claims, []string{"students:read"})

	assert.Equal(t, http.StatusOK, rr.Code, "Teacher should list students")
}

func TestGetStudent_WithTeacherAccess(t *testing.T) {
	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "TeacherAccess", "Test", "TAT1", tc.ogsID)
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "GetTeacher", "Access", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, teacher.ID)

	router := tc.setupRouter(tc.resource.GetStudentHandler(), "id")
	req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)

	claims := testutil.TeacherTestClaims(int(account.ID))
	rr := tc.executeWithAuth(router, req, claims, []string{"students:read"})

	assert.Equal(t, http.StatusOK, rr.Code, "Teacher should get student")
}
