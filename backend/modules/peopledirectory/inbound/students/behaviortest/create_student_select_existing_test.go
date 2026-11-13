package students_test

import (
	"testing"
)

// TestCreateStudent_SelectExistingGuardian_NotFound verifies a guardian_profile_id
// that does not exist (or belongs to another tenant — RLS makes it invisible) is
// a clean 400, not a 500, and commits no orphaned student.
func TestCreateStudent_SelectExistingGuardian_NotFound(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	const firstName = "GhostRef"
	const lastName = "Orphan"

	body := map[string]interface{}{
		"first_name":   firstName,
		"last_name":    lastName,
		"school_class": "3d",
		"guardians": []map[string]interface{}{
			{"guardian_profile_id": int64(999999999), "relationship_type": "parent", "emergency_priority": 1},
		},
	}

	assertNoStudentCommittedUnderTenantTx(t, tc, firstName, lastName, body, "nicht gefunden")
}
