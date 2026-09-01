// Verifies the schema installed by migration 1.15.190 (staff shift series).
package migrations

import (
	"context"
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaffShiftSeriesSchema(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.OwnTenantRows(t, db, tenantID)
	staff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Serie", "Schema")
	t.Cleanup(func() {
		ctx := context.Background()
		for _, table := range []string{
			"schedule.staff_shifts",
			"schedule.staff_shift_series_exceptions",
			"schedule.staff_shift_series",
			"schedule.calendar_periods",
		} {
			_, err := db.NewDelete().TableExpr(table).Where("tenant_id = ?", tenantID).Exec(ctx)
			require.NoError(t, err)
		}
	})

	ctx := context.Background()
	var periodID int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO schedule.calendar_periods (
			tenant_id, name, period_type, start_date, end_date, week_cycle_length, is_active
		) VALUES (?, ?, 'custom', '2026-08-01', '2027-01-31', 1, TRUE)
		RETURNING id
	`, tenantID, fmt.Sprintf("Serie-Schema-%d", tenantID)).Scan(ctx, &periodID))

	insertSeries := func(weekdays string, weekPattern int) (int64, error) {
		var id int64
		err := db.NewRaw(`
			INSERT INTO schedule.staff_shift_series (
				tenant_id, staff_id, weekdays, start_time, end_time,
				break_minutes, calendar_period_id, week_pattern,
				valid_from, created_by
			) VALUES (?, ?, ?::smallint[], '09:00', '12:00', 0, ?, ?, '2026-09-01', ?)
			RETURNING id
		`, tenantID, staff.ID, weekdays, periodID, weekPattern, staff.ID).Scan(ctx, &id)
		return id, err
	}

	seriesID, err := insertSeries("{1,3}", 0)
	require.NoError(t, err)

	t.Run("weekday check constraint", func(t *testing.T) {
		_, err := insertSeries("{}", 0)
		assert.ErrorContains(t, err, "chk_staff_shift_series_weekdays")
		_, err = insertSeries("{8}", 0)
		assert.ErrorContains(t, err, "chk_staff_shift_series_weekdays")
	})

	t.Run("week pattern check constraint", func(t *testing.T) {
		_, err := insertSeries("{1}", 3)
		assert.ErrorContains(t, err, "chk_staff_shift_series_week_pattern")
	})

	t.Run("validity check allows empty segment", func(t *testing.T) {
		insertWithValidUntil := func(validUntil string) error {
			_, err := db.NewRaw(`
				INSERT INTO schedule.staff_shift_series (
					tenant_id, staff_id, weekdays, start_time, end_time,
					break_minutes, calendar_period_id, week_pattern,
					valid_from, valid_until, created_by
				) VALUES (?, ?, '{1}'::smallint[], '09:00', '12:00', 0, ?, 0, '2026-09-01', ?, ?)
			`, tenantID, staff.ID, periodID, validUntil, staff.ID).Exec(ctx)
			return err
		}
		// valid_until = valid_from is a deliberately emptied segment (caps at
		// or before the first occurrence, offboarding of future series).
		require.NoError(t, insertWithValidUntil("2026-09-01"))
		assert.ErrorContains(t, insertWithValidUntil("2026-08-31"), "chk_staff_shift_series_validity")
	})

	t.Run("exception unique per series and date", func(t *testing.T) {
		insertException := func() error {
			_, err := db.NewRaw(`
				INSERT INTO schedule.staff_shift_series_exceptions (tenant_id, series_id, date, created_by)
				VALUES (?, ?, '2026-09-07', ?)
			`, tenantID, seriesID, staff.ID).Exec(ctx)
			return err
		}
		require.NoError(t, insertException())
		assert.ErrorContains(t, insertException(), "uniq_staff_shift_series_exception")
	})

	t.Run("shift series_id degrades to NULL on hard series delete", func(t *testing.T) {
		orphanSeriesID, err := insertSeries("{5}", 0)
		require.NoError(t, err)
		var shiftID int64
		require.NoError(t, db.NewRaw(`
			INSERT INTO schedule.staff_shifts (
				tenant_id, staff_id, date, start_time, end_time, break_minutes,
				created_by, series_id, series_occurrence_date, detached
			) VALUES (?, ?, '2026-09-04', '09:00', '12:00', 0, ?, ?, '2026-09-04', FALSE)
			RETURNING id
		`, tenantID, staff.ID, staff.ID, orphanSeriesID).Scan(ctx, &shiftID))

		_, err = db.NewDelete().TableExpr("schedule.staff_shift_series").Where("id = ?", orphanSeriesID).Exec(ctx)
		require.NoError(t, err)

		var seriesRef *int64
		require.NoError(t, db.NewRaw(`SELECT series_id FROM schedule.staff_shifts WHERE id = ?`, shiftID).Scan(ctx, &seriesRef))
		assert.Nil(t, seriesRef, "ON DELETE SET NULL must keep the concrete shift as a standalone row")
	})

	t.Run("calendar period with series cannot be deleted", func(t *testing.T) {
		_, err := db.NewDelete().TableExpr("schedule.calendar_periods").Where("id = ?", periodID).Exec(ctx)
		assert.ErrorContains(t, err, "fk_staff_shift_series_calendar_period")
	})

	t.Run("rls forced on both tables", func(t *testing.T) {
		for _, table := range []string{"staff_shift_series", "staff_shift_series_exceptions"} {
			var forced bool
			require.NoError(t, db.NewRaw(`
				SELECT relforcerowsecurity FROM pg_class
				WHERE oid = ('schedule.' || ?)::regclass
			`, table).Scan(ctx, &forced))
			assert.True(t, forced, "%s must FORCE ROW LEVEL SECURITY", table)
		}
	})
}
