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

// TestHolidayRegistrationsFollowTheTraegerPolicy pins ADR 0110 on the
// registration tables and the outbox: under the request role a Träger
// transaction sees and writes only its own Träger's registrations and mails,
// a school transaction sees none, only an undeleted school of the Träger is
// a home school, and a booked week cannot be removed.
func TestHolidayRegistrationsFollowTheTraegerPolicy(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	newTraeger := func(name string) (int64, int64) {
		slug := fmt.Sprintf("traeger-anmeldung-%d", testpkg.UniqueTestTenantID(t))
		organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: name, Slug: slug}).ID
		testpkg.OwnTraegerHolidayPeriods(t, db, organization)
		return organization, testpkg.CreateTestSchoolInOrganization(t, db, organization, name+" OGS")
	}
	own, ownSchool := newTraeger("Träger Anmeldung Eigen")
	foreign, foreignSchool := newTraeger("Träger Anmeldung Fremd")

	var period, week, model, registration int64
	var token string
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_periods (organization_id, name, registration_opens_on, registration_closes_on)
		VALUES (?, 'Sommerferien', '2027-05-03', '2027-06-04') RETURNING id, public_token`, own).Scan(ctx, &period, &token))
	assert.Len(t, token, 64, "each period has its own random link key")
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_period_weeks (organization_id, holiday_period_id, starts_on, ends_on, host_school_id)
		VALUES (?, ?, '2027-07-05', '2027-07-09', ?) RETURNING id`, own, period, ownSchool).Scan(ctx, &week))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_week_time_models (organization_id, holiday_period_week_id, name, starts_at, ends_at, price_cents)
		VALUES (?, ?, 'bis 13 Uhr', '08:00', '13:00', 4500) RETURNING id`, own, week).Scan(ctx, &model))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registrations (organization_id, holiday_period_id, guardian_first_name,
			guardian_last_name, guardian_email, guardian_phone, privacy_consent_at)
		VALUES (?, ?, 'Eva', 'Muster', 'eva@example.org', '0123', NOW()) RETURNING id`, own, period).Scan(ctx, &registration))
	insertChild := func(homeSchool any) (int64, error) {
		var child int64
		err := db.NewRaw(`INSERT INTO enrollment.holiday_registration_children (organization_id, holiday_period_id,
				holiday_registration_id, position, first_name, last_name, birth_date, home_school_id)
			VALUES (?, ?, ?, (SELECT COUNT(*) + 1 FROM enrollment.holiday_registration_children WHERE holiday_registration_id = ?),
				'Lia', 'Muster', '2018-03-01', ?) RETURNING id`, own, period, registration, registration, homeSchool).Scan(ctx, &child)
		return child, err
	}
	child, err := insertChild(ownSchool)
	require.NoError(t, err)
	_, err = insertChild(nil)
	require.NoError(t, err, "a holiday guest has no home school")
	_, err = insertChild(foreignSchool)
	assert.ErrorContains(t, err, "is not a school of Träger", "a foreign school is no home school")
	_, err = db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_bookings (organization_id, holiday_period_id,
			holiday_registration_child_id, holiday_period_week_id, holiday_week_time_model_id) VALUES (?, ?, ?, ?, ?)`,
		own, period, child, week, model)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `DELETE FROM enrollment.holiday_period_weeks WHERE id = ?`, week)
	assert.ErrorContains(t, err, "fk_holiday_registration_bookings", "a booked week cannot be removed")
	_, err = db.ExecContext(ctx, `INSERT INTO platform.email_outbox (organization_id, kind, recipient, payload)
		VALUES (?, 'holiday_registration_received', '{}'::jsonb, '{}'::jsonb)`, own)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO platform.email_outbox (tenant_id, organization_id, kind, recipient, payload)
		VALUES (?, ?, 'holiday_registration_received', '{}'::jsonb, '{}'::jsonb)`, ownSchool, own)
	assert.ErrorContains(t, err, "chk_email_outbox_one_scope", "a mail has a school or a Träger, not both")

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
	count := func(setting, value string) int {
		var visible int
		require.NoError(t, asRequestRole(setting, value, func(tx bun.Tx) error {
			return tx.NewRaw(`SELECT
				(SELECT COUNT(*) FROM enrollment.holiday_registrations WHERE organization_id = ?)
				+ (SELECT COUNT(*) FROM enrollment.holiday_registration_children WHERE organization_id = ?)
				+ (SELECT COUNT(*) FROM enrollment.holiday_registration_bookings WHERE organization_id = ?)
				+ (SELECT COUNT(*) FROM platform.email_outbox WHERE organization_id = ?)`, own, own, own, own).Scan(ctx, &visible)
		}))
		return visible
	}
	assert.Equal(t, 5, count("app.current_org_id", fmt.Sprint(own)), "the Träger sees its registration, children, booking and mail")
	assert.Zero(t, count("app.current_org_id", fmt.Sprint(foreign)), "another Träger sees nothing")
	assert.Zero(t, count("app.current_tenant_id", fmt.Sprint(ownSchool)), "a school transaction sees nothing of its Träger's registrations")

	err = asRequestRole("app.current_org_id", fmt.Sprint(foreign), func(tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO platform.email_outbox (organization_id, kind, recipient, payload)
			VALUES (?, 'holiday_registration_received', '{}'::jsonb, '{}'::jsonb)`, own)
		return err
	})
	assert.ErrorContains(t, err, "row-level security", "a Träger cannot queue a mail for another Träger")
}
