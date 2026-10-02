package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A row carries either one daily target or five weekday targets in range. The
// down migration only permits weekday rows it can represent without loss.
func TestStaffTargetOverrideWeekdaysConstraintsAndDown(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	staff := testpkg.CreateTestStaff(t, db, "Sonder", "Migration")
	tenantID := testpkg.Tenant(t)

	insert := func(start string, daily any, weekdays any) error {
		_, err := db.NewRaw(`INSERT INTO config.staff_target_overrides
			(tenant_id, staff_id, start_date, end_date, daily_minutes, weekday_minutes)
			VALUES (?, ?, ?::date, ?::date, ?, ?::integer[])`,
			tenantID, staff.ID, start, start, daily, weekdays).Exec(ctx)
		return err
	}

	require.NoError(t, insert("2026-10-05", 510, nil), "a uniform row stays valid")
	require.NoError(t, insert("2026-10-19", nil, "{210,30,30,30,30}"), "a weekday row is valid")

	for name, row := range map[string][2]any{
		"no target":      {nil, nil},
		"both targets":   {60, "{60,60,60,60,60}"},
		"four weekdays":  {nil, "{60,60,60,60}"},
		"NULL weekday":   {nil, "{60,NULL,60,60,60}"},
		"weekday >12h":   {nil, "{60,721,60,60,60}"},
		"weekday < 0":    {nil, "{60,-1,60,60,60}"},
		"two dimensions": {nil, "{{60,60,60,60,60}}"},
		"daily >12h":     {721, nil},
	} {
		assert.Error(t, insert("2026-11-02", row[0], row[1]), name)
	}

	require.ErrorContains(t, staffTargetOverrideWeekdaysDown(ctx, db), "non-uniform weekday targets")
	var monday int
	require.NoError(t, db.NewRaw(`SELECT weekday_minutes[1] FROM config.staff_target_overrides
		WHERE staff_id = ? AND start_date = '2026-10-19'`, staff.ID).Scan(ctx, &monday))
	assert.Equal(t, 210, monday, "a failed rollback must leave weekday targets unchanged")

	_, err := db.NewRaw(`UPDATE config.staff_target_overrides
		SET weekday_minutes = '{66,66,66,66,66}'
		WHERE staff_id = ? AND start_date = '2026-10-19'`, staff.ID).Exec(ctx)
	require.NoError(t, err)

	require.NoError(t, staffTargetOverrideWeekdaysDown(ctx, db))
	var folded int
	require.NoError(t, db.NewRaw(`SELECT daily_minutes FROM config.staff_target_overrides
		WHERE staff_id = ? AND start_date = '2026-10-19'`, staff.ID).Scan(ctx, &folded))
	assert.Equal(t, 66, folded, "a uniform weekday target stays unchanged")

	require.NoError(t, staffTargetOverrideWeekdaysUp(ctx, db))
	assert.Error(t, insert("2026-11-02", nil, nil), "the up migration restores the one-target check")
}
