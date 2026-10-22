package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The backfill writes a catalog for every school, so the test runs on a
// clone of its own instead of the package's shared one.
func TestRoomCategoriesBackfillsDefaultsAndUsedCategories(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()

	_, err := db.ExecContext(ctx, `DROP TABLE facilities.room_categories`)
	require.NoError(t, err, "stage the state before the migration")
	school, _ := testpkg.CreateTestTenant(t, db)
	_, err = db.ExecContext(ctx, `
		INSERT INTO facilities.rooms (tenant_id, name, category, is_system) VALUES
			(?, 'OGS-Raum 1', 'gruppenraum', false),
			(?, 'Werkraum', ' Werkstatt ', false),
			(?, 'Holzwerkstatt', 'werkstatt', false),
			(?, 'Schulhof', 'Schulhof', true),
			(?, 'Abstellraum', NULL, false)
	`, school, school, school, school, school)
	require.NoError(t, err)

	require.NoError(t, roomCategoriesUp(ctx, db))
	require.NoError(t, roomCategoriesUp(ctx, db), "a second run changes nothing")

	var rows []struct {
		Name  string `bun:"name"`
		Color string `bun:"color"`
	}
	require.NoError(t, db.NewRaw(`
		SELECT name, color FROM facilities.room_categories WHERE tenant_id = ? ORDER BY LOWER(name)
	`, school).Scan(ctx, &rows))
	colors := map[string]string{}
	for _, row := range rows {
		colors[row.Name] = row.Color
	}
	assert.Equal(t, map[string]string{
		"Gruppenraum":   "#283590",
		"Normaler Raum": "#374750",
		"Sport":         "#827710",
		"Themenraum":    "#6A1B90",
		"Werkstatt":     "#8D3B1E",
	}, colors, "the defaults keep their spelling; a used category is added once, without the Schulhof")

	var colorIndexes int
	require.NoError(t, db.NewRaw(`
		SELECT COUNT(*) FROM pg_indexes
		WHERE schemaname = 'facilities' AND indexname = 'uniq_facilities_rooms_tenant_color'
	`).Scan(ctx, &colorIndexes))
	assert.Zero(t, colorIndexes, "room colors may repeat within a school")

	var forced bool
	require.NoError(t, db.NewRaw(`
		SELECT relforcerowsecurity FROM pg_class WHERE oid = 'facilities.room_categories'::regclass
	`).Scan(ctx, &forced))
	assert.True(t, forced, "the catalog is tenant-isolated")
}
