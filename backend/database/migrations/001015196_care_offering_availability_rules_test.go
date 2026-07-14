package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func careOfferingAvailabilityRuleColumnExists(t *testing.T, db *bun.DB) bool {
	t.Helper()
	var exists bool
	require.NoError(t, db.NewRaw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'enrollment'
				AND table_name = 'care_offerings'
				AND column_name = 'availability_rule'
				AND data_type = 'jsonb'
		)
	`).Scan(context.Background(), &exists))
	return exists
}

func TestCareOfferingAvailabilityRulesMigration(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()

	if !careOfferingAvailabilityRuleColumnExists(t, db) {
		require.NoError(t, careOfferingAvailabilityRulesUp(ctx, db))
	}
	t.Cleanup(func() {
		if !careOfferingAvailabilityRuleColumnExists(t, db) {
			require.NoError(t, careOfferingAvailabilityRulesUp(ctx, db))
		}
	})

	require.True(t, careOfferingAvailabilityRuleColumnExists(t, db))
	require.NoError(t, careOfferingAvailabilityRulesDown(ctx, db))
	require.False(t, careOfferingAvailabilityRuleColumnExists(t, db))
	require.NoError(t, careOfferingAvailabilityRulesUp(ctx, db))
	require.True(t, careOfferingAvailabilityRuleColumnExists(t, db))
	require.NoError(t, careOfferingAvailabilityRulesUp(ctx, db), "Up must be idempotent")
}
