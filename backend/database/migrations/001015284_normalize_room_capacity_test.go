package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeRoomCapacity(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	tenantID, _ := testpkg.CreateTestTenant(t, db)
	defer testpkg.CleanupTestTenant(t, db, tenantID)

	ctx := context.Background()
	require.NoError(t, normalizeRoomCapacityDown(ctx, db))

	var zeroID, unlimitedID, limitedID int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO facilities.rooms (tenant_id, name, capacity) VALUES (?, 'Legacy zero capacity', 0)
		RETURNING id;
	`, tenantID).Scan(ctx, &zeroID))
	require.NoError(t, db.NewRaw(`
		INSERT INTO facilities.rooms (tenant_id, name, capacity) VALUES (?, 'Unlimited capacity', NULL)
		RETURNING id;
	`, tenantID).Scan(ctx, &unlimitedID))
	require.NoError(t, db.NewRaw(`
		INSERT INTO facilities.rooms (tenant_id, name, capacity) VALUES (?, 'Limited capacity', 25)
		RETURNING id;
	`, tenantID).Scan(ctx, &limitedID))

	require.NoError(t, normalizeRoomCapacityUp(ctx, db))

	var zeroCapacity, unlimitedCapacity, limitedCapacity *int
	require.NoError(t, db.NewRaw(`SELECT capacity FROM facilities.rooms WHERE id = ?`, zeroID).Scan(ctx, &zeroCapacity))
	require.NoError(t, db.NewRaw(`SELECT capacity FROM facilities.rooms WHERE id = ?`, unlimitedID).Scan(ctx, &unlimitedCapacity))
	require.NoError(t, db.NewRaw(`SELECT capacity FROM facilities.rooms WHERE id = ?`, limitedID).Scan(ctx, &limitedCapacity))

	assert.Nil(t, zeroCapacity)
	assert.Nil(t, unlimitedCapacity)
	if assert.NotNil(t, limitedCapacity) {
		assert.Equal(t, 25, *limitedCapacity)
	}

	_, err := db.NewRaw(`UPDATE facilities.rooms SET capacity = 0 WHERE id = ?`, limitedID).Exec(ctx)
	require.Error(t, err)
}
