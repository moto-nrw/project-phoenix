package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStudentGuardianRolesMigration_LegalRelationshipWinsOverContactFlags(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	testpkg.EnsureTestTenant(t, db, 1)
	student := testpkg.CreateTestStudent(t, db, "Legacy", "Contact", "1a")
	profile := testpkg.CreateTestGuardianProfile(t, db, "legacy-contact")

	var relationshipID int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO users.students_guardians
			(tenant_id, student_id, guardian_profile_id, relationship_type,
			 is_primary, is_emergency_contact, can_pickup, emergency_priority,
			 guardian_role, permissions)
		VALUES (?, ?, ?, 'guardian', FALSE, TRUE, TRUE, 1, 'custom', '{}'::jsonb)
		RETURNING id
	`, testpkg.Tenant(t), student.ID, profile.ID).Scan(ctx, &relationshipID))

	require.NoError(t, studentGuardianRolesUp(ctx, db))

	var row struct {
		GuardianRole string         `bun:"guardian_role"`
		Permissions  map[string]any `bun:"permissions"`
	}
	require.NoError(t, db.NewRaw(`
		SELECT guardian_role, permissions
		FROM users.students_guardians
		WHERE id = ?
	`, relationshipID).Scan(ctx, &row))

	assert.Equal(t, "legal_guardian", row.GuardianRole)
	assert.Equal(t, true, row.Permissions["parent_portal.access"])
	assert.Equal(t, true, row.Permissions["parent_portal.enrollment.submit"])
}

func TestStudentGuardianRolesMigration_BackfillsRelativeEmergencyPickupWithoutPortalAccess(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	testpkg.EnsureTestTenant(t, db, 1)
	student := testpkg.CreateTestStudent(t, db, "Legacy", "Contact", "1a")
	profile := testpkg.CreateTestGuardianProfile(t, db, "legacy-relative-contact")

	var relationshipID int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO users.students_guardians
			(tenant_id, student_id, guardian_profile_id, relationship_type,
			 is_primary, is_emergency_contact, can_pickup, emergency_priority,
			 guardian_role, permissions)
		VALUES (?, ?, ?, 'relative', FALSE, TRUE, TRUE, 1, 'custom', '{}'::jsonb)
		RETURNING id
	`, testpkg.Tenant(t), student.ID, profile.ID).Scan(ctx, &relationshipID))

	require.NoError(t, studentGuardianRolesUp(ctx, db))

	var row struct {
		GuardianRole string         `bun:"guardian_role"`
		Permissions  map[string]any `bun:"permissions"`
	}
	require.NoError(t, db.NewRaw(`
		SELECT guardian_role, permissions
		FROM users.students_guardians
		WHERE id = ?
	`, relationshipID).Scan(ctx, &row))

	assert.Equal(t, "pickup_only", row.GuardianRole)
	assert.NotContains(t, row.Permissions, "parent_portal.access")
	assert.NotContains(t, row.Permissions, "parent_portal.sick_note.submit")
}
