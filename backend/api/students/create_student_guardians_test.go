package students_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
)

// createStudentResponse is the minimal envelope shape needed to read the IDs of
// a freshly created student.
type createStudentResponse struct {
	Data struct {
		ID       int64 `json:"id"`
		PersonID int64 `json:"person_id"`
	} `json:"data"`
}

// cleanupStudentWithGuardians removes a student and every guardian record linked
// to it, in FK-safe order, so the hermetic test leaves no residue.
func cleanupStudentWithGuardians(t *testing.T, tc *testContext, studentID, personID int64) {
	t.Helper()
	ctx := context.Background()

	var guardianIDs []int64
	if err := tc.db.NewSelect().
		Table("users.students_guardians").
		Column("guardian_profile_id").
		Where("student_id = ?", studentID).
		Scan(ctx, &guardianIDs); err != nil {
		t.Logf("failed to load guardian ids for cleanup: %v", err)
	}

	if _, err := tc.db.NewDelete().
		Table("users.students_guardians").
		Where("student_id = ?", studentID).
		Exec(ctx); err != nil {
		t.Logf("failed to delete student_guardians: %v", err)
	}

	for _, gid := range guardianIDs {
		if _, err := tc.db.NewDelete().
			Table("users.guardian_phone_numbers").
			Where("guardian_profile_id = ?", gid).
			Exec(ctx); err != nil {
			t.Logf("failed to delete guardian phone numbers: %v", err)
		}
		if _, err := tc.db.NewDelete().
			Table("users.guardian_profiles").
			Where("id = ?", gid).
			Exec(ctx); err != nil {
			t.Logf("failed to delete guardian profile: %v", err)
		}
	}

	if _, err := tc.db.NewDelete().Table("users.students").Where("id = ?", studentID).Exec(ctx); err != nil {
		t.Logf("failed to delete student: %v", err)
	}
	if _, err := tc.db.NewDelete().Table("users.persons").Where("id = ?", personID).Exec(ctx); err != nil {
		t.Logf("failed to delete person: %v", err)
	}
}

// TestCreateStudent_WithGuardians verifies a student and its guardians (profile,
// relationship, and phone numbers) are created together in one request.
func TestCreateStudent_WithGuardians(t *testing.T) {
	tc := setupTestContext(t)

	router := setupRouter(tc.resource.CreateStudentHandler(), "")

	body := map[string]interface{}{
		"first_name":   "Guarded",
		"last_name":    "Child",
		"school_class": "1a",
		"guardians": []map[string]interface{}{
			{
				"first_name":           "Erika",
				"last_name":            "Mustermann",
				"email":                "erika.guardian.test@example.com",
				"relationship_type":    "parent",
				"is_primary":           true,
				"can_pickup":           true,
				"is_emergency_contact": true,
				"emergency_priority":   1,
				"phone_numbers": []map[string]interface{}{
					{"phone_number": "0151 2345678", "phone_type": "mobile", "is_primary": true},
				},
			},
		},
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
	rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	require.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())

	var resp createStudentResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.NotZero(t, resp.Data.ID, "expected a student id in the response")

	defer cleanupStudentWithGuardians(t, tc, resp.Data.ID, resp.Data.PersonID)

	ctx := context.Background()

	relCount, err := tc.db.NewSelect().
		Table("users.students_guardians").
		Where("student_id = ?", resp.Data.ID).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, relCount, "expected exactly one student-guardian relationship")

	var guardianID int64
	require.NoError(t, tc.db.NewSelect().
		Table("users.students_guardians").
		Column("guardian_profile_id").
		Where("student_id = ?", resp.Data.ID).
		Scan(ctx, &guardianID))

	var canPickup, isPrimary bool
	require.NoError(t, tc.db.NewSelect().
		Table("users.students_guardians").
		Column("can_pickup").
		Where("student_id = ?", resp.Data.ID).
		Scan(ctx, &canPickup))
	assert.True(t, canPickup, "guardian should be marked as pickup-authorized")
	require.NoError(t, tc.db.NewSelect().
		Table("users.students_guardians").
		Column("is_primary").
		Where("student_id = ?", resp.Data.ID).
		Scan(ctx, &isPrimary))
	assert.True(t, isPrimary, "guardian should be marked primary")

	phoneCount, err := tc.db.NewSelect().
		Table("users.guardian_phone_numbers").
		Where("guardian_profile_id = ?", guardianID).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, phoneCount, "expected exactly one guardian phone number")
}

