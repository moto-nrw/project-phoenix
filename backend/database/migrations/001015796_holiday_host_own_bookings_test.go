package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestHolidayHostOwnBookingsReadsOnlyConfirmedOwnChildren pins migration
// 1.15.796 (#5994): the hosting school reads, also after the week, the
// confirmed bookings of its own children in the weeks it hosts, and nothing
// else: not a received registration, not a holiday guest of another school,
// not a week another school hosts, not another school's children. A
// registration cancelled after its confirmation keeps the days before the
// cancellation; one cancelled without a confirmation, or before its week,
// books nothing.
func TestHolidayHostOwnBookingsReadsOnlyConfirmedOwnChildren(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Träger Eigene Ferien", Slug: fmt.Sprintf("traeger-eigene-ferien-%d", testpkg.UniqueTestTenantID(t)), Active: true,
	}).ID
	testpkg.OwnTraegerHolidayPeriods(t, db, organization)
	host := testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Eigene Ferien")
	other := testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Andere Ferien")

	var period int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_periods (organization_id, name, registration_opens_on,
			registration_closes_on, status, published_at) VALUES (?, 'Herbstferien', '2026-01-01', '2026-09-01', 'published', NOW())
		RETURNING id`, organization).Scan(ctx, &period))
	hostWeek := func(school int64, monday string) (week, model int64) {
		require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_period_weeks (organization_id, holiday_period_id, starts_on, ends_on, host_school_id)
			VALUES (?, ?, ?::date, ?::date + 4, ?) RETURNING id`, organization, period, monday, monday, school).Scan(ctx, &week))
		require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_week_time_models (organization_id, holiday_period_week_id, name, starts_at, ends_at, price_cents)
			VALUES (?, ?, 'Ganztag', '08:00', '16:00', 4500) RETURNING id`, organization, week).Scan(ctx, &model))
		return week, model
	}
	book := func(status, first string, homeSchool any, week, model int64) int64 {
		var registration, child int64
		require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registrations (organization_id, holiday_period_id, guardian_first_name,
				guardian_last_name, guardian_email, guardian_phone, privacy_consent_at, status)
			VALUES (?, ?, 'Eva', 'Muster', 'eva@example.org', '0123', NOW(), ?) RETURNING id`, organization, period, status).Scan(ctx, &registration))
		require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registration_children (organization_id, holiday_period_id,
				holiday_registration_id, position, first_name, last_name, birth_date, home_school_id)
			VALUES (?, ?, ?, 1, ?, 'Muster', '2018-03-04', ?) RETURNING id`, organization, period, registration, first, homeSchool).Scan(ctx, &child))
		_, err := db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_bookings (organization_id, holiday_period_id,
				holiday_registration_child_id, holiday_period_week_id, holiday_week_time_model_id) VALUES (?, ?, ?, ?, ?)`,
			organization, period, child, week, model)
		require.NoError(t, err)
		return registration
	}
	trail := func(registration int64, action, at string) {
		_, err := db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_audit
				(organization_id, holiday_period_id, holiday_registration_id, action, occurred_at)
			VALUES (?, ?, ?, ?, ?::timestamptz)`, organization, period, registration, action, at)
		require.NoError(t, err)
	}
	// A past week: the comparison looks back after the week.
	week, model := hostWeek(host, "2026-10-19")
	book("confirmed", "Ole", host, week, model)
	book("received", "Rita", host, week, model)
	book("confirmed", "Gustav", other, week, model)
	book("confirmed", "Fritz", nil, week, model)
	// Confirmed, then cancelled on Wednesday 21 October: booked Monday and
	// Tuesday.
	midweek := book("cancelled", "Mara", host, week, model)
	trail(midweek, "confirm", "2026-10-01T10:00:00+02:00")
	trail(midweek, "cancel", "2026-10-21T09:00:00+02:00")
	// Cancelled without a confirmation, and confirmed but cancelled before
	// the week: nothing booked.
	unconfirmed := book("cancelled", "Nora", host, week, model)
	trail(unconfirmed, "cancel", "2026-10-20T09:00:00+02:00")
	early := book("cancelled", "Emil", host, week, model)
	trail(early, "confirm", "2026-10-01T10:00:00+02:00")
	trail(early, "cancel", "2026-10-15T09:00:00+02:00")
	otherWeek, otherModel := hostWeek(other, "2026-10-26")
	book("confirmed", "Anna", other, otherWeek, otherModel)

	asSchool := func(school int64) []string {
		var names []string
		var rows []struct {
			FirstName string `bun:"first_name"`
			EndsOn    string `bun:"ends_on"`
		}
		require.NoError(t, db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant`); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id', ?, true)`, fmt.Sprint(school)); err != nil {
				return err
			}
			return tx.NewRaw(`SELECT first_name, ends_on::text AS ends_on
				FROM enrollment.holiday_host_own_bookings('2026-10-01', '2026-10-31') ORDER BY 1`).Scan(ctx, &rows)
		}))
		for _, row := range rows {
			names = append(names, row.FirstName+" bis "+row.EndsOn)
		}
		return names
	}
	assert.Equal(t, []string{"Mara bis 2026-10-20", "Ole bis 2026-10-23"}, asSchool(host),
		"the confirmed own child, and the cancelled one until the day before the cancellation; "+
			"no received registration, no cancellation without confirmation or before the week, no guest")
	assert.Equal(t, []string{"Anna bis 2026-10-30"}, asSchool(other), "each school reads its own hosted week only")

	var outside []string
	require.NoError(t, db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant`); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id', ?, true)`, fmt.Sprint(host)); err != nil {
			return err
		}
		return tx.NewRaw(`SELECT first_name FROM enrollment.holiday_host_own_bookings('2026-11-02', '2026-11-30')`).Scan(ctx, &outside)
	}))
	assert.Empty(t, outside, "a window outside the week reads nothing")
}
