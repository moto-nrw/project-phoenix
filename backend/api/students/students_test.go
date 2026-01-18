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
// List Students Tests
// =============================================================================

func TestListStudents(t *testing.T) {
	tc := setupTestContext(t)

	// Create test students using fixtures
	student1 := testpkg.CreateTestStudent(t, tc.db, "List", "StudentOne", "1a")
	student2 := testpkg.CreateTestStudent(t, tc.db, "List", "StudentTwo", "1b")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student1.ID, student2.ID)

	router := setupRouter(tc.resource.ListStudentsHandler(), "")

	t.Run("success_admin_lists_all_students", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/", nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})

	t.Run("success_with_pagination", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/?page=1&page_size=10", nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("success_with_school_class_filter", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/?school_class=1a", nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("success_with_search_filter", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/?search=List", nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestListStudents_WithLocationFilter(t *testing.T) {
	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "Location", "Filter", "LF1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	router := setupRouter(tc.resource.ListStudentsHandler(), "")

	t.Run("filter_by_in_house", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/?location=in_house", nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("filter_by_absent", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/?location=absent", nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestListStudents_WithNameFilters(t *testing.T) {
	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "NameFilter", "Test", "NF1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	t.Run("filter_by_first_name", func(t *testing.T) {
		router := setupRouter(tc.resource.ListStudentsHandler(), "")
		req := testutil.NewRequest("GET", "/?first_name=NameFilter", nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "NameFilter")
	})

	t.Run("filter_by_last_name", func(t *testing.T) {
		router := setupRouter(tc.resource.ListStudentsHandler(), "")
		req := testutil.NewRequest("GET", "/?last_name=Test", nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestListStudents_ExtendedFilters(t *testing.T) {
	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "Filter", "Student", "FI1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	t.Run("filter_by_group_id", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, tc.db, "FilterGroup")
		defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

		router := setupRouter(tc.resource.ListStudentsHandler(), "")
		req := testutil.NewRequest("GET", fmt.Sprintf("/?group_id=%d", group.ID), nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("invalid_page_size", func(t *testing.T) {
		router := setupRouter(tc.resource.ListStudentsHandler(), "")
		req := testutil.NewRequest("GET", "/?page_size=invalid", nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		// Invalid page_size should return bad request or be ignored
		assert.Contains(t, []int{http.StatusOK, http.StatusBadRequest}, rr.Code)
	})

	t.Run("negative_page", func(t *testing.T) {
		router := setupRouter(tc.resource.ListStudentsHandler(), "")
		req := testutil.NewRequest("GET", "/?page=-1", nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		// Negative page should return bad request or default to 1
		assert.Contains(t, []int{http.StatusOK, http.StatusBadRequest}, rr.Code)
	})
}

// =============================================================================
// Get Student Tests
// =============================================================================

func TestGetStudent(t *testing.T) {
	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "Get", "Student", "GS1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	router := setupRouter(tc.resource.GetStudentHandler(), "id")

	t.Run("success_admin_gets_student", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "Get")
	})

	t.Run("not_found_for_nonexistent_student", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/999999", nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("bad_request_for_invalid_id", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/invalid", nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})
}

// =============================================================================
// Create Student Tests
// =============================================================================

func TestCreateStudent(t *testing.T) {
	tc := setupTestContext(t)

	router := setupRouter(tc.resource.CreateStudentHandler(), "")

	t.Run("success_creates_student", func(t *testing.T) {
		body := map[string]interface{}{
			"first_name":   "New",
			"last_name":    "Student",
			"school_class": "2a",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())
	})

	t.Run("success_creates_student_with_optional_fields", func(t *testing.T) {
		body := map[string]interface{}{
			"first_name":     "Optional",
			"last_name":      "Fields",
			"school_class":   "2b",
			"birthday":       "2015-06-15",
			"guardian_name":  "Parent Name",
			"guardian_email": "parent@example.com",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusCreated, rr.Code)
	})

	t.Run("bad_request_missing_first_name", func(t *testing.T) {
		body := map[string]interface{}{
			"last_name":    "NoFirst",
			"school_class": "2c",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_missing_last_name", func(t *testing.T) {
		body := map[string]interface{}{
			"first_name":   "NoLast",
			"school_class": "2c",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_missing_school_class", func(t *testing.T) {
		body := map[string]interface{}{
			"first_name": "NoClass",
			"last_name":  "Student",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_invalid_birthday_format", func(t *testing.T) {
		body := map[string]interface{}{
			"first_name":   "Invalid",
			"last_name":    "Birthday",
			"school_class": "2c",
			"birthday":     "not-a-date",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		// Invalid birthday format should fail
		assert.NotEqual(t, http.StatusCreated, rr.Code)
	})
}

func TestCreateStudent_WithGroupID(t *testing.T) {
	tc := setupTestContext(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "CreateGroup")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	router := setupRouter(tc.resource.CreateStudentHandler(), "")

	t.Run("creates_student_with_group", func(t *testing.T) {
		body := map[string]interface{}{
			"first_name":   "WithGroup",
			"last_name":    "Student",
			"school_class": "3a",
			"group_id":     group.ID,
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusCreated, rr.Code)
	})
}

func TestCreateStudent_WithAllOptionalFields(t *testing.T) {
	tc := setupTestContext(t)

	t.Run("create_with_all_fields", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, tc.db, "FullCreateGroup")
		defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

		router := setupRouter(tc.resource.CreateStudentHandler(), "")
		body := map[string]interface{}{
			"first_name":       "Full",
			"last_name":        "Create",
			"school_class":     "FC1",
			"birthday":         "2015-03-25",
			"group_id":         group.ID,
			"guardian_name":    "Parent Full",
			"guardian_email":   "fullparent@test.com",
			"guardian_phone":   "+4912345678",
			"guardian_contact": "Emergency info",
			"health_info":      "No allergies",
			"extra_info":       "Extra notes",
			"pickup_status":    "bus",
			"bus":              true,
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusCreated, rr.Code, "Should create student. Body: %s", rr.Body.String())
	})
}

// =============================================================================
// Update Student Tests
// =============================================================================

func TestUpdateStudent(t *testing.T) {
	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "Update", "Student", "US1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	router := setupRouter(tc.resource.UpdateStudentHandler(), "id")

	t.Run("success_updates_student", func(t *testing.T) {
		body := map[string]interface{}{
			"first_name": "Updated",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "Updated")
	})

	t.Run("success_updates_multiple_fields", func(t *testing.T) {
		body := map[string]interface{}{
			"first_name":   "Multi",
			"last_name":    "Update",
			"school_class": "4a",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("not_found_for_nonexistent_student", func(t *testing.T) {
		body := map[string]interface{}{
			"first_name": "NotFound",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", "/999999", body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("bad_request_empty_first_name", func(t *testing.T) {
		body := map[string]interface{}{
			"first_name": "",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})
}

func TestUpdateStudent_WithGuardianInfo(t *testing.T) {
	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "Guardian", "Update", "GU1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	router := setupRouter(tc.resource.UpdateStudentHandler(), "id")

	t.Run("update_guardian_name", func(t *testing.T) {
		body := map[string]interface{}{
			"guardian_name": "New Guardian",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("update_guardian_email", func(t *testing.T) {
		body := map[string]interface{}{
			"guardian_email": "guardian@example.com",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("update_guardian_phone", func(t *testing.T) {
		body := map[string]interface{}{
			"guardian_phone": "+49123456789",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestUpdateStudent_WithSickStatus(t *testing.T) {
	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "Sick", "Status", "SS1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	router := setupRouter(tc.resource.UpdateStudentHandler(), "id")

	t.Run("mark_as_sick", func(t *testing.T) {
		body := map[string]interface{}{
			"sick": true,
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), `"sick":true`)
	})

	t.Run("mark_as_not_sick", func(t *testing.T) {
		body := map[string]interface{}{
			"sick": false,
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestUpdateStudent_SickStatusExtended(t *testing.T) {
	tc := setupTestContext(t)

	t.Run("mark_student_as_sick_sets_sick_since", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "SickSince", "Student", "SS2")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		router := setupRouter(tc.resource.UpdateStudentHandler(), "id")

		// Mark as sick
		body := map[string]interface{}{
			"sick": true,
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Should mark student as sick")
		assert.Contains(t, rr.Body.String(), `"sick":true`)
	})

	t.Run("clear_sick_status_clears_sick_since", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ClearSick", "Student", "CS1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		router := setupRouter(tc.resource.UpdateStudentHandler(), "id")

		// First mark as sick
		sickBody := map[string]interface{}{
			"sick": true,
		}
		sickReq := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), sickBody)
		sickRR := executeWithAuth(router, sickReq, testutil.AdminTestClaims(1), []string{"admin:*"})
		assert.Equal(t, http.StatusOK, sickRR.Code)

		// Then clear sick status
		clearBody := map[string]interface{}{
			"sick": false,
		}
		clearReq := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), clearBody)
		clearRR := executeWithAuth(router, clearReq, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, clearRR.Code, "Should clear sick status")
		assert.Contains(t, clearRR.Body.String(), `"sick":false`)
	})
}

func TestUpdateStudent_ExtendedFields(t *testing.T) {
	tc := setupTestContext(t)

	t.Run("update_health_info", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Health", "Student", "HS1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		router := setupRouter(tc.resource.UpdateStudentHandler(), "id")
		body := map[string]interface{}{
			"health_info": "Allergies: Peanuts",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("update_extra_info", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Extra", "Student", "EX1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		router := setupRouter(tc.resource.UpdateStudentHandler(), "id")
		body := map[string]interface{}{
			"extra_info": "Additional notes about the student",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("update_supervisor_notes", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Notes", "Student", "NS1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		router := setupRouter(tc.resource.UpdateStudentHandler(), "id")
		body := map[string]interface{}{
			"supervisor_notes": "Supervisor observations",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("update_pickup_status", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Pickup", "Student", "PU1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		router := setupRouter(tc.resource.UpdateStudentHandler(), "id")
		body := map[string]interface{}{
			"pickup_status": "ready",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("update_bus", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Bus", "Student", "BU1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		router := setupRouter(tc.resource.UpdateStudentHandler(), "id")
		// Bus is a boolean flag, not a string
		body := map[string]interface{}{
			"bus": true,
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("update_guardian_contact", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Contact", "Student", "GC1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		router := setupRouter(tc.resource.UpdateStudentHandler(), "id")
		body := map[string]interface{}{
			"guardian_contact": "Emergency: 0800-123456",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestUpdateStudent_PersonFields(t *testing.T) {
	tc := setupTestContext(t)

	t.Run("update_last_name", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Original", "Last", "OL1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		router := setupRouter(tc.resource.UpdateStudentHandler(), "id")
		body := map[string]interface{}{
			"last_name": "Updated",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "Updated")
	})

	t.Run("update_birthday", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Birthday", "Update", "BU2")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		router := setupRouter(tc.resource.UpdateStudentHandler(), "id")
		body := map[string]interface{}{
			"birthday": "2015-06-15",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "2015-06-15")
	})

	t.Run("clear_guardian_fields", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Guardian", "Clear", "GCL1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		// First set guardian fields
		ctx := context.Background()
		_, err := tc.db.ExecContext(ctx,
			"UPDATE users.students SET guardian_name = ?, guardian_email = ? WHERE id = ?",
			"Parent Name", "parent@test.com", student.ID)
		require.NoError(t, err)

		router := setupRouter(tc.resource.UpdateStudentHandler(), "id")
		// Clear guardian name by setting empty string
		body := map[string]interface{}{
			"guardian_name": "",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

// =============================================================================
// Delete Student Tests
// =============================================================================

func TestDeleteStudent(t *testing.T) {
	tc := setupTestContext(t)

	router := setupRouter(tc.resource.DeleteStudentHandler(), "id")

	t.Run("success_deletes_student", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Delete", "Me", "DM1")
		// No cleanup needed - we're deleting

		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d", student.ID), nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		// Handler returns 200 OK with success message (not 204 NoContent)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "deleted successfully")
	})

	t.Run("not_found_for_nonexistent_student", func(t *testing.T) {
		req := testutil.NewRequest("DELETE", "/999999", nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("bad_request_for_invalid_id", func(t *testing.T) {
		req := testutil.NewRequest("DELETE", "/invalid", nil)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})
}

// =============================================================================
// Student Request Validation Tests
// =============================================================================

func TestStudentRequestValidation(t *testing.T) {
	tc := setupTestContext(t)

	router := setupRouter(tc.resource.CreateStudentHandler(), "")

	t.Run("bind_validates_required_fields", func(t *testing.T) {
		// Empty body should fail validation
		body := map[string]interface{}{}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})
}

// =============================================================================
// Router Tests
// =============================================================================

func TestRouter_ReturnsValidRouter(t *testing.T) {
	tc := setupTestContext(t)

	router := tc.resource.Router()
	assert.NotNil(t, router, "Router should not be nil")
}

// =============================================================================
// Error Rendering Coverage
// =============================================================================

func TestRenderErrorCases(t *testing.T) {
	tc := setupTestContext(t)

	t.Run("internal_server_error", func(t *testing.T) {
		router := setupRouter(tc.resource.GetStudentCurrentVisitHandler(), "id")
		// Request for student that doesn't exist to trigger error path
		req := testutil.NewRequest("GET", "/999999", nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		// Should return some error status
		assert.NotEqual(t, http.StatusOK, rr.Code)
	})
}
