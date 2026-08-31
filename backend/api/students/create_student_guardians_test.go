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

// TestCreateStudent_WithGuardians verifies a student and its guardians (profile,
// relationship, and phone numbers) are created together in one request.
func TestCreateStudent_WithGuardians(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

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
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	require.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())

	var resp createStudentResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.NotZero(t, resp.Data.ID, "expected a student id in the response")

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
	t.Parallel()

	tc := setupStudentsRoute(t)

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
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

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
	t.Parallel()

	tc := setupStudentsRoute(t)

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
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

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

// assertGuardianBadRequestNoOrphan posts a create-student body that must fail
// guardian validation, asserts a 400 carrying the given German fragment, and
// verifies the transaction rolled back so no person/student survives. It also
// cleans up any residue from a regressed rollback or a prior failed run.
func assertGuardianBadRequestNoOrphan(
	t *testing.T,
	tc *testContext,
	firstName, lastName string,
	body map[string]interface{},
	wantBodyContains string,
) {
	t.Helper()

	req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	assert.Equal(t, http.StatusBadRequest, rr.Code,
		"invalid guardian input must return 400. Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), wantBodyContains,
		"400 body should name the offending field, not a generic server error")

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
	assert.Equal(t, 0, personCount, "person must not persist when guardian validation fails (transaction must roll back)")
}

// TestCreateStudent_MultipleGuardians verifies the batch path: several guardians
// in one request are each created and linked to the same student atomically.
func TestCreateStudent_MultipleGuardians(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	body := map[string]interface{}{
		"first_name":   "Multi",
		"last_name":    "Guardians",
		"school_class": "2a",
		"guardians": []map[string]interface{}{
			{
				"first_name":         "Mother",
				"last_name":          "One",
				"email":              "mother.multi.test@example.com",
				"relationship_type":  "parent",
				"is_primary":         true,
				"emergency_priority": 1,
				"phone_numbers": []map[string]interface{}{
					{"phone_number": "0151 1111111", "phone_type": "mobile", "is_primary": true},
				},
			},
			{
				"first_name":         "Father",
				"last_name":          "Two",
				"email":              "father.multi.test@example.com",
				"relationship_type":  "guardian",
				"can_pickup":         true,
				"emergency_priority": 2,
				"phone_numbers": []map[string]interface{}{
					// Unknown phone type — must be coerced to "mobile", not
					// rejected (matches AddPhoneNumber's own coercion).
					{"phone_number": "0151 2222222", "phone_type": "fax-machine", "is_primary": true},
				},
			},
		},
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	require.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())

	var resp createStudentResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.NotZero(t, resp.Data.ID)

	ctx := context.Background()
	relCount, err := tc.db.NewSelect().
		Table("users.students_guardians").
		Where("student_id = ?", resp.Data.ID).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, relCount, "both guardians must be linked to the student")

	// The unknown phone type was coerced to "mobile" and persisted.
	var phoneType string
	require.NoError(t, tc.db.NewSelect().
		ColumnExpr("phone_type").
		Table("users.guardian_phone_numbers").
		Where("phone_number = ?", "0151 2222222").
		Scan(ctx, &phoneType))
	assert.Equal(t, "mobile", phoneType, "unknown phone type must be coerced to mobile")
}

