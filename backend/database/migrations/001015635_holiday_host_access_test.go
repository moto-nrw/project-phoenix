package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestHolidayHostSchoolReadsOnlyConfirmedChildrenOfItsOpenWeeks pins ADR 0116
// on the Träger's holiday tables: under the request role the school that
// hosts a week of a published period reads that week, its period and time
// models and the bookings of confirmed registrations, until the last day of
// the week has passed, and the children only through
// enrollment.holiday_host_participants with their minimum fields. It reads
// no row of a child, no health information, no registration, no contact, no
// unconfirmed child and nothing of a week it does not host; another school
// of the Träger and a school of another Träger read nothing; no school
// writes. The Träger transaction keeps seeing exactly its own rows.
func TestHolidayHostSchoolReadsOnlyConfirmedChildrenOfItsOpenWeeks(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	newTraeger := func(name string) int64 {
		slug := fmt.Sprintf("traeger-ausrichtung-%d", testpkg.UniqueTestTenantID(t))
		organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: name, Slug: slug}).ID
		testpkg.OwnTraegerHolidayPeriods(t, db, organization)
		return organization
	}
	own := newTraeger("Träger Ausrichtung Eigen")
	host := testpkg.CreateTestSchoolInOrganization(t, db, own, "Träger Ausrichtung OGS Nord")
	home := testpkg.CreateTestSchoolInOrganization(t, db, own, "Träger Ausrichtung OGS Süd")
	foreign := newTraeger("Träger Ausrichtung Fremd")
	foreignSchool := testpkg.CreateTestSchoolInOrganization(t, db, foreign, "Träger Ausrichtung Fremd OGS")

	insertPeriod := func(status string) int64 {
		var period int64
		require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_periods (organization_id, name, registration_opens_on,
				registration_closes_on, status, published_at)
			VALUES (?, 'Sommerferien', '2020-01-01', '2099-06-30', ?, CASE WHEN ? = 'published' THEN NOW() END) RETURNING id`,
			own, status, status).Scan(ctx, &period))
		return period
	}
	insertWeek := func(period int64, startsOn, endsOn string) (int64, int64) {
		var week, model int64
		require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_period_weeks (organization_id, holiday_period_id, starts_on, ends_on, host_school_id)
			VALUES (?, ?, ?, ?, ?) RETURNING id`, own, period, startsOn, endsOn, host).Scan(ctx, &week))
		require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_week_time_models (organization_id, holiday_period_week_id, name, starts_at, ends_at, price_cents)
			VALUES (?, ?, 'bis 13 Uhr', '08:00', '13:00', 4500) RETURNING id`, own, week).Scan(ctx, &model))
		return week, model
	}
	register := func(period int64, status string, homeSchool any, bookings ...[2]int64) int64 {
		var registration, child int64
		require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registrations (organization_id, holiday_period_id, guardian_first_name,
				guardian_last_name, guardian_email, guardian_phone, privacy_consent_at, status)
			VALUES (?, ?, 'Eva', 'Muster', 'eva@example.org', '0123', NOW(), ?) RETURNING id`, own, period, status).Scan(ctx, &registration))
		require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registration_children (organization_id, holiday_period_id,
				holiday_registration_id, position, first_name, last_name, birth_date, home_school_id)
			VALUES (?, ?, ?, 1, 'Lia', 'Muster', '2018-03-01', ?) RETURNING id`, own, period, registration, homeSchool).Scan(ctx, &child))
		_, err := db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_contacts (organization_id, holiday_registration_id, kind, position, name)
			VALUES (?, ?, 'emergency', 1, 'Oma Muster')`, own, registration)
		require.NoError(t, err)
		for _, booking := range bookings {
			_, err := db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_bookings (organization_id, holiday_period_id,
					holiday_registration_child_id, holiday_period_week_id, holiday_week_time_model_id) VALUES (?, ?, ?, ?, ?)`,
				own, period, child, booking[0], booking[1])
			require.NoError(t, err)
		}
		return child
	}

	published := insertPeriod("published")
	open, openModel := insertWeek(published, "2099-07-06", "2099-07-10")
	ended, endedModel := insertWeek(published, "2020-07-06", "2020-07-10")
	draft := insertPeriod("draft")
	draftWeek, draftModel := insertWeek(draft, "2099-08-03", "2099-08-07")
	confirmed := register(published, "confirmed", home, [2]int64{open, openModel}, [2]int64{ended, endedModel})
	register(published, "confirmed", nil, [2]int64{open, openModel})
	bookedElsewhere := register(published, "confirmed", home, [2]int64{ended, endedModel})
	register(published, "received", home, [2]int64{open, openModel})
	register(published, "cancelled", nil, [2]int64{open, openModel})
	register(draft, "confirmed", home, [2]int64{draftWeek, draftModel})

	asRequestRole := func(setting, value string, fn func(bun.Tx) error) error {
		return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant`); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `SELECT set_config(?, ?, true)`, setting, value); err != nil {
				return err
			}
			return fn(tx)
		})
	}
	type visible struct {
		Periods       int `bun:"periods"`
		Weeks         int `bun:"weeks"`
		Models        int `bun:"models"`
		Registrations int `bun:"registrations"`
		Children      int `bun:"children"`
		Bookings      int `bun:"bookings"`
		Contacts      int `bun:"contacts"`
	}
	read := func(setting string, value int64) visible {
		var seen visible
		require.NoError(t, asRequestRole(setting, fmt.Sprint(value), func(tx bun.Tx) error {
			return tx.NewRaw(`SELECT
				(SELECT COUNT(*) FROM enrollment.holiday_periods WHERE organization_id = ?0) AS periods,
				(SELECT COUNT(*) FROM enrollment.holiday_period_weeks WHERE organization_id = ?0) AS weeks,
				(SELECT COUNT(*) FROM enrollment.holiday_week_time_models WHERE organization_id = ?0) AS models,
				(SELECT COUNT(*) FROM enrollment.holiday_registrations WHERE organization_id = ?0) AS registrations,
				(SELECT COUNT(*) FROM enrollment.holiday_registration_children WHERE organization_id = ?0) AS children,
				(SELECT COUNT(*) FROM enrollment.holiday_registration_bookings WHERE organization_id = ?0) AS bookings,
				(SELECT COUNT(*) FROM enrollment.holiday_registration_contacts WHERE organization_id = ?0) AS contacts`, own).
				Scan(ctx, &seen)
		}))
		return seen
	}

	assert.Equal(t, visible{Periods: 2, Weeks: 3, Models: 3, Registrations: 6, Children: 6, Bookings: 7, Contacts: 6},
		read("app.current_org_id", own), "the Träger keeps seeing all of its rows")
	assert.Equal(t, visible{Periods: 1, Weeks: 1, Models: 1, Bookings: 2},
		read("app.current_tenant_id", host),
		"the host reads its open week of the published period and the two confirmed bookings there, but no row of a child")
	assert.Equal(t, visible{}, read("app.current_tenant_id", home), "a school that hosts no week reads nothing, not even its own children")
	assert.Equal(t, visible{}, read("app.current_tenant_id", foreignSchool), "a school of another Träger reads nothing")
	assert.Equal(t, visible{}, read("app.current_org_id", foreign), "another Träger reads nothing")

	participants := func(school, week int64) []string {
		var names []string
		require.NoError(t, asRequestRole("app.current_tenant_id", fmt.Sprint(school), func(tx bun.Tx) error {
			return tx.NewRaw(`SELECT first_name || ' ' || birth_date::text || ' ' || COALESCE(home_school_id::text, 'guest')
				FROM enrollment.holiday_host_participants(?, ?) ORDER BY child_id`, own, week).Scan(ctx, &names)
		}))
		return names
	}
	assert.Equal(t, []string{"Lia 2018-03-01 " + fmt.Sprint(home), "Lia 2018-03-01 guest"}, participants(host, open),
		"the host reads the minimum fields of the two confirmed children of its open week")
	assert.Empty(t, participants(host, ended), "the access ends after the last day of the week")
	assert.Empty(t, participants(host, draftWeek), "a week of a draft period opens nothing")
	assert.Empty(t, participants(home, open), "a school that does not host the week reads no child")
	assert.Empty(t, participants(foreignSchool, open), "a school of another Träger reads no child")
	hostBooking := func(child int64) bool {
		var booked bool
		require.NoError(t, asRequestRole("app.current_tenant_id", fmt.Sprint(host), func(tx bun.Tx) error {
			return tx.NewRaw(`SELECT enrollment.holiday_host_booking(?, ?, ?)`, own, open, child).Scan(ctx, &booked)
		}))
		return booked
	}
	assert.True(t, hostBooking(confirmed), "the host confirms a booking of its open week")
	assert.False(t, hostBooking(bookedElsewhere),
		"a confirmed child of the Träger without a booking in the hosted week stays hidden from the host")

	err := asRequestRole("app.current_tenant_id", fmt.Sprint(host), func(tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `SELECT allergies FROM enrollment.holiday_host_participants(?, ?)`, own, open)
		return err
	})
	assert.ErrorContains(t, err, "allergies", "the host reads no health information")

	err = asRequestRole("app.current_tenant_id", fmt.Sprint(host), func(tx bun.Tx) error {
		result, err := tx.ExecContext(ctx, `UPDATE enrollment.holiday_registration_children SET first_name = 'Mia' WHERE id = ?`, confirmed)
		if err != nil {
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		assert.Zero(t, changed, "the host cannot change a child it reads")
		return nil
	})
	require.NoError(t, err)
	err = asRequestRole("app.current_tenant_id", fmt.Sprint(host), func(tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_bookings (organization_id, holiday_period_id,
				holiday_registration_child_id, holiday_period_week_id, holiday_week_time_model_id) VALUES (?, ?, ?, ?, ?)`,
			own, draft, confirmed, draftWeek, draftModel)
		return err
	})
	assert.ErrorContains(t, err, "row-level security", "the host writes no booking")
}
