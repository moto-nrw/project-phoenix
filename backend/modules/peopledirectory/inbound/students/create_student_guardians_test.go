package students_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
				if _, err := tc.db.NewDelete().Table("users.student_profiles").Where("person_id = ?", pid).Exec(ctx); err != nil {
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
	assert.Equal(t, int64(0), personCount, "person must not persist when guardian validation fails (transaction must roll back)")
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
