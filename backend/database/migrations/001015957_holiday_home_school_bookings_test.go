package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// TestHolidayHomeSchoolBookingsReadsOnlyConfirmedChildrenAtOtherSchools pins
// migration 1.15.957 (#6679): the home school reads the confirmed, placed
// bookings of its own children in the weeks another school of its Träger
// hosts, until the last day of the week, and nothing else: no received
// registration, no place on the waiting list, no week it hosts itself, no
// child of another school, no week that ended, nothing outside a school
// transaction.
func TestHolidayHomeSchoolBookingsReadsOnlyConfirmedChildrenAtOtherSchools(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Träger Heimatschule", Slug: fmt.Sprintf("traeger-heimatschule-%d", testpkg.UniqueTestTenantID(t)), Active: true,
	}).ID
	testpkg.OwnTraegerHolidayPeriods(t, db, organization)
	home := testpkg.CreateTestSchoolInOrganization(t, db, organization, "GS Wissingen")
	host := testpkg.CreateTestSchoolInOrganization(t, db, organization, "GS Bissendorf")

	var period int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_periods (organization_id, name, registration_opens_on,
			registration_closes_on, status, published_at) VALUES (?, 'Weihnachtsferien', '2026-01-01', '2026-12-01', 'published', NOW())
		RETURNING id`, organization).Scan(ctx, &period))
	hostWeek := func(school int64, monday string) (week, model int64) {
		require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_period_weeks (organization_id, holiday_period_id, starts_on, ends_on, host_school_id)
			VALUES (?, ?, ?::date, ?::date + 4, ?) RETURNING id`, organization, period, monday, monday, school).Scan(ctx, &week))
		require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_week_time_models (organization_id, holiday_period_week_id, name, starts_at, ends_at, price_cents)
			VALUES (?, ?, 'bis 15 Uhr', '08:00', '15:00', 4500) RETURNING id`, organization, week).Scan(ctx, &model))
		return week, model
	}
	book := func(status, placement, first string, homeSchool any, week, model int64) {
		var registration, child int64
		require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registrations (organization_id, holiday_period_id, guardian_first_name,
				guardian_last_name, guardian_email, guardian_phone, privacy_consent_at, status)
			VALUES (?, ?, 'Eva', 'Muster', 'eva@example.org', '0123', NOW(), ?) RETURNING id`, organization, period, status).Scan(ctx, &registration))
		require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registration_children (organization_id, holiday_period_id,
				holiday_registration_id, position, first_name, last_name, birth_date, home_school_id)
			VALUES (?, ?, ?, 1, ?, 'Muster', '2018-03-04', ?) RETURNING id`, organization, period, registration, first, homeSchool).Scan(ctx, &child))
		_, err := db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_bookings (organization_id, holiday_period_id,
				holiday_registration_child_id, holiday_period_week_id, holiday_week_time_model_id, placement) VALUES (?, ?, ?, ?, ?, ?)`,
			organization, period, child, week, model, placement)
		require.NoError(t, err)
	}
	away, awayModel := hostWeek(host, "2099-01-04")
	book("confirmed", "placed", "Ole", home, away, awayModel)
	book("received", "placed", "Rita", home, away, awayModel)
	book("confirmed", "waitlisted", "Wanda", home, away, awayModel)
	book("cancelled", "placed", "Carl", home, away, awayModel)
	book("confirmed", "placed", "Gustav", host, away, awayModel)
	book("confirmed", "placed", "Fritz", nil, away, awayModel)
	// The home school hosts a week itself: its participant list shows
	// those children already.
	own, ownModel := hostWeek(home, "2099-01-11")
	book("confirmed", "placed", "Jan", home, own, ownModel)
	// For the other school that week is at another OGS: it reads its child.
	book("confirmed", "placed", "Ida", host, own, ownModel)
	// A week that ended: the access ended with its last day.
	past, pastModel := hostWeek(host, "2026-10-19")
	book("confirmed", "placed", "Paul", home, past, pastModel)

	// A school of another Träger hosting a week of that Träger never sees
	// the home school's children, nor the reverse.
	foreignOrganization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Träger Fremd", Slug: fmt.Sprintf("traeger-fremd-%d", testpkg.UniqueTestTenantID(t)), Active: true,
	}).ID
	foreign := testpkg.CreateTestSchoolInOrganization(t, db, foreignOrganization, "GS Fremd")

	type row struct {
		FirstName     string `bun:"first_name"`
		HostSchoolID  int64  `bun:"host_school_id"`
		StartsOn      string `bun:"starts_on"`
		TimeModelName string `bun:"time_model_name"`
		StartsAt      string `bun:"starts_at"`
	}
	asSchool := func(school any) []row {
		var rows []row
		require.NoError(t, db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant`); err != nil {
				return err
			}
			if school != nil {
				if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id', ?, true)`, fmt.Sprint(school)); err != nil {
					return err
				}
			}
			return tx.NewRaw(`SELECT first_name, host_school_id, starts_on::text AS starts_on, time_model_name, starts_at
				FROM enrollment.holiday_home_school_bookings() ORDER BY first_name`).Scan(ctx, &rows)
		}))
		return rows
	}
	assert.Equal(t, []row{{FirstName: "Ole", HostSchoolID: host, StartsOn: "2099-01-04", TimeModelName: "bis 15 Uhr", StartsAt: "08:00"}}, asSchool(home),
		"only the confirmed, placed child at another school's open week")
	hostNames := []string{}
	for _, entry := range asSchool(host) {
		hostNames = append(hostNames, entry.FirstName)
	}
	assert.Equal(t, []string{"Ida"}, hostNames, "each school reads its own children at the other school's week")
	assert.Empty(t, asSchool(foreign), "a school of another Träger reads nothing")
	assert.Empty(t, asSchool(nil), "outside a school transaction the function reads nothing")
}
