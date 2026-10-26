package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestHolidayGuestContactsReachOnlyTheHostOfAConfirmedGuest pins ADR 0120 on
// the contacts of the Träger's holiday registrations: under the request role
// the school that hosts an open week of a published period reads the
// emergency contacts and pickup persons of its confirmed guests through
// enrollment.holiday_host_guest_contacts, and nothing of its own children,
// of an unconfirmed registration, of an ended week, or as a school that does
// not host the week. The guest tables belong to the hosting school's tenant.
func TestHolidayGuestContactsReachOnlyTheHostOfAConfirmedGuest(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	slug := fmt.Sprintf("traeger-feriengaeste-%d", testpkg.UniqueTestTenantID(t))
	own := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger Feriengäste", Slug: slug}).ID
	testpkg.OwnTraegerHolidayPeriods(t, db, own)
	host := testpkg.CreateTestSchoolInOrganization(t, db, own, "Träger Feriengäste OGS Nord")
	home := testpkg.CreateTestSchoolInOrganization(t, db, own, "Träger Feriengäste OGS Süd")

	var period int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_periods (organization_id, name, registration_opens_on,
			registration_closes_on, status, published_at)
		VALUES (?, 'Herbstferien', '2020-01-01', '2099-06-30', 'published', NOW()) RETURNING id`, own).Scan(ctx, &period))
	insertWeek := func(startsOn, endsOn string) (int64, int64) {
		var week, model int64
		require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_period_weeks (organization_id, holiday_period_id, starts_on, ends_on, host_school_id)
			VALUES (?, ?, ?, ?, ?) RETURNING id`, own, period, startsOn, endsOn, host).Scan(ctx, &week))
		require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_week_time_models (organization_id, holiday_period_week_id, name, starts_at, ends_at, price_cents)
			VALUES (?, ?, 'bis 13 Uhr', '08:00', '13:00', 4500) RETURNING id`, own, week).Scan(ctx, &model))
		return week, model
	}
	open, openModel := insertWeek("2099-10-12", "2099-10-16")
	ended, endedModel := insertWeek("2020-10-12", "2020-10-16")
	register := func(status, contact string, homeSchool any, week, model int64) {
		var registration, child int64
		require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registrations (organization_id, holiday_period_id, guardian_first_name,
				guardian_last_name, guardian_email, guardian_phone, privacy_consent_at, status)
			VALUES (?, ?, 'Eva', 'Muster', 'eva@example.org', '0123', NOW(), ?) RETURNING id`, own, period, status).Scan(ctx, &registration))
		require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registration_children (organization_id, holiday_period_id,
				holiday_registration_id, position, first_name, last_name, birth_date, home_school_id)
			VALUES (?, ?, ?, 1, 'Ben', 'Muster', '2016-09-12', ?) RETURNING id`, own, period, registration, homeSchool).Scan(ctx, &child))
		_, err := db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_contacts (organization_id, holiday_registration_id, kind, position, name)
			VALUES (?, ?, 'emergency', 1, ?), (?, ?, 'pickup', 1, ?)`, own, registration, contact, own, registration, "Abholung "+contact)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_bookings (organization_id, holiday_period_id,
				holiday_registration_child_id, holiday_period_week_id, holiday_week_time_model_id) VALUES (?, ?, ?, ?, ?)`,
			own, period, child, week, model)
		require.NoError(t, err)
	}
	register("confirmed", "Gast ohne Schule", nil, open, openModel)
	register("confirmed", "Gast aus Süd", home, open, openModel)
	register("confirmed", "Kind der OGS Nord", host, open, openModel)
	register("received", "Nicht bestätigt", nil, open, openModel)
	register("confirmed", "Woche vorbei", nil, ended, endedModel)

	asSchool := func(school int64, fn func(bun.Tx) error) error {
		return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant`); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id', ?, true)`, fmt.Sprint(school)); err != nil {
				return err
			}
			return fn(tx)
		})
	}
	contacts := func(school, week int64) []string {
		var names []string
		require.NoError(t, asSchool(school, func(tx bun.Tx) error {
			return tx.NewRaw(`SELECT kind || ':' || name FROM enrollment.holiday_host_guest_contacts(?, ?) ORDER BY name, kind`,
				own, week).Scan(ctx, &names)
		}))
		return names
	}
	assert.Equal(t, []string{
		"pickup:Abholung Gast aus Süd", "pickup:Abholung Gast ohne Schule",
		"emergency:Eva Muster", "emergency:Eva Muster",
		"emergency:Gast aus Süd", "emergency:Gast ohne Schule",
	}, contacts(host, open),
		"the host reads the parent and the contacts of its confirmed guests, not of its own child or an unconfirmed one")
	err := asSchool(host, func(tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `SELECT guardian_email FROM enrollment.holiday_host_guest_contacts(?, ?)`, own, open)
		return err
	})
	assert.ErrorContains(t, err, "guardian_email", "the host reads no e-mail address of the parents")
	assert.Empty(t, contacts(host, ended), "the access ends after the last day of the week")
	assert.Empty(t, contacts(home, open), "a school that does not host the week reads no contact")

	group := testpkg.CreateTestEducationGroupForTenant(t, db, host, "Feriengruppe").ID
	_, err = db.ExecContext(ctx, `INSERT INTO enrollment.holiday_host_groups (tenant_id, organization_id, holiday_period_week_id, group_id, assigned_by_account_id)
		VALUES (?, ?, ?, ?, 1)`, host, own, open, group)
	require.NoError(t, err)
	countGroups := func(school int64) int {
		var count int
		require.NoError(t, asSchool(school, func(tx bun.Tx) error {
			return tx.NewRaw(`SELECT COUNT(*) FROM enrollment.holiday_host_groups`).Scan(ctx, &count)
		}))
		return count
	}
	assert.Equal(t, 1, countGroups(host), "the host reads the group it picked")
	assert.Zero(t, countGroups(home), "another school reads no group of the host")

	err = asSchool(home, func(tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO enrollment.holiday_host_groups (tenant_id, organization_id, holiday_period_week_id, group_id, assigned_by_account_id)
			VALUES (?, ?, ?, ?, 1)`, host, own, ended, group)
		return err
	})
	assert.ErrorContains(t, err, "row-level security", "a school writes no guest data of another school")
}

// rollBackHolidayGuests rolls back 1.15.681 and 1.15.680 in an isolated
// database, for a test that rolls back an older migration which reinstalls
// the school erasure functions without the holiday guest tables.
func rollBackHolidayGuests(t *testing.T, db *bun.DB) {
	t.Helper()
	require.NoError(t, holidayGuestErasureDown(t.Context(), db))
	require.NoError(t, holidayGuestsDown(t.Context(), db))
}

// TestHolidayGuestContactHoldsTheFullNameOfTheParent pins the length of a
// copied contact: the registering parent becomes the first emergency contact
// with first and last name (100 characters each) and the space between them.
func TestHolidayGuestContactHoldsTheFullNameOfTheParent(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	school := testpkg.Tenant(t)
	student := testpkg.CreateTestStudentForTenant(t, db, school, "Ben", "Muster", "Feriengast")
	var guest int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_guests (tenant_id, organization_id, holiday_period_week_id,
			holiday_registration_child_id, student_id, starts_on, ends_on, time_model_name, time_model_starts_at, time_model_ends_at)
		VALUES (?, 1, 1, 1, ?, '2099-07-06', '2099-07-10', 'bis 13 Uhr', '08:00', '13:00') RETURNING id`, school, student.ID).Scan(ctx, &guest))
	name := strings.Repeat("E", 100) + " " + strings.Repeat("M", 100)
	_, err := db.ExecContext(ctx, `INSERT INTO enrollment.holiday_guest_contacts (tenant_id, holiday_guest_id, kind, position, name)
		VALUES (?, ?, 'emergency', 1, ?)`, school, guest, name)
	require.NoError(t, err, "a parent with the longest names the registration accepts is copied in full")
	_, err = db.ExecContext(ctx, `INSERT INTO enrollment.holiday_guest_contacts (tenant_id, holiday_guest_id, kind, position, name)
		VALUES (?, ?, 'emergency', 2, ?)`, school, guest, name+"X")
	assert.Error(t, err, "a longer name is refused")
}
