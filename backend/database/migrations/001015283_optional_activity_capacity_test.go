package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOptionalActivityCapacity(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	const tenantID int64 = 22361
	testpkg.EnsureTestTenant(t, db, tenantID)
	defer testpkg.CleanupTenantTestData(t, db, tenantID)

	ctx := context.Background()
	require.NoError(t, optionalActivityCapacityDown(ctx, db))
	defer func() { require.NoError(t, optionalActivityCapacityUp(ctx, db)) }()

	template := testpkg.CreateTestActivityGroupForTenant(t, db, tenantID, "LegacyTemplateCapacity")
	operational := testpkg.CreateTestActivityGroupForTenant(t, db, tenantID, "OperationalCapacity")
	limited := testpkg.CreateTestActivityGroupForTenant(t, db, tenantID, "LimitedTemplateCapacity")

	_, err := db.NewRaw(`UPDATE activities.groups SET is_template = TRUE, max_participants = 999 WHERE id = ?`, template.ID).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw(`UPDATE activities.groups SET is_template = FALSE, max_participants = 999 WHERE id = ?`, operational.ID).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw(`UPDATE activities.groups SET is_template = TRUE, max_participants = 75 WHERE id = ?`, limited.ID).Exec(ctx)
	require.NoError(t, err)

	require.NoError(t, optionalActivityCapacityUp(ctx, db))

	var templateCapacity, operationalCapacity, limitedCapacity *int
	require.NoError(t, db.NewRaw(`SELECT max_participants FROM activities.groups WHERE id = ?`, template.ID).Scan(ctx, &templateCapacity))
	require.NoError(t, db.NewRaw(`SELECT max_participants FROM activities.groups WHERE id = ?`, operational.ID).Scan(ctx, &operationalCapacity))
	require.NoError(t, db.NewRaw(`SELECT max_participants FROM activities.groups WHERE id = ?`, limited.ID).Scan(ctx, &limitedCapacity))

	assert.Nil(t, templateCapacity)
	if assert.NotNil(t, operationalCapacity) {
		assert.Equal(t, 999, *operationalCapacity)
	}
	if assert.NotNil(t, limitedCapacity) {
		assert.Equal(t, 75, *limitedCapacity)
	}

	_, err = db.NewRaw(`UPDATE activities.groups SET max_participants = 0 WHERE id = ?`, limited.ID).Exec(ctx)
	require.Error(t, err)
}