// TestCreateStudent_GuardianFailureRollsBackStudent verifies the whole creation
// is atomic: an invalid guardian (bad email) aborts the transaction and leaves
// no orphaned person/student behind.
func TestCreateStudent_GuardianFailureRollsBackStudent(t *testing.T) {
	tc := setupTestContext(t)

	router := setupRouter(tc.resource.CreateStudentHandler(), "")

	const firstName = "Rollback"
	const lastName = "Orphan"

	body := map[string]interface{}{
		"first_name":   firstName,
		"last_name":    lastName,
		"school_class": "1b",
		"guardians": []map[string]interface{}{
			{
				"first_name":         "Invalid",
				"last_name":          "Email",
				"email":              "not-a-valid-email",
				"relationship_type":  "parent",
				"emergency_priority": 1,
			},
		},
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
	rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	// Bad guardian input is a client error, not a server error: the handler
	// classifies the ValidationError and returns 400 (not 500), and the
	// surrounding transaction rolls back so no orphaned student survives.
	assert.Equal(t, http.StatusBadRequest, rr.Code,
		"invalid guardian email must return 400. Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), "Erziehungsberechtigte",
		"400 body should name the offending guardian, not a generic server error")

	ctx := context.Background()

	// Safety net in case a prior failed run left residue or the rollback regressed.
	defer func() {
		var personIDs []int64
		if err := tc.db.NewSelect().
			Table("users.persons").
			Column("id").
			Where("first_name = ? AND last_name = ?", firstName, lastName).
			Scan(ctx, &personIDs); err == nil {
			for _, pid := range personIDs {
				if _, err := tc.db.NewDelete().Table("users.students").Where("person_id = ?", pid).Exec(ctx); err != nil {
					t.Logf("cleanup students: %v", err)
				}
				if _, err := tc.db.NewDelete().Table("users.persons").Where("id = ?", pid).Exec(ctx); err != nil {
					t.Logf("cleanup persons: %v", err)
				}
			}
		}
	}()

	personCount, err := tc.db.NewSelect().
		Table("users.persons").
		Where("first_name = ? AND last_name = ?", firstName, lastName).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, personCount, "person must not persist when guardian creation fails (transaction must roll back)")
}

// TestCreateStudent_InvalidGuardianPhoneRollsBackStudent verifies a malformed
// guardian phone number is treated as a client error (400, not 500) and that
// the surrounding transaction rolls back so no orphaned student survives —
// matching the detail page, where a bad phone is a validation error.
func TestCreateStudent_InvalidGuardianPhoneRollsBackStudent(t *testing.T) {
	tc := setupTestContext(t)

	router := setupRouter(tc.resource.CreateStudentHandler(), "")

	const firstName = "PhoneRollback"
	const lastName = "Orphan"

	body := map[string]interface{}{
		"first_name":   firstName,
		"last_name":    lastName,
		"school_class": "1c",
		"guardians": []map[string]interface{}{
			{
				"first_name":         "Valid",
				"last_name":          "Profile",
				"email":              "valid.guardian.phone.test@example.com",
				"relationship_type":  "parent",
				"emergency_priority": 1,
				"phone_numbers": []map[string]interface{}{
					// No digits at all — fails the model's phone format check.
					{"phone_number": "not-a-phone", "phone_type": "mobile", "is_primary": true},
				},
			},
		},
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
	rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	assert.Equal(t, http.StatusBadRequest, rr.Code,
		"invalid guardian phone must return 400. Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), "Telefonnummer",
		"400 body should name the offending phone number")

	ctx := context.Background()

	defer func() {
		var personIDs []int64
		if err := tc.db.NewSelect().
			Table("users.persons").
			Column("id").
			Where("first_name = ? AND last_name = ?", firstName, lastName).
			Scan(ctx, &personIDs); err == nil {
			for _, pid := range personIDs {
				if _, err := tc.db.NewDelete().Table("users.students").Where("person_id = ?", pid).Exec(ctx); err != nil {
					t.Logf("cleanup students: %v", err)
				}
				if _, err := tc.db.NewDelete().Table("users.persons").Where("id = ?", pid).Exec(ctx); err != nil {
					t.Logf("cleanup persons: %v", err)
				}
			}
		}
	}()

	personCount, err := tc.db.NewSelect().
		Table("users.persons").
		Where("first_name = ? AND last_name = ?", firstName, lastName).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, personCount, "person must not persist when guardian phone validation fails (transaction must roll back)")
}
