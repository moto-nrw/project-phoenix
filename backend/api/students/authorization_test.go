package students_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// setStudentDataScope overrides gdpr.student_data_scope for tenant 1 and
// registers a cleanup to reset it at the end of the test.
func setStudentDataScope(t *testing.T, tc *testContext, scope string) {
	t.Helper()
	ctx := testpkg.TenantContext(1)
	err := tc.services.Settings.SetValue(ctx, configModel.KeyStudentDataScope, scope, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = tc.services.Settings.ResetValue(ctx, configModel.KeyStudentDataScope, nil, nil)
	})
}

// =============================================================================
// Authorization Tests (Non-Admin Access)
// =============================================================================

func TestStudentAuthorization_NonAdminAccess(t *testing.T) {
	tc := setupTestContext(t)

	// Create teacher, group, and student
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Auth", "Teacher")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "AuthTestGroup")
	student := testpkg.CreateTestStudent(t, tc.db, "Auth", "Student", "AT1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID, group.ID, student.ID)

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

	t.Run("staff_without_permission_cannot_update", func(t *testing.T) {
		// Create a staff member that does not supervise the student's group
		otherStaff, otherAccount := testpkg.CreateTestStaffWithAccount(t, tc.db, "Other", "Staff")
		defer testpkg.CleanupActivityFixtures(t, tc.db, otherStaff.ID)

		body := map[string]interface{}{
			"first_name": "Updated",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		claims := testutil.TeacherTestClaims(int(otherAccount.ID))
		rr := authExec(t, tc, req, claims, []string{"users:update"})

		// Non-supervisor should be forbidden
		testutil.AssertForbidden(t, rr)
	})

	t.Run("staff_without_permission_cannot_delete", func(t *testing.T) {
		otherStaff, otherAccount := testpkg.CreateTestStaffWithAccount(t, tc.db, "Delete", "Restricted")
		defer testpkg.CleanupActivityFixtures(t, tc.db, otherStaff.ID)

		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d", student.ID), nil)

		claims := testutil.TeacherTestClaims(int(otherAccount.ID))
		rr := authExec(t, tc, req, claims, []string{"users:delete"})

		// Non-supervisor should be forbidden
		testutil.AssertForbidden(t, rr)
	})
}

func TestStudentAuthorization_StudentWithoutGroup(t *testing.T) {
	tc := setupTestContext(t)

	// Create student without group assignment
	student := testpkg.CreateTestStudent(t, tc.db, "NoGroup", "Student", "NG1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "NoGroup", "Staff")
	defer testpkg.CleanupActivityFixtures(t, tc.db, staff.ID)

	t.Run("non_admin_cannot_update_student_without_group", func(t *testing.T) {
		body := map[string]interface{}{
			"first_name": "Updated",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		claims := testutil.TeacherTestClaims(int(account.ID))
		rr := authExec(t, tc, req, claims, []string{"users:update"})

		// Only administrators can update students without groups
		testutil.AssertForbidden(t, rr)
		assert.Contains(t, rr.Body.String(), "administrator")
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
	tc := setupTestContext(t)

	t.Run("admin_sees_all_fields", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Full", "Access", "FA1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		// Update student with additional fields using raw SQL - use ? placeholders
		ctx := testpkg.TenantContext(1)
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

	t.Run("non_supervisor_sees_limited_fields", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Limited", "Access", "LA1")
		staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Limited", "Staff")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, staff.ID)

		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)

		claims := testutil.TeacherTestClaims(int(account.ID))
		rr := authExec(t, tc, req, claims, []string{"users:read"})

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
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Detail", "Teacher")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "DetailGroup")
	student := testpkg.CreateTestStudent(t, tc.db, "Detail", "Student", "DT1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID, group.ID, student.ID)

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
	tc := setupTestContext(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "ListTeacher", "Access")
	defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID)

	req := testutil.NewRequest("GET", "/", nil)

	claims := testutil.TeacherTestClaims(int(account.ID))
	rr := authExec(t, tc, req, claims, []string{"users:read"})

	assert.Equal(t, http.StatusOK, rr.Code, "Teacher should list students")
}

func TestGetStudent_WithTeacherAccess(t *testing.T) {
	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "TeacherAccess", "Test", "TAT1")
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "GetTeacher", "Access")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, teacher.ID)

	req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)

	claims := testutil.TeacherTestClaims(int(account.ID))
	rr := authExec(t, tc, req, claims, []string{"users:read"})

	assert.Equal(t, http.StatusOK, rr.Code, "Teacher should get student")
}

// =============================================================================
// Student Data Scope Setting Tests (gdpr.student_data_scope)
// =============================================================================

// TestStudentDataScope_AllStaff_GrantsFullReadAccess verifies that when a tenant
// sets gdpr.student_data_scope = all_staff, any authenticated staff member with
// users:read permission sees the full student profile, not just the redacted view.
func TestStudentDataScope_AllStaff_GrantsFullReadAccess(t *testing.T) {
	tc := setupTestContext(t)
	setStudentDataScope(t, tc, configModel.StudentDataScopeAllStaff)

	// Create a student assigned to a group, plus an unrelated staff member
	// who does NOT supervise that group. Under the default scope this staff
	// would only see the limited/redacted fields; with all_staff they should
	// see the full profile.
	group := testpkg.CreateTestEducationGroup(t, tc.db, "DataScopeGroup")
	student := testpkg.CreateTestStudent(t, tc.db, "Scope", "Student", "DS1")
	otherStaff, otherAccount := testpkg.CreateTestStaffWithAccount(t, tc.db, "Other", "Staff")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID, student.ID, otherStaff.ID)

	testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)

	req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)
	claims := testutil.TeacherTestClaims(int(otherAccount.ID))
	rr := authExec(t, tc, req, claims, []string{"users:read"})

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var resp struct {
		Data struct {
			HasFullAccess  bool `json:"has_full_access"`
			HasWriteAccess bool `json:"has_write_access"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.True(t, resp.Data.HasFullAccess, "all_staff scope should grant full read access to non-supervisors")
	assert.False(t, resp.Data.HasWriteAccess, "all_staff scope should NOT grant write access to non-supervisors")
}

// TestStudentDataScope_GroupSupervisorsOnly_RedactsForNonSupervisor verifies that
// the default scope still redacts non-supervisor access. This is the pre-existing
// behavior — the test locks it in to catch regressions from the scope rewrite.
func TestStudentDataScope_GroupSupervisorsOnly_RedactsForNonSupervisor(t *testing.T) {
	tc := setupTestContext(t)
	// Explicitly set the default value so the test is independent of
	// whatever the tenant happens to have persisted in the DB.
	setStudentDataScope(t, tc, configModel.StudentDataScopeGroupSupervisorsOnly)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "DefaultScopeGroup")
	student := testpkg.CreateTestStudent(t, tc.db, "Default", "Student", "DS2")
	otherStaff, otherAccount := testpkg.CreateTestStaffWithAccount(t, tc.db, "Unrelated", "Staff")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID, student.ID, otherStaff.ID)

	testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)

	req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)
	claims := testutil.TeacherTestClaims(int(otherAccount.ID))
	rr := authExec(t, tc, req, claims, []string{"users:read"})

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var resp struct {
		Data struct {
			HasFullAccess  bool `json:"has_full_access"`
			HasWriteAccess bool `json:"has_write_access"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.False(t, resp.Data.HasFullAccess, "group_supervisors_only should redact non-supervisors")
	assert.False(t, resp.Data.HasWriteAccess, "group_supervisors_only should not grant write access to non-supervisors")
}

// TestStudentDataScope_AllStaff_GrantsAccessToGrouplessStudent covers the edge
// case where a student has no group assigned. Under the default scope, the only
// way to see full data is to be an admin. With all_staff, any staff should see it.
func TestStudentDataScope_AllStaff_GrantsAccessToGrouplessStudent(t *testing.T) {
	tc := setupTestContext(t)
	setStudentDataScope(t, tc, configModel.StudentDataScopeAllStaff)

	student := testpkg.CreateTestStudent(t, tc.db, "NoGroup", "Scoped", "DS3")
	staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "AnyStaff", "Member")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, staff.ID)

	req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)
	claims := testutil.TeacherTestClaims(int(account.ID))
	rr := authExec(t, tc, req, claims, []string{"users:read"})

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var resp struct {
		Data struct {
			HasFullAccess  bool `json:"has_full_access"`
			HasWriteAccess bool `json:"has_write_access"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.True(t, resp.Data.HasFullAccess, "all_staff scope should grant access even to students without groups")
	assert.False(t, resp.Data.HasWriteAccess, "all_staff scope should NOT grant write access even for groupless students")
}

// TestStudentDataScope_DoesNotAffectWrites confirms that flipping the scope to
// all_staff does NOT relax write authorization — update/delete remain restricted
// to group supervisors. This is the critical invariant of this feature.
func TestStudentDataScope_DoesNotAffectWrites(t *testing.T) {
	tc := setupTestContext(t)
	setStudentDataScope(t, tc, configModel.StudentDataScopeAllStaff)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "WriteScopeGroup")
	student := testpkg.CreateTestStudent(t, tc.db, "Write", "Student", "DS4")
	otherStaff, otherAccount := testpkg.CreateTestStaffWithAccount(t, tc.db, "WriteDenied", "Staff")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID, student.ID, otherStaff.ID)

	testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)

	// A non-supervisor attempts to update → must still be forbidden even
	// though the read scope is fully open.
	body := map[string]interface{}{"first_name": "ShouldNotChange"}
	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
	claims := testutil.TeacherTestClaims(int(otherAccount.ID))
	rr := authExec(t, tc, req, claims, []string{"users:update"})

	testutil.AssertForbidden(t, rr)
}

// TestStudentDataScope_AllStaff_ListReturnsFullAccess verifies that the student
// LIST endpoint uses the per-request access context, and with all_staff scope
// non-supervisor staff see sensitive fields (like extra_info) that would normally
// be redacted. This covers studentAccessContext.hasFullAccessToStudent.
func TestStudentDataScope_AllStaff_ListReturnsFullAccess(t *testing.T) {
	tc := setupTestContext(t)
	setStudentDataScope(t, tc, configModel.StudentDataScopeAllStaff)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "ListScopeGroup")
	student := testpkg.CreateTestStudent(t, tc.db, "ListScope", "Student", "DS5")
	otherStaff, otherAccount := testpkg.CreateTestStaffWithAccount(t, tc.db, "ListViewer", "Staff")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID, student.ID, otherStaff.ID)

	testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)

	// Add a sensitive field that only appears when hasFullAccessToStudent returns true.
	// extra_info is a supervisor-visible field redacted from non-supervisors under the
	// default scope. With all_staff scope it should appear for any staff viewer.
	ctx := testpkg.TenantContext(1)
	_, err := tc.db.ExecContext(ctx,
		"UPDATE users.students SET extra_info = ? WHERE id = ?",
		"LIST_SCOPE_MARKER_d5s", student.ID)
	require.NoError(t, err)

	// Filter by search query so we only get our specific student back and
	// aren't fooled by leftover test data from other tests in the same run.
	req := testutil.NewRequest("GET", "/?search=ListScope", nil)
	claims := testutil.TeacherTestClaims(int(otherAccount.ID))
	rr := authExec(t, tc, req, claims, []string{"users:read"})

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
	// The extra_info field should only be present when the viewer has full access.
	// Under all_staff scope, this non-supervisor must see it.
	assert.Contains(t, rr.Body.String(), "LIST_SCOPE_MARKER_d5s",
		"all_staff scope should expose extra_info to non-supervisors in list responses")
}

// TestStudentDataScope_AllStaff_GroupRoomAccessible verifies that the
// in-group-room endpoint works for non-supervisor staff when scope is all_staff.
func TestStudentDataScope_AllStaff_GroupRoomAccessible(t *testing.T) {
	tc := setupTestContext(t)
	setStudentDataScope(t, tc, configModel.StudentDataScopeAllStaff)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "GroupRoomScopeGroup")
	student := testpkg.CreateTestStudent(t, tc.db, "GroupRoom", "Scoped", "DS6")
	otherStaff, otherAccount := testpkg.CreateTestStaffWithAccount(t, tc.db, "GroupRoom", "Viewer")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID, student.ID, otherStaff.ID)

	testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)

	req := testutil.NewRequest("GET", fmt.Sprintf("/%d/in-group-room", student.ID), nil)
	claims := testutil.TeacherTestClaims(int(otherAccount.ID))
	rr := authExec(t, tc, req, claims, []string{"users:read"})

	// Under the default scope this would be 403; with all_staff it should be 200.
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
}

// TestStudentDataScope_GroupSupervisorsOnly_GroupRoomForbidden verifies the
// default behavior — a non-supervisor accessing the in-group-room endpoint is
// forbidden. Locks in the pre-existing restriction to catch regressions.
func TestStudentDataScope_GroupSupervisorsOnly_GroupRoomForbidden(t *testing.T) {
	tc := setupTestContext(t)
	setStudentDataScope(t, tc, configModel.StudentDataScopeGroupSupervisorsOnly)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "GroupRoomForbiddenGroup")
	student := testpkg.CreateTestStudent(t, tc.db, "GroupRoom", "Forbidden", "DS7")
	otherStaff, otherAccount := testpkg.CreateTestStaffWithAccount(t, tc.db, "GroupRoom", "Denied")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID, student.ID, otherStaff.ID)

	testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)

	req := testutil.NewRequest("GET", fmt.Sprintf("/%d/in-group-room", student.ID), nil)
	claims := testutil.TeacherTestClaims(int(otherAccount.ID))
	rr := authExec(t, tc, req, claims, []string{"users:read"})

	testutil.AssertForbidden(t, rr)
}
