package students_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// seedExistingGuardian inserts a guardian profile in tenant 1 and returns it,
// registering its cleanup. Used by the select-existing (#1513) tests to stand in
// for a sibling's already-existing guardian.
func seedExistingGuardian(t *testing.T, tc *testContext, firstName, lastName, email string) *users.GuardianProfile {
	t.Helper()
	ctx := context.Background()

	g := &users.GuardianProfile{
		FirstName:              firstName,
		LastName:               lastName,
		Email:                  emailPtr(email),
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
	}
	g.SetTenantID(testpkg.Tenant(t))
	_, err := tc.db.NewInsert().Model(g).Exec(ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		if _, err := tc.db.NewDelete().
			Table("users.guardian_profiles").
			Where("id = ?", g.ID).
			Exec(ctx); err != nil {
			t.Logf("cleanup seeded guardian: %v", err)
		}
	})
	return g
}

// TestCreateStudent_SelectExistingGuardian verifies the sibling case: a student
// created with guardian_profile_id links the EXISTING profile (no new profile,
// no mutation, no phone writes) and applies only the relationship flags.
func TestCreateStudent_SelectExistingGuardian(t *testing.T) {
	tc := setupTestContext(t)
	ctx := context.Background()

	existing := seedExistingGuardian(t, tc, "Sibling", "Parent", "sibling.parent.existing@example.com")

	body := map[string]interface{}{
		"first_name":   "Second",
		"last_name":    "Child",
		"school_class": "3a",
		"guardians": []map[string]interface{}{
			{
				"guardian_profile_id":  existing.ID,
				"relationship_type":    "parent",
				"is_primary":           true,
				"can_pickup":           true,
				"is_emergency_contact": true,
				"emergency_priority":   1,
			},
		},
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
	require.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())

	var resp createStudentResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.NotZero(t, resp.Data.ID)
	defer cleanupStudentWithGuardians(t, tc, resp.Data.ID, resp.Data.PersonID)

	// Exactly one link, pointing at the EXISTING profile.
	var linkedID int64
	require.NoError(t, tc.db.NewSelect().
		Table("users.students_guardians").
		Column("guardian_profile_id").
		Where("student_id = ?", resp.Data.ID).
		Scan(ctx, &linkedID))
	assert.Equal(t, existing.ID, linkedID, "link must target the existing profile, not a new one")

	relCount, err := tc.db.NewSelect().
		Table("users.students_guardians").
		Where("student_id = ?", resp.Data.ID).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, relCount, "exactly one student-guardian relationship")

	// No duplicate profile was created for the same email.
	profileCount, err := tc.db.NewSelect().
		Table("users.guardian_profiles").
		Where("email = ?", "sibling.parent.existing@example.com").
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, profileCount, "the existing profile must not be duplicated")

	// The existing profile's own data is untouched.
	var reloaded struct {
		FirstName string  `bun:"first_name"`
		LastName  string  `bun:"last_name"`
		Email     *string `bun:"email"`
	}
	require.NoError(t, tc.db.NewSelect().
		ColumnExpr("first_name, last_name, email").
		Table("users.guardian_profiles").
		Where("id = ?", existing.ID).
		Scan(ctx, &reloaded))
	assert.Equal(t, "Sibling", reloaded.FirstName, "existing first name must not be mutated")
	assert.Equal(t, "Parent", reloaded.LastName, "existing last name must not be mutated")
	require.NotNil(t, reloaded.Email)
	assert.Equal(t, "sibling.parent.existing@example.com", *reloaded.Email, "existing email must not be mutated")

	// No phone numbers were written against the existing profile.
	phoneCount, err := tc.db.NewSelect().
		Table("users.guardian_phone_numbers").
		Where("guardian_profile_id = ?", existing.ID).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, phoneCount, "linking an existing guardian must not write phone numbers")

	// The relationship flags from the form were applied to the new link.
	var rel struct {
		RelationshipType string `bun:"relationship_type"`
		IsPrimary        bool   `bun:"is_primary"`
		CanPickup        bool   `bun:"can_pickup"`
	}
	require.NoError(t, tc.db.NewSelect().
		ColumnExpr("relationship_type, is_primary, can_pickup").
		Table("users.students_guardians").
		Where("student_id = ?", resp.Data.ID).
		Scan(ctx, &rel))
	assert.Equal(t, "parent", rel.RelationshipType)
	assert.True(t, rel.IsPrimary)
	assert.True(t, rel.CanPickup)
}

