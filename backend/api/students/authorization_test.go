package students_test

import (
	"encoding/json"
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

// =============================================================================
// Student Full Access Tests
// =============================================================================

func TestStudentResponse_FullAccess(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("admin_sees_all_fields", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Full", "Access", "FA1")

		// Update student with additional fields using raw SQL - use ? placeholders
		ctx := testpkg.Ctx(t)
		_, err := tc.db.ExecContext(ctx,
			"UPDATE users.students SET guardian_email = ?, guardian_phone = ?, extra_info = ? WHERE id = ?",
			"guardian@example.com", "+49123456789", "Important notes", student.ID)
		require.NoError(t, err)

		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
		// Admin should see sensitive fields
		body := rr.Body.String()
		assert.Contains(t, body, "guardian_email", "Admin should see guardian email")
	})

	t.Run("staff_outside_the_group_sees_full_access_fields", func(t *testing.T) {
		// #2329: read access is tenant-wide for staff, so the address block —
		// gated on full access — reaches a staff member who supervises nothing.
		student := testpkg.CreateTestStudent(t, tc.db, "Limited", "Access", "LA1")
		_, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Limited", "Staff")

		ctx := testpkg.Ctx(t)
		_, err := tc.db.ExecContext(ctx,
			"UPDATE users.students SET address_street = ? WHERE id = ?",
			"Vollzugriffweg 1", student.ID)
		require.NoError(t, err)

		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)

		claims := testutil.TeacherTestClaims(int(account.ID))
		rr := authExec(t, tc, req, claims, []string{"users:read"})

		require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

		var resp struct {
			Data struct {
				HasFullAccess bool `json:"has_full_access"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		assert.True(t, resp.Data.HasFullAccess, "any verified staff member has full read access")
		assert.Contains(t, rr.Body.String(), "Vollzugriffweg 1")
	})

	t.Run("account_without_staff_record_stays_redacted", func(t *testing.T) {
		// Guest/guardian accounts hold users:read but no staff record, so the
		// full-access fields (address, RFID tag, photo URL) stay out.
		student := testpkg.CreateTestStudent(t, tc.db, "Redacted", "Access", "RA1")
		guest := testpkg.CreateTestAccount(t, tc.db, "students-redacted-guest@example.com")

		ctx := testpkg.Ctx(t)
		_, err := tc.db.ExecContext(ctx,
			"UPDATE users.students SET address_street = ? WHERE id = ?",
			"Geheimstrasse 9", student.ID)
		require.NoError(t, err)

		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)

		claims := testutil.TeacherTestClaims(int(guest.ID))
		rr := authExec(t, tc, req, claims, []string{"users:read"})

		require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

		var resp struct {
			Data struct {
				HasFullAccess  bool `json:"has_full_access"`
				HasWriteAccess bool `json:"has_write_access"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		assert.False(t, resp.Data.HasFullAccess)
		assert.False(t, resp.Data.HasWriteAccess)
		assert.NotContains(t, rr.Body.String(), "Geheimstrasse 9",
			"the address block must stay out of a redacted response")
	})
}

// =============================================================================
// Student Detail With Group Tests
// =============================================================================

func TestGetStudentDetail_WithGroup(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	// Create teacher, group, and student
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Detail", "Teacher")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "DetailGroup")
	student := testpkg.CreateTestStudent(t, tc.db, "Detail", "Student", "DT1")

	// Assign teacher and student to group
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)
	testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)

	t.Run("supervisor_gets_full_access", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)

		claims := testutil.TeacherTestClaims(int(account.ID))
		rr := authExec(t, tc, req, claims, []string{"users:read"})

		assert.Equal(t, http.StatusOK, rr.Code, "Supervisor should get student")
		// Check response includes group name
		assert.Contains(t, rr.Body.String(), "DetailGroup")
	})
}

// =============================================================================
// Teacher Access Tests
// =============================================================================

func TestListStudents_WithTeacherAccess(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "ListTeacher", "Access")

	req := testutil.NewRequest("GET", "/", nil)

	claims := testutil.TeacherTestClaims(int(account.ID))
	rr := authExec(t, tc, req, claims, []string{"users:read"})

	assert.Equal(t, http.StatusOK, rr.Code, "Teacher should list students")
}

func TestGetStudent_WithTeacherAccess(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	student := testpkg.CreateTestStudent(t, tc.db, "TeacherAccess", "Test", "TAT1")
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "GetTeacher", "Access")

	req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)

	claims := testutil.TeacherTestClaims(int(account.ID))
	rr := authExec(t, tc, req, claims, []string{"users:read"})

	assert.Equal(t, http.StatusOK, rr.Code, "Teacher should get student")
}
