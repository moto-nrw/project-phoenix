package migrations

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

func TestHolidayChildIdentityBackfillPreservesIndividualNumbers(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	newPeriod := func() (int64, int64) {
		org := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Ferienkinder", Slug: fmt.Sprintf("ferienkinder-%d", testpkg.UniqueTestTenantID(t))}).ID
		var period int64
		require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_periods (organization_id, name, registration_opens_on, registration_closes_on)
			VALUES (?, 'Sommerferien', '2099-05-03', '2099-06-04') RETURNING id`, org).Scan(ctx, &period))
		return org, period
	}
	own, period := newPeriod()
	foreign, foreignPeriod := newPeriod()
	child := func(org, period int64, email, name, status string) int64 {
		var registration, id int64
		require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registrations (organization_id, holiday_period_id,
			guardian_first_name, guardian_last_name, guardian_email, guardian_phone, privacy_consent_at, status, debtor_number)
			VALUES (?, ?, 'Eva', 'Muster', ?, '0123', NOW(), ?, '70001') RETURNING id`, org, period, email, status).Scan(ctx, &registration))
		require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registration_children (organization_id, holiday_period_id,
			holiday_registration_id, position, first_name, last_name, birth_date)
			VALUES (?, ?, ?, 1, ?, 'Muster', '2018-03-01') RETURNING id`, org, period, registration, name).Scan(ctx, &id))
		return id
	}
	mia := child(own, period, "EVA@example.org", "Mia", "confirmed")
	child(own, period, "eva@example.org", "MIA", "confirmed")
	child(own, period, "eva@example.org", "Ben", "confirmed")
	child(own, period, "eva@example.org", "Offen", "received")
	child(own, period, "eva@example.org", "Abgesagt", "cancelled")
	child(foreign, foreignPeriod, "eva@example.org", "Mia", "confirmed")

	school := testpkg.CreateTestSchoolInOrganization(t, db, own, "Ferien OGS")
	var week, model int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_period_weeks (organization_id, holiday_period_id, starts_on, ends_on, host_school_id)
		VALUES (?, ?, '2099-07-06', '2099-07-10', ?) RETURNING id`, own, period, school).Scan(ctx, &week))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_week_time_models (organization_id, holiday_period_week_id, name, starts_at, ends_at, price_cents)
		VALUES (?, ?, 'Ganztag', '08:00', '16:00', 4500) RETURNING id`, own, week).Scan(ctx, &model))
	_, err := db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_bookings
		(organization_id, holiday_period_id, holiday_registration_child_id, holiday_period_week_id, holiday_week_time_model_id)
		VALUES (?, ?, ?, ?, ?)`, own, period, mia, week, model)
	require.NoError(t, err)
	key := "mia\x1fmuster\x1f2018-03-01"
	_, err = db.ExecContext(ctx, `INSERT INTO parent_billing.child_debtor_numbers
		(organization_id, family_email, child_key, child_first_name, child_last_name, child_birth_date, debtor_number, last_used_on)
		VALUES (?, 'eva@example.org', ?, 'Mia', 'Muster', '2018-03-01', '80001', '2020-01-01')`, own, key)
	require.NoError(t, err)

	require.NoError(t, holidayChildIdentitiesUp(ctx, db))
	require.NoError(t, holidayChildIdentitiesUp(ctx, db), "replaying the data repair is harmless")
	var rows []struct {
		OrganizationID int64
		FamilyEmail    string
		ChildKey       string
		DebtorNumber   string
		LastUsedOn     calendar.Date `bun:"last_used_on,type:date"`
	}
	require.NoError(t, db.NewRaw(`SELECT organization_id, family_email, child_key, debtor_number, last_used_on
		FROM parent_billing.child_debtor_numbers WHERE organization_id IN (?, ?) ORDER BY organization_id, child_key`, own, foreign).Scan(ctx, &rows))
	require.Len(t, rows, 3, "confirmed children only, deduplicated within each Träger")
	for _, row := range rows {
		assert.Equal(t, "eva@example.org", row.FamilyEmail)
		if row.OrganizationID == own && row.ChildKey == key {
			assert.Equal(t, "80001", row.DebtorNumber)
			assert.Equal(t, calendar.NewDate(2099, 7, 10), row.LastUsedOn)
		} else {
			assert.Empty(t, row.DebtorNumber, "neither a shared number nor another Träger's individual number is copied")
		}
	}
	var events int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM parent_billing.child_debtor_number_events WHERE organization_id IN (?, ?)`, own, foreign).Scan(ctx, &events))
	assert.Equal(t, 3, events, "each written identity is recorded once")
	require.NoError(t, holidayChildIdentitiesDown(ctx, db))
	var preserved string
	require.NoError(t, db.NewRaw(`SELECT debtor_number FROM parent_billing.child_debtor_numbers
		WHERE organization_id = ? AND child_key = ?`, own, key).Scan(ctx, &preserved))
	assert.Equal(t, "80001", preserved, "rollback never removes an individual number")
}
