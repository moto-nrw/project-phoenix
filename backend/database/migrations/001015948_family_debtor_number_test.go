package migrations

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestFamilyDebtorNumberBackfillTakesOnlyAnUnambiguousNumber pins the
// backfill of #6692: a family whose confirmed holiday registrations name one
// number remembers it, keyed by the trimmed lower-case address, with the
// name of the parent who registered last and a "save" in its trail; a family
// with two numbers, a received registration and a family whose row names
// another number stay as they are. The number is kept from the last week
// its registrations booked, not from the day of the migration, and it came
// from the family's first registration that named it. A rollback deletes
// what has no contract.
func TestFamilyDebtorNumberBackfillTakesOnlyAnUnambiguousNumber(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Ferien Familie", Slug: fmt.Sprintf("ferien-familie-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	testpkg.OwnTraegerHolidayPeriods(t, db, organization)
	require.NoError(t, familyDebtorNumberDown(ctx, db))

	var period int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_periods
  (organization_id, name, registration_opens_on, registration_closes_on, status, published_at)
  VALUES (?, 'Sommerferien', '2026-01-01', '2099-12-31', 'published', now()) RETURNING id`, organization).Scan(ctx, &period))
	school := testpkg.CreateTestSchoolInOrganization(t, db, organization, "Grundschule Ferien")
	week := func(startsOn, endsOn string) (int64, int64) {
		t.Helper()
		var weekID, modelID int64
		require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_period_weeks (organization_id, holiday_period_id, starts_on, ends_on, host_school_id)
  VALUES (?, ?, ?::date, ?::date, ?) RETURNING id`, organization, period, startsOn, endsOn, school).Scan(ctx, &weekID))
		require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_week_time_models (organization_id, holiday_period_week_id, name, starts_at, ends_at, price_cents)
  VALUES (?, ?, 'Ganztag', '08:00', '16:00', 0) RETURNING id`, organization, weekID).Scan(ctx, &modelID))
		return weekID, modelID
	}
	book := func(registration int64, weeks ...[2]int64) {
		t.Helper()
		var child int64
		require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registration_children (organization_id, holiday_period_id,
   holiday_registration_id, position, first_name, last_name, birth_date) VALUES (?, ?, ?, 1, 'Lina', 'Muster', '2018-03-01') RETURNING id`,
			organization, period, registration).Scan(ctx, &child))
		for _, booked := range weeks {
			_, err := db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_bookings (organization_id, holiday_period_id,
   holiday_registration_child_id, holiday_period_week_id, holiday_week_time_model_id) VALUES (?, ?, ?, ?, ?)`,
				organization, period, child, booked[0], booked[1])
			require.NoError(t, err)
		}
	}
	first, firstModel := week("2026-07-06", "2026-07-10")
	second, secondModel := week("2026-07-13", "2026-07-17")
	third, thirdModel := week("2027-07-19", "2027-07-23")
	register := func(first, email, status string, number any, submitted string) int64 {
		t.Helper()
		var id int64
		require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registrations
  (organization_id, holiday_period_id, guardian_first_name, guardian_last_name, guardian_email, guardian_phone,
   privacy_consent_at, status, debtor_number, submitted_at)
  VALUES (?, ?, ?, 'Muster', ?, '0151 2345678', now(), ?, ?, ?::timestamptz) RETURNING id`,
			organization, period, first, email, status, number, submitted).Scan(ctx, &id))
		return id
	}
	register("Eva", "eva.muster@example.org", "confirmed", nil, "2026-05-01 10:00+02")
	origin := register("Eva", "Eva.Muster@example.org ", "confirmed", "10 4711", "2026-06-01 10:00+02")
	book(origin, [2]int64{first, firstModel})
	book(register("Paul", "eva.muster@example.org", "confirmed", "10 4711", "2026-06-02 10:00+02"), [2]int64{second, secondModel})
	book(register("Paula", "eva.muster@example.org", "confirmed", nil, "2027-06-03 10:00+02"), [2]int64{third, thirdModel})
	register("Ina", "zwei@example.org", "confirmed", "1", "2026-06-01 10:00+02")
	register("Ina", "zwei@example.org", "confirmed", "2", "2026-06-02 10:00+02")
	register("Rita", "offen@example.org", "received", "77", "2026-06-01 10:00+02")
	register("Nina", "ohne@example.org", "confirmed", nil, "2027-06-01 10:00+02")

	require.NoError(t, familyDebtorNumberUp(ctx, db))

	type family struct {
		Email   string  `bun:"family_email"`
		Number  *string `bun:"debtor_number"`
		First   string  `bun:"holder_first_name"`
		Last    string  `bun:"holder_last_name"`
		Kind    string  `bun:"recipient_kind"`
		Retains bool    `bun:"retains"`
		UsedOn  string  `bun:"used_on"`
		Origin  *int64  `bun:"debtor_number_registration_id"`
		Events  int     `bun:"events"`
	}
	var families []family
	require.NoError(t, db.NewRaw(`SELECT family.family_email, family.debtor_number, family.holder_first_name, family.holder_last_name,
   family.recipient_kind, family.last_used_on IS NOT NULL AND family.care_contract_id IS NULL AS retains,
   to_char(family.last_used_on, 'YYYY-MM-DD') AS used_on, family.debtor_number_registration_id,
   (SELECT count(*) FROM parent_billing.family_billing_events AS event
    WHERE event.family_billing_id = family.id AND event.action = 'save' AND event.actor_account_id IS NULL) AS events
  FROM parent_billing.family_billing AS family WHERE family.organization_id = ? ORDER BY family.family_email`, organization).
		Scan(ctx, &families))
	number := "10 4711"
	assert.Equal(t, []family{{Email: "eva.muster@example.org", Number: &number, First: "Paula", Last: "Muster", Kind: "applicant",
		Retains: true, UsedOn: "2027-07-23", Origin: &origin, Events: 1}}, families,
		"one family with one number remembers it with the parent who registered last, until the last week a registration "+
			"booked even without its own number, from its first numbered registration; two numbers and families without a number stay out")

	// Existing contract numbers that match holiday usage need retention too,
	// but never become registration-owned merely because the values match.
	testpkg.OwnTraegerCareContracts(t, db, organization)
	var contract int64
	require.NoError(t, db.NewRaw(`INSERT INTO care_contract.contracts (organization_id, school_id, request_child_id, status)
  VALUES (?, ?, 21, 'entwurf') RETURNING id`, organization, school).Scan(ctx, &contract))
	register("Mara", "matching@example.org", "confirmed", "30100", "2026-06-04 10:00+02")
	register("Nora", "different@example.org", "confirmed", "40100", "2026-06-05 10:00+02")
	book(register("Lara", "inherited@example.org", "confirmed", nil, "2027-06-06 10:00+02"), [2]int64{third, thirdModel})
	register("Lara", "inherited@example.org", "confirmed", nil, "2028-01-01 00:30+01")
	register("Lara", "inherited@example.org", "received", nil, "2029-06-06 10:00+02")
	register("Nina", "empty@example.org", "confirmed", nil, "2027-06-06 10:00+02")
	_, err := db.ExecContext(ctx, `INSERT INTO parent_billing.family_billing (organization_id, family_email, care_contract_id, recipient_kind, debtor_number)
  VALUES (?0, 'matching@example.org', ?1, 'applicant', '30100'), (?0, 'different@example.org', ?1, 'applicant', '50100'),
   (?0, 'inherited@example.org', ?1, 'applicant', '60100'), (?0, 'empty@example.org', ?1, 'applicant', NULL)`, organization, contract)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, familyDebtorNumberBackfillSQL)
	require.NoError(t, err)
	var matching struct {
		Holder string `bun:"holder_first_name"`
		UsedOn string `bun:"used_on"`
		Origin *int64 `bun:"debtor_number_registration_id"`
	}
	require.NoError(t, db.NewRaw(`SELECT holder_first_name, COALESCE(last_used_on::text, '') AS used_on, debtor_number_registration_id
  FROM parent_billing.family_billing WHERE organization_id = ? AND family_email = 'matching@example.org'`, organization).Scan(ctx, &matching))
	assert.Equal(t, "Mara", matching.Holder)
	assert.Equal(t, "2026-06-04", matching.UsedOn)
	assert.Nil(t, matching.Origin, "an independently assigned matching number keeps its provenance")
	var inherited struct {
		UsedOn string `bun:"used_on"`
		Origin *int64 `bun:"debtor_number_registration_id"`
	}
	require.NoError(t, db.NewRaw(`SELECT last_used_on::text AS used_on, debtor_number_registration_id
  FROM parent_billing.family_billing WHERE organization_id = ? AND family_email = 'inherited@example.org'`, organization).Scan(ctx, &inherited))
	assert.Equal(t, "2028-01-01", inherited.UsedOn, "numberless use keeps the later of the last booked day and Berlin submission date; received registrations stay out")
	assert.Nil(t, inherited.Origin, "inherited use does not take ownership of the existing number")
	var different string
	require.NoError(t, db.NewRaw(`SELECT debtor_number FROM parent_billing.family_billing WHERE organization_id = ?
  AND family_email = 'different@example.org'`, organization).Scan(ctx, &different))
	assert.Equal(t, "50100", different, "independently assigned numbers stay unchanged")
	_, err = db.ExecContext(ctx, `DELETE FROM care_contract.contracts WHERE organization_id = ? AND id = ?`, organization, contract)
	require.NoError(t, err)
	var retained int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM parent_billing.family_billing WHERE organization_id = ?
  AND family_email = 'matching@example.org' AND care_contract_id IS NULL AND debtor_number = '30100'`, organization).Scan(ctx, &retained))
	assert.Equal(t, 1, retained, "holiday use keeps the matching number after contract deletion")
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM parent_billing.family_billing WHERE organization_id = ?
  AND family_email = 'inherited@example.org' AND care_contract_id IS NULL AND debtor_number = '60100'`, organization).Scan(ctx, &retained))
	assert.Equal(t, 1, retained, "numberless holiday use keeps the inherited number after contract deletion")
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM parent_billing.family_billing WHERE organization_id = ?
  AND family_email = 'empty@example.org'`, organization).Scan(ctx, &retained))
	assert.Zero(t, retained, "numberless use does not retain a family row without a number")

	require.NoError(t, familyDebtorNumberDown(ctx, db))
	var left int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM parent_billing.family_billing WHERE organization_id = ?`, organization).Scan(ctx, &left))
	assert.Zero(t, left, "a rollback deletes the rows without a contract")
}
