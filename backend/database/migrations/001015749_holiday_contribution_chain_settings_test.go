package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHolidayContributionChainSettingsMigration pins #5664: a contribution
// frozen before names the fixed chain it was computed with, other values are
// refused, and the down migration removes only the two columns.
func TestHolidayContributionChainSettingsMigration(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	column := func(name string) (string, bool) {
		var fallback string
		err := db.NewRaw(`SELECT column_default FROM information_schema.columns
			WHERE table_schema = 'parent_billing' AND table_name = 'holiday_contributions' AND column_name = ?`, name).Scan(ctx, &fallback)
		return fallback, err == nil
	}
	rank, found := column("sibling_rank")
	require.True(t, found)
	assert.Contains(t, rank, "highest_contribution")
	order, found := column("cost_takeover_order")
	require.True(t, found)
	assert.Contains(t, order, "after_discount")
	var checks int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_constraint WHERE conname IN
		('chk_holiday_contributions_sibling_rank', 'chk_holiday_contributions_cost_takeover_order')`).Scan(ctx, &checks))
	assert.Equal(t, 2, checks)

	require.NoError(t, holidayContributionChainSettingsDown(ctx, db))
	_, found = column("sibling_rank")
	assert.False(t, found)
	_, found = column("cost_takeover_order")
	assert.False(t, found)
	_, found = column("version")
	assert.True(t, found, "the down migration keeps the rest of the table")
}
