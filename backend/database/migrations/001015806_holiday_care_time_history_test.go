package migrations

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
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
	_, err = db.ExecContext(ctx, `DELETE FROM enrollment.holiday_registrations WHERE id=(
  SELECT holiday_registration_id FROM enrollment.holiday_registration_children WHERE id=?)`, child)
	require.NoError(t, err)
	assert.Equal(t, []string{"2020-07-06..2020-07-07 08:00/16:00", "2020-07-08..2020-07-10 09:00/13:00"}, read(f.host), "cutover and retained windows remain disjoint after source deletion")
}

// The registration expires after 92 days, but the school's guest child has
// its own retention. Removing the source must not rewrite its historical Soll.
func TestHolidayCareHistorySurvivesRegistrationRetention(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	for _, rebook := range []bool{false, true} {
		t.Run(fmt.Sprintf("rebooking=%t", rebook), func(t *testing.T) {
			f := newHolidaySharedAnswersFixture(t, db)
			// This period has no later week that would defer the retention cutoff.
			_, err := db.ExecContext(ctx, `DELETE FROM enrollment.holiday_period_weeks WHERE id=?`, f.open)
			require.NoError(t, err)
			child := f.register(t, holidaySharedRegistration{organization: f.own, period: f.period, version: 1, status: "confirmed", homeSchool: f.home, bookings: [][2]int64{{f.ended, f.endedModel}}})
			student := testpkg.CreateTestStudentForTenant(t, db, f.host, "Lia", "Muster", "Feriengast")
			_, err = db.ExecContext(ctx, `UPDATE enrollment.holiday_week_time_models SET starts_at='09:00', ends_at='13:00',
 care_time_history='{"2020-07-08":{"starts_at":"08:00","ends_at":"16:00"}}' WHERE id=?`, f.endedModel)
			require.NoError(t, err)
			// Reconciliation has copied the latest model times into the guest.
			_, err = db.ExecContext(ctx, `INSERT INTO enrollment.holiday_guests
 (tenant_id,organization_id,holiday_period_week_id,holiday_registration_child_id,student_id,starts_on,ends_on,time_model_name,time_model_starts_at,time_model_ends_at)
 VALUES (?,?,?,?,?,'2020-07-06','2020-07-10','Ganztag','09:00','13:00')`, f.host, f.own, f.ended, child, student.ID)
			require.NoError(t, err)
			read := func(school int64) []string {
				t.Helper()
				var windows []string
				f.asSchool(t, school, func(ctx context.Context, tx bun.Tx) error {
					// Test the explicit tenant filter without RLS masking mistakes.
					if _, err := tx.ExecContext(ctx, `RESET ROLE`); err != nil {
						return err
					}
					return tx.NewRaw(`SELECT starts_on::text || '..' || ends_on::text || ' ' || to_char(starts_at,'HH24:MI') || '/' || to_char(ends_at,'HH24:MI')
 FROM enrollment.holiday_host_guest_bookings('2020-07-06','2020-07-10') WHERE student_id=? ORDER BY starts_on`, student.ID).Scan(ctx, &windows)
				})
				return windows
			}
			want := []string{"2020-07-06..2020-07-07 08:00/16:00", "2020-07-08..2020-07-10 09:00/13:00"}
			require.Equal(t, want, read(f.host))
			if rebook {
				var model int64
				require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_week_time_models
   (organization_id,holiday_period_week_id,name,starts_at,ends_at,price_cents)
   VALUES (?,?,'Spät','10:00','17:00',4500) RETURNING id`, f.own, f.ended).Scan(ctx, &model))
				_, err = db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_audit
   (organization_id,holiday_period_id,holiday_registration_id,action,occurred_at,details)
   SELECT organization_id,holiday_period_id,holiday_registration_id,'rebook','2020-07-09T10:00:00+02:00',?::jsonb
   FROM enrollment.holiday_registration_children WHERE id=?`,
					fmt.Sprintf(`{"child_id":%d,"before":[{"week_id":%d,"time_model_id":%d}],"after":[{"week_id":%d,"time_model_id":%d}]}`, child, f.ended, f.endedModel, f.ended, model), child)
				require.NoError(t, err)
				_, err = db.ExecContext(ctx, `UPDATE enrollment.holiday_registration_bookings SET holiday_week_time_model_id=? WHERE holiday_registration_child_id=?`, model, child)
				require.NoError(t, err)
				_, err = db.ExecContext(ctx, `UPDATE enrollment.holiday_guests SET time_model_starts_at='10:00',time_model_ends_at='17:00' WHERE student_id=?`, student.ID)
				require.NoError(t, err)
				want = []string{"2020-07-06..2020-07-07 08:00/16:00", "2020-07-08..2020-07-08 09:00/13:00", "2020-07-09..2020-07-10 10:00/17:00"}
				require.Equal(t, want, read(f.host))
			}
			// Exercise the schema boundary directly, without composing Enrollment
			// from a migration test: the retention deletes registrations and audit
			// in one statement, and cascades their children/bookings. In particular,
			// the trigger must resolve rebookings while that audit is being deleted.
			_, err = db.ExecContext(ctx, `WITH ended AS (
    SELECT organization_id, holiday_period_id FROM enrollment.holiday_period_weeks
    GROUP BY organization_id, holiday_period_id HAVING max(ends_on) < ?
   ), audit AS (
    DELETE FROM enrollment.holiday_registration_audit AS entry USING ended
    WHERE entry.organization_id=ended.organization_id AND entry.holiday_period_id=ended.holiday_period_id
   )
   DELETE FROM enrollment.holiday_registrations AS registration USING ended
   WHERE registration.organization_id=ended.organization_id AND registration.holiday_period_id=ended.holiday_period_id`,
				calendar.NewDate(2020, 10, 12).AddDays(-92))
			require.NoError(t, err)
			var remaining int
			require.NoError(t, db.NewRaw(`SELECT count(*) FROM enrollment.holiday_registration_children WHERE id=?`, child).Scan(ctx, &remaining))
			require.Zero(t, remaining, "the source child really expired")
			assert.Equal(t, want, read(f.host), "past guest Soll survives the 92-day source deletion")
			assert.Empty(t, read(f.foreignSchool), "the retained windows remain school-scoped beneath RLS")
			// The only retained fields are dates and wall-clock times. The private
			// cross-school helper is not an additional school-readable capability.
			var publicHelper bool
			require.NoError(t, db.NewRaw(`SELECT has_function_privilege('phoenix_tenant',
  'enrollment.holiday_host_booked_windows_for_retention(date,date,bigint,bigint)','EXECUTE')`).Scan(ctx, &publicHelper))
			assert.False(t, publicHelper)
			if rebook {
				result := eraseSchoolAsCLI(t, db, f.host, beginErasure(t, db, f.host))
				require.Equal(t, "db_done", result.Status, "school erasure includes the retained care copy: %v", result.ErrorCode)
				assert.EqualValues(t, 1, result.RowCounts["enrollment.holiday_guests"])
			} else {
				_, err = db.ExecContext(ctx, `DELETE FROM users.student_profiles WHERE tenant_id=? AND id=?`, f.host, student.ID)
				require.NoError(t, err)
			}
			require.NoError(t, db.NewRaw(`SELECT count(*) FROM enrollment.holiday_guests WHERE tenant_id=? AND student_id=?`, f.host, student.ID).Scan(ctx, &remaining))
			assert.Zero(t, remaining, "retained care windows go with the guest child or its erased school")
			assert.Empty(t, read(f.host))
		})
	}
}
