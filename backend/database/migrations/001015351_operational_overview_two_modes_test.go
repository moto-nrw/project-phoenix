package migrations

import (
	"context"
	"sync"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Both operational-overview migrations intentionally inspect every school.
// Their tests may run alongside the package, but not through each other's
// global school scan.
var operationalOverviewMigrationTests sync.Mutex

func TestOperationalOverviewTwoModesPreservesExistingSchools(t *testing.T) {
	t.Parallel()
	operationalOverviewMigrationTests.Lock()
	defer operationalOverviewMigrationTests.Unlock()

	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	adminsTenant, _ := testpkg.CreateTestTenant(t, db)
	allStaffTenant, _ := testpkg.CreateTestTenant(t, db)
	ownTenant, _ := testpkg.CreateTestTenant(t, db)
	missingTenant, _ := testpkg.CreateTestTenant(t, db)

	_, err := db.NewRaw(`
		INSERT INTO config.setting_values (tenant_id, setting_key, value) VALUES
			(?, ?, '"admins"'::jsonb),
			(?, ?, '"all_staff"'::jsonb),
			(?, ?, '"own"'::jsonb)
	`,
		adminsTenant, operationalOverviewScopeKey,
		allStaffTenant, operationalOverviewScopeKey,
		ownTenant, operationalOverviewScopeKey,
	).Exec(ctx)
	require.NoError(t, err)

	require.NoError(t, operationalOverviewTwoModesUp(ctx, db))

	scopeOf := func(tenantID int64) string {
		t.Helper()
		var scope string
		require.NoError(t, db.NewRaw(`
			SELECT value #>> '{}'
			FROM config.setting_values
			WHERE tenant_id = ?
				AND setting_key = ?
		`, tenantID, operationalOverviewScopeKey).Scan(ctx, &scope))
		return scope
	}

	assert.Equal(t, "own", scopeOf(adminsTenant), "legacy admins keeps personal staff visibility and full admin visibility")
	assert.Equal(t, "all_staff", scopeOf(allStaffTenant), "whole-team visibility stays broad")
	assert.Equal(t, "own", scopeOf(ownTenant), "personal visibility stays personal")
	assert.Equal(t, "own", scopeOf(missingTenant), "the old restrictive default is pinned before the registry default changes")

	var auditCount int
	require.NoError(t, db.NewRaw(`
		SELECT COUNT(*)
		FROM config.setting_audit
		WHERE tenant_id IN (?, ?, ?, ?)
			AND setting_key = ?
			AND action = 'set'
	`, adminsTenant, allStaffTenant, ownTenant, missingTenant, operationalOverviewScopeKey).Scan(ctx, &auditCount))
	assert.Equal(t, 2, auditCount, "only the canonicalized and newly pinned values need audit entries")
}
