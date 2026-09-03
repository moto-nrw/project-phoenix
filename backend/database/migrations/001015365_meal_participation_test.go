package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMealParticipationMigrationBackfillsFullGuardianPermission(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	require.NoError(t, mealParticipationDown(ctx, db))
	defer func() {
		require.NoError(t, mealParticipationDown(ctx, db))
		require.NoError(t, mealParticipationUp(ctx, db))
	}()

	fullGuardian := testpkg.CreateTestParentGuardianChain(t, db)
	restrictedGuardian := testpkg.CreateTestParentGuardianChain(t, db)
	for _, guardian := range []struct {
		chain testpkg.ParentChain
		role  string
	}{
		{chain: fullGuardian, role: "primary_guardian"},
		{chain: restrictedGuardian, role: "pickup_only"},
	} {
		_, err := db.NewRaw(`
			UPDATE users.students_guardians
			SET guardian_role = ?, permissions = '{"parent_portal.access": true}'::jsonb
			WHERE tenant_id = ? AND student_id = ? AND guardian_profile_id = ?
		`, guardian.role, guardian.chain.TenantID, guardian.chain.StudentID, guardian.chain.GuardianProfileID).Exec(ctx)
		require.NoError(t, err)
	}

	require.NoError(t, mealParticipationUp(ctx, db))
	permissionValue := func(chain testpkg.ParentChain) string {
		t.Helper()
		var value string
		require.NoError(t, db.NewRaw(`
			SELECT COALESCE(permissions ->> ?, '<missing>')
			FROM users.students_guardians
			WHERE tenant_id = ? AND student_id = ? AND guardian_profile_id = ?
		`, "parent_portal.meal_participation.manage", chain.TenantID, chain.StudentID, chain.GuardianProfileID).Scan(ctx, &value))
		return value
	}

	assert.Equal(t, "true", permissionValue(fullGuardian))
	assert.Equal(t, "<missing>", permissionValue(restrictedGuardian))
}
