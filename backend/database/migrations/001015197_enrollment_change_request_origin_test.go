package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func enrollmentChangeRequestOriginColumnExists(t *testing.T, db *bun.DB) bool {
	t.Helper()
	var exists bool
	require.NoError(t, db.NewRaw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'enrollment'
				AND table_name = 'change_requests'
				AND column_name = 'origin'
		)
	`).Scan(context.Background(), &exists))
	return exists
}

func TestEnrollmentChangeRequestOriginMigration(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	if !enrollmentChangeRequestOriginColumnExists(t, db) {
		require.NoError(t, enrollmentChangeRequestOriginUp(ctx, db))
	}
	t.Cleanup(func() {
		if !enrollmentChangeRequestOriginColumnExists(t, db) {
			require.NoError(t, enrollmentChangeRequestOriginUp(ctx, db))
		}
	})

	require.NoError(t, enrollmentChangeRequestOriginDown(ctx, db))
	assert.False(t, enrollmentChangeRequestOriginColumnExists(t, db))
	require.NoError(t, enrollmentChangeRequestOriginUp(ctx, db))
	assert.True(t, enrollmentChangeRequestOriginColumnExists(t, db))
	require.NoError(t, enrollmentChangeRequestOriginUp(ctx, db), "Up must be idempotent")
}
