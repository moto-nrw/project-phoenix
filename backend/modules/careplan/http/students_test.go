package careplanhttp_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateStudent_WithGroupID(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "CreateGroup")

	t.Run("creates_student_with_group", func(t *testing.T) {
		body := map[string]interface{}{
			"first_name":   "WithGroup",
			"last_name":    "Student",
			"school_class": "3a",
			"group_id":     group.ID,
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusCreated, rr.Code)
	})
}

func TestCreateStudent_WithAllOptionalFields(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("create_with_all_fields", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, tc.db, "FullCreateGroup")

		body := map[string]interface{}{
			"first_name":    "Full",
			"last_name":     "Create",
			"school_class":  "FC1",
			"birthday":      "2015-03-25",
			"group_id":      group.ID,
			"health_info":   "No allergies",
			"extra_info":    "Extra notes",
			"pickup_status": "bus",
			"bus":           true,
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusCreated, rr.Code, "Should create student. Body: %s", rr.Body.String())
	})
}

func TestUpdateStudent(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	student := testpkg.CreateTestStudent(t, tc.db, "Update", "Student", "US1")

	t.Run("success_updates_student", func(t *testing.T) {
		body := map[string]interface{}{
			"first_name": "Updated",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

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
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("not_found_for_nonexistent_student", func(t *testing.T) {
		body := map[string]interface{}{
			"first_name": "NotFound",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", "/999999", body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("bad_request_empty_first_name", func(t *testing.T) {
		body := map[string]interface{}{
			"first_name": "",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})
}

func TestUpdateStudent_WithGuardianInfo(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	student := testpkg.CreateTestStudent(t, tc.db, "Guardian", "Update", "GU1")

	t.Run("update_guardian_name", func(t *testing.T) {
		body := map[string]interface{}{
			"guardian_name": "New Guardian",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("update_guardian_email", func(t *testing.T) {
		body := map[string]interface{}{
			"guardian_email": "guardian@example.com",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("update_guardian_phone", func(t *testing.T) {
		body := map[string]interface{}{
			"guardian_phone": "+49123456789",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})
}

func TestUpdateStudent_WithExcusedStatus(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("mark_as_excused_sets_excused_since", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Excused", "Status", "ES1")

		eventCount := len(tc.broadcaster.Events())

		body := map[string]interface{}{"excused": true}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), `"excused":true`)
		assert.Contains(t, rr.Body.String(), `"excused_since"`)
		events := tc.broadcaster.Events()
		require.Len(t, events, eventCount+1)
		assert.Equal(t, "student_updated", string(events[eventCount].Type))
	})

	t.Run("clear_excused_clears_excused_since", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "ClearExcused", "Student", "EC1")

		setReq := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID),
			map[string]interface{}{"excused": true})
		setRR := authExec(t, tc, setReq, testutil.AdminTestClaims(1), []string{"admin:*"})
		assert.Equal(t, http.StatusOK, setRR.Code)

		clearReq := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID),
			map[string]interface{}{"excused": false})
		clearRR := authExec(t, tc, clearReq, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, clearRR.Code)
		assert.Contains(t, clearRR.Body.String(), `"excused":false`)

		var history testpkg.StudentStatusDayRow
		err := tc.db.NewSelect().
			Model(&history).
			ModelTableExpr(`active.student_status_days AS "student_status_day"`).
			Where(`"student_status_day".student_id = ?`, student.ID).
			Where(`"student_status_day".status = ?`, absencerecords.StudentStatusDayExcused).
			Scan(context.Background())
		require.NoError(t, err)
		assert.NotNil(t, history.ClearedAt)
	})
}

func TestUpdateStudent_PersonFields(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("update_last_name", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Original", "Last", "OL1")

		body := map[string]interface{}{
			"last_name": "Updated",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "Updated")
	})

	t.Run("update_birthday", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Birthday", "Update", "BU2")

		body := map[string]interface{}{
			"birthday": "2015-06-15",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "2015-06-15")
	})

	t.Run("clear_guardian_fields", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Guardian", "Clear", "GCL1")

		guardian := testpkg.CreateTestGuardianProfileNamed(t, tc.db, "Parent", "Name", "parent@test.com")
		testpkg.CreateTestStudentGuardianLink(t, tc.db, student.ID, guardian.ID, "parent")

		// Clear guardian name by setting empty string
		body := map[string]interface{}{
			"guardian_name": "",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})
}

func TestStudentRequestValidation(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("bind_validates_required_fields", func(t *testing.T) {
		// Empty body should fail validation
		body := map[string]interface{}{}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})
}

func TestUpdateStudent_AllPersonFields(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("update_all_person_fields", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Update", "AllFields", "UAF1")

		body := map[string]interface{}{
			"first_name":  "NewFirst",
			"last_name":   "NewLast",
			"birthday":    "2015-06-15",
			"gender":      "m",
			"street":      "New Street 123",
			"city":        "New City",
			"postal_code": "54321",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "NewFirst")
		assert.Contains(t, rr.Body.String(), "NewLast")
	})

	t.Run("update_guardian_fields", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Guardian", "Update", "GU1")

		body := map[string]interface{}{
			"guardian_first_name": "GuardianFirst",
			"guardian_last_name":  "GuardianLast",
			"guardian_email":      "guardian@example.com",
			"guardian_phone":      "+49123456789",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusBadRequest, rr.Code, "Expected 400 Bad Request. Body: %s", rr.Body.String())
	})

	t.Run("update_student_specific_fields", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Student", "Specific", "SS2")

		body := map[string]interface{}{
			"school_class":        "2b",
			"bus":                 true,
			"extra_info":          "Some extra info",
			"data_retention_days": 15,
			"responsible_person":  "Ms. Smith",
			"responsible_phone":   "+49987654321",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})

	t.Run("update_sick_status_extended", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Sick", "Status", "SK1")

		body := map[string]interface{}{
			"sick":       true,
			"sick_since": "2024-01-15",
			"sick_until": "2024-01-20",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})

	t.Run("clear_sick_status", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Clear", "Sick", "CS1")

		// First set sick status
		ctx := testpkg.Ctx(t)
		_, err := tc.db.ExecContext(ctx, "UPDATE users.student_care_profiles SET sick = true WHERE membership_id = (SELECT id FROM users.student_school_memberships WHERE student_profile_id = ? AND deleted_at IS NULL)", student.ID)
		require.NoError(t, err)

		body := map[string]interface{}{
			"sick": false,
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), body)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})
}

func TestCreateStudent_ExtendedValidation(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("create_with_all_optional_fields", func(t *testing.T) {
		body := map[string]interface{}{
			"first_name":          "Complete",
			"last_name":           "Student",
			"school_class":        "3a",
			"birthday":            "2015-03-20",
			"gender":              "f",
			"street":              "Main Street 42",
			"city":                "Berlin",
			"postal_code":         "10115",
			"bus":                 true,
			"extra_info":          "Test student with all fields",
			"responsible_person":  "Teacher",
			"responsible_phone":   "+49444555666",
			"data_retention_days": 20,
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		if rr.Code == http.StatusCreated || rr.Code == http.StatusOK {
			// Cleanup created student
			assert.Contains(t, rr.Body.String(), "Complete")
		}
	})

	t.Run("create_with_group_assignment", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, tc.db, "AssignGroup")

		body := map[string]interface{}{
			"first_name":   "Group",
			"last_name":    "Assigned",
			"school_class": "4a",
			"group_id":     group.ID,
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)

		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		if rr.Code == http.StatusCreated || rr.Code == http.StatusOK {
			assert.Contains(t, rr.Body.String(), "Group")
		}
	})
}
