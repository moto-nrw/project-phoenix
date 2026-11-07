package migrations

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// A pre-migration guest copy can still hold its original 08:00–16:00
// although its model was already changed to 09:00–13:00. Deployment must not
// introduce retrospective overruns by replacing that stored past baseline.
func TestHolidayCareHistoryCutoverKeepsExistingGuestBaseline(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	require.NoError(t, holidayCareTimeHistoryDown(ctx, db))
	f := newHolidaySharedAnswersFixture(t, db)
	child := f.register(t, holidaySharedRegistration{organization: f.own, period: f.period, version: 1, status: "confirmed", homeSchool: f.home, bookings: [][2]int64{{f.ended, f.endedModel}}})
	student := testpkg.CreateTestStudentForTenant(t, db, f.host, "Lia", "Muster", "Feriengast")
	_, err := db.ExecContext(ctx, `INSERT INTO enrollment.holiday_guests
  (tenant_id,organization_id,holiday_period_week_id,holiday_registration_child_id,student_id,starts_on,ends_on,time_model_name,time_model_starts_at,time_model_ends_at)
  VALUES (?,?,?,?,?,'2020-07-06','2020-07-10','Ganztag','08:00','16:00')`, f.host, f.own, f.ended, child, student.ID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE enrollment.holiday_week_time_models SET starts_at='09:00',ends_at='13:00' WHERE id=?`, f.endedModel)
	require.NoError(t, err)
	require.NoError(t, holidayCareTimeHistoryUpOn(ctx, db, calendar.NewDate(2020, 7, 8)))
	read := func(school int64) []string {
		t.Helper()
		var windows []string
		f.asSchool(t, school, func(ctx context.Context, tx bun.Tx) error {
			// The request's explicit school boundary must hold even as the owner,
			// without RLS hiding a missing filter in the cutover-baseline branch.
			if _, err := tx.ExecContext(ctx, `RESET ROLE`); err != nil {
				return err
			}
			return tx.NewRaw(`SELECT starts_on::text || '..' || ends_on::text || ' ' || to_char(starts_at,'HH24:MI') || '/' || to_char(ends_at,'HH24:MI')
    FROM enrollment.holiday_host_guest_bookings('2020-07-06','2020-07-10') WHERE student_id=? ORDER BY starts_on`, student.ID).Scan(ctx, &windows)
		})
		return windows
	}
	assert.Equal(t, []string{"2020-07-06..2020-07-07 08:00/16:00", "2020-07-08..2020-07-10 09:00/13:00"}, read(f.host))
	assert.Empty(t, read(f.foreignSchool), "a foreign school gets no historical guest baseline beneath RLS")
}
