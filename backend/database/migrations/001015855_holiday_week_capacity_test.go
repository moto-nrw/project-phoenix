package migrations

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestHolidayPromotedCareWindowsStartOnTheBerlinPromotionDay(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	f := newHolidaySharedAnswersFixture(t, db)
	child := f.register(t, holidaySharedRegistration{organization: f.own, period: f.period, version: 1, status: "confirmed", homeSchool: f.home, bookings: [][2]int64{{f.ended, f.endedModel}}})
	_, err := db.ExecContext(ctx, `UPDATE enrollment.holiday_registration_bookings SET promoted_at='2020-07-07T22:30:00Z' WHERE holiday_registration_child_id=?`, child)
	require.NoError(t, err)
	student := testpkg.CreateTestStudentForTenant(t, db, f.host, "Lia", "Muster", "Feriengast")
	_, err = db.ExecContext(ctx, `INSERT INTO enrollment.holiday_guests
  (tenant_id,organization_id,holiday_period_week_id,holiday_registration_child_id,student_id,starts_on,ends_on,time_model_name,time_model_starts_at,time_model_ends_at)
  VALUES (?,?,?,?,?,'2020-07-06','2020-07-10','Ganztag','08:00','13:00')`, f.host, f.own, f.ended, child, student.ID)
	require.NoError(t, err)
	read := func() []string {
		t.Helper()
		var windows []string
		f.asSchool(t, f.host, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `RESET ROLE`); err != nil {
				return err
			}
			return tx.NewRaw(`SELECT starts_on::text || '..' || ends_on::text
    FROM enrollment.holiday_host_booked_windows('2020-07-06','2020-07-10') WHERE child_id=? ORDER BY starts_on`, child).Scan(ctx, &windows)
		})
		return windows
	}
	assert.Equal(t, []string{"2020-07-08..2020-07-10"}, read(), "UTC evening is already the next Berlin day")
	_, err = db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_audit
  (organization_id,holiday_period_id,holiday_registration_id,action,occurred_at,details)
  SELECT organization_id,holiday_period_id,holiday_registration_id,'rebook','2020-07-10T10:00:00+02:00',?::jsonb
  FROM enrollment.holiday_registration_children WHERE id=?`,
		fmt.Sprintf(`{"child_id":%d,"before":[{"week_id":%d,"time_model_id":%d,"promoted_at":"2020-07-07T22:30:00Z"}],"after":[]}`, child, f.ended, f.endedModel), child)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `DELETE FROM enrollment.holiday_registration_bookings WHERE holiday_registration_child_id=?`, child)
	require.NoError(t, err)
	assert.Equal(t, []string{"2020-07-08..2020-07-09"}, read(), "historical rebooking segments preserve the promotion boundary")
	// Source retention removes audit and registration together. The guest's
	// retained historical windows must still exclude days before admission.
	_, err = db.ExecContext(ctx, `WITH audit AS (
   DELETE FROM enrollment.holiday_registration_audit WHERE organization_id=? AND holiday_period_id=?
  ) DELETE FROM enrollment.holiday_registrations WHERE organization_id=? AND holiday_period_id=?`, f.own, f.period, f.own, f.period)
	require.NoError(t, err)
	var retained []string
	f.asSchool(t, f.host, func(ctx context.Context, tx bun.Tx) error {
		return tx.NewRaw(`SELECT starts_on::text || '..' || ends_on::text
   FROM enrollment.holiday_host_guest_bookings('2020-07-06','2020-07-10') WHERE student_id=? ORDER BY starts_on`, student.ID).Scan(ctx, &retained)
	})
	assert.Equal(t, []string{"2020-07-08..2020-07-09"}, retained, "source retention preserves the promotion boundary")
}

// The erasure of 1.15.855 is that of 1.15.843 with the place mail named
// next to the receipt, nothing else.
func TestHolidayPlaceErasureExtendsTheErasureOf843(t *testing.T) {
	t.Parallel()
	require.Equal(t, 1, strings.Count(demoVisitorErasureOutboxKindsSQL, holidayReceiptErasureKind))
	require.NotEqual(t, demoVisitorErasureOutboxKindsSQL, holidayPlaceErasureSQL)
	assert.Equal(t, demoVisitorErasureOutboxKindsSQL,
		strings.Replace(holidayPlaceErasureSQL, holidayPlaceErasureKinds, holidayReceiptErasureKind, 1))
}

// A base line frozen before 1.15.855 gets the id of its week only where one
// week of its period has its dates (#4699). Parallel weeks of the same dates
// at two host schools leave it NULL: a wrong id would let a later move up
// price the replacement on top of it.
func TestHolidayContributionLinesNameTheirWeekOnlyWhenUnique(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	f := newHolidaySharedAnswersFixture(t, db)
	parallel, _ := f.insertWeek(t, f.own, f.period, f.home, "2020-07-06", "2020-07-10")
	child := f.register(t, holidaySharedRegistration{organization: f.own, period: f.period, version: 1, status: "confirmed", homeSchool: f.home, bookings: [][2]int64{{f.ended, f.endedModel}}})
	var contribution int64
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.holiday_contributions
			(organization_id, holiday_period_id, holiday_registration_id, holiday_registration_child_id, total_cents)
		SELECT organization_id, holiday_period_id, holiday_registration_id, id, 9000
		FROM enrollment.holiday_registration_children WHERE id = ? RETURNING id`, child).Scan(ctx, &contribution))
	_, err := db.ExecContext(ctx, `INSERT INTO parent_billing.holiday_contribution_lines
			(organization_id, holiday_contribution_id, position, kind, amount_cents, week_starts_on, week_ends_on, time_model_name)
		VALUES (?0, ?1, 1, 'base', 4500, '2020-07-06', '2020-07-10', 'bis 13 Uhr'),
			(?0, ?1, 2, 'base', 4500, '2099-07-06', '2099-07-10', 'bis 13 Uhr')`, f.own, contribution)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, holidayContributionLineWeeksSQL)
	require.NoError(t, err)
	var weeks []int64
	require.NoError(t, db.NewRaw(`SELECT COALESCE(holiday_period_week_id, 0) FROM parent_billing.holiday_contribution_lines
		WHERE holiday_contribution_id = ? ORDER BY position`, contribution).Scan(ctx, &weeks))
	require.Len(t, weeks, 2)
	assert.Zero(t, weeks[0], "the dates of week %d and %d are the same, the line names neither", f.ended, parallel)
	assert.Equal(t, f.open, weeks[1], "the one week with the line's dates")
}