// TestCreateStudent_GuardianOptionalFieldsPersisted verifies the optional
// profile/relationship fields (address, notes, contact method, pickup notes,
// emergency flags) are mapped through and persisted, not silently dropped.
func TestCreateStudent_GuardianOptionalFieldsPersisted(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	body := map[string]interface{}{
		"first_name":   "Optional",
		"last_name":    "Fields",
		"school_class": "2b",
		"guardians": []map[string]interface{}{
			{
				"first_name":               "Detailed",
				"last_name":                "Guardian",
				"email":                    "detailed.guardian.test@example.com",
				"address_street":           "Musterstraße 1",
				"address_city":             "Musterstadt",
				"address_postal_code":      "12345",
				"preferred_contact_method": "email",
				"notes":                    "Bevorzugt nachmittags erreichbar",
				"relationship_type":        "relative",
				"is_emergency_contact":     true,
				"pickup_notes":             "Nur mit Ausweis",
				"emergency_priority":       3,
			},
		},
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	require.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())

	var resp createStudentResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.NotZero(t, resp.Data.ID)

	ctx := context.Background()

	var profile struct {
		AddressStreet          string `bun:"address_street"`
		AddressCity            string `bun:"address_city"`
		AddressPostalCode      string `bun:"address_postal_code"`
		PreferredContactMethod string `bun:"preferred_contact_method"`
		Notes                  string `bun:"notes"`
	}
	require.NoError(t, tc.db.NewSelect().
		ColumnExpr("gp.address_street, gp.address_city, gp.address_postal_code, gp.preferred_contact_method, gp.notes").
		TableExpr("users.guardian_profiles AS gp").
		Join("JOIN users.students_guardians AS sg ON sg.guardian_profile_id = gp.id").
		Where("sg.student_id = ?", resp.Data.ID).
		Scan(ctx, &profile))
	assert.Equal(t, "Musterstraße 1", profile.AddressStreet)
	assert.Equal(t, "Musterstadt", profile.AddressCity)
	assert.Equal(t, "12345", profile.AddressPostalCode)
	assert.Equal(t, "email", profile.PreferredContactMethod)
	assert.Equal(t, "Bevorzugt nachmittags erreichbar", profile.Notes)

	var rel struct {
		RelationshipType   string `bun:"relationship_type"`
		IsEmergencyContact bool   `bun:"is_emergency_contact"`
		EmergencyPriority  int    `bun:"emergency_priority"`
		PickupNotes        string `bun:"pickup_notes"`
	}
	require.NoError(t, tc.db.NewSelect().
		ColumnExpr("relationship_type, is_emergency_contact, emergency_priority, pickup_notes").
		Table("users.students_guardians").
		Where("student_id = ?", resp.Data.ID).
		Scan(ctx, &rel))
	assert.Equal(t, "relative", rel.RelationshipType)
	assert.True(t, rel.IsEmergencyContact)
	assert.Equal(t, 3, rel.EmergencyPriority)
	assert.Equal(t, "Nur mit Ausweis", rel.PickupNotes)
}

// TestCreateStudent_GuardianMissingRelationshipType verifies Bind rejects a
// guardian without a relationship_type (the one field required to link) with a
// 400 before any row is written.
func TestCreateStudent_GuardianMissingRelationshipType(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	assertGuardianBadRequestNoOrphan(t, tc, "NoRelType", "Guardian", map[string]interface{}{
		"first_name":   "NoRelType",
		"last_name":    "Guardian",
		"school_class": "2c",
		"guardians": []map[string]interface{}{
			{
				"first_name": "Missing",
				"last_name":  "Relationship",
				"email":      "missing.rel.test@example.com",
				// relationship_type omitted
			},
		},
	}, "relationship_type")
}

// TestCreateStudent_GuardianInvalidContactMethod verifies an unknown preferred
// contact method is a classified client error (400) and rolls back the student.
func TestCreateStudent_GuardianInvalidContactMethod(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	assertGuardianBadRequestNoOrphan(t, tc, "BadContact", "Method", map[string]interface{}{
		"first_name":   "BadContact",
		"last_name":    "Method",
		"school_class": "2d",
		"guardians": []map[string]interface{}{
			{
				"first_name":               "Invalid",
				"last_name":                "Contact",
				"email":                    "valid.contact.test@example.com",
				"preferred_contact_method": "carrier-pigeon",
				"relationship_type":        "parent",
				"emergency_priority":       1,
			},
		},
	}, "Kontaktmethode")
}

// TestCreateStudent_GuardianPhoneMissingNumber verifies an empty phone number is
// a classified client error (400) and rolls back the student.
func TestCreateStudent_GuardianPhoneMissingNumber(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	assertGuardianBadRequestNoOrphan(t, tc, "EmptyPhone", "Guardian", map[string]interface{}{
		"first_name":   "EmptyPhone",
		"last_name":    "Guardian",
		"school_class": "2e",
		"guardians": []map[string]interface{}{
			{
				"first_name":         "Empty",
				"last_name":          "Phone",
				"email":              "empty.phone.test@example.com",
				"relationship_type":  "parent",
				"emergency_priority": 1,
				"phone_numbers": []map[string]interface{}{
					{"phone_number": "   ", "phone_type": "mobile", "is_primary": true},
				},
			},
		},
	}, "Telefonnummer ist erforderlich")
}

// TestCreateStudent_GuardianPhoneTooFewDigits verifies a phone number with fewer
// than three digits is a classified client error (400) and rolls back.
func TestCreateStudent_GuardianPhoneTooFewDigits(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	assertGuardianBadRequestNoOrphan(t, tc, "ShortPhone", "Guardian", map[string]interface{}{
		"first_name":   "ShortPhone",
		"last_name":    "Guardian",
		"school_class": "2f",
		"guardians": []map[string]interface{}{
			{
				"first_name":         "Short",
				"last_name":          "Phone",
				"email":              "short.phone.test@example.com",
				"relationship_type":  "parent",
				"emergency_priority": 1,
				"phone_numbers": []map[string]interface{}{
					{"phone_number": "12", "phone_type": "mobile", "is_primary": true},
				},
			},
		},
	}, "Ziffern")
}