// TestCreateStudent_SelectExistingGuardian_DuplicateSkipped verifies the same
// existing guardian selected twice in one request yields a single link (the
// duplicate is skipped silently), never a UNIQUE-constraint 500.
func TestCreateStudent_SelectExistingGuardian_DuplicateSkipped(t *testing.T) {
	tc := setupTestContext(t)
	ctx := context.Background()

	existing := seedExistingGuardian(t, tc, "Dup", "Selection", "dup.selection.existing@example.com")

	body := map[string]interface{}{
		"first_name":   "DupSelect",
		"last_name":    "Child",
		"school_class": "3b",
		"guardians": []map[string]interface{}{
			{"guardian_profile_id": existing.ID, "relationship_type": "parent", "emergency_priority": 1},
			{"guardian_profile_id": existing.ID, "relationship_type": "guardian", "emergency_priority": 2},
		},
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
	require.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())

	var resp createStudentResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.NotZero(t, resp.Data.ID)
	defer cleanupStudentWithGuardians(t, tc, resp.Data.ID, resp.Data.PersonID)

	relCount, err := tc.db.NewSelect().
		Table("users.students_guardians").
		Where("student_id = ?", resp.Data.ID).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, relCount, "the duplicate existing selection must be skipped, leaving exactly one link")
}

// TestCreateStudent_MixedNewAndExistingGuardian verifies one request can both
// link an existing guardian and create a brand-new one in the same atomic write.
func TestCreateStudent_MixedNewAndExistingGuardian(t *testing.T) {
	tc := setupTestContext(t)
	ctx := context.Background()

	existing := seedExistingGuardian(t, tc, "Mixed", "Existing", "mixed.existing@example.com")

	body := map[string]interface{}{
		"first_name":   "Mixed",
		"last_name":    "Child",
		"school_class": "3c",
		"guardians": []map[string]interface{}{
			{"guardian_profile_id": existing.ID, "relationship_type": "parent", "is_primary": true, "emergency_priority": 1},
			{
				"first_name":         "Brand",
				"last_name":          "New",
				"email":              "brand.new.mixed@example.com",
				"relationship_type":  "guardian",
				"emergency_priority": 2,
			},
		},
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
	require.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())

	var resp createStudentResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.NotZero(t, resp.Data.ID)
	defer cleanupStudentWithGuardians(t, tc, resp.Data.ID, resp.Data.PersonID)

	relCount, err := tc.db.NewSelect().
		Table("users.students_guardians").
		Where("student_id = ?", resp.Data.ID).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, relCount, "both the existing link and the new guardian must be linked")

	// The existing profile is linked alongside a freshly created one.
	var existingLinks int
	existingLinks, err = tc.db.NewSelect().
		Table("users.students_guardians").
		Where("student_id = ?", resp.Data.ID).
		Where("guardian_profile_id = ?", existing.ID).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, existingLinks, "the existing profile must be linked exactly once")
}

// TestCreateStudent_SelectExistingGuardian_NotFound verifies a guardian_profile_id
// that does not exist (or belongs to another tenant — RLS makes it invisible) is
// a clean 400, not a 500, and commits no orphaned student.
func TestCreateStudent_SelectExistingGuardian_NotFound(t *testing.T) {
	tc := setupTestContext(t)

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
