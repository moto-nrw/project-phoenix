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
// another number stay as they are. A rollback deletes what has no contract.
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
	register := func(first, email, status string, number any, submitted string) {
		t.Helper()
		_, err := db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registrations
  (organization_id, holiday_period_id, guardian_first_name, guardian_last_name, guardian_email, guardian_phone,
   privacy_consent_at, status, debtor_number, submitted_at)
  VALUES (?, ?, ?, 'Muster', ?, '0151 2345678', now(), ?, ?, ?::timestamptz)`,
			organization, period, first, email, status, number, submitted)
		require.NoError(t, err)
	}
	register("Eva", "Eva.Muster@example.org ", "confirmed", "10 4711", "2026-06-01 10:00+02")
	register("Paul", "eva.muster@example.org", "confirmed", "10 4711", "2026-06-02 10:00+02")
	register("Paul", "eva.muster@example.org", "confirmed", nil, "2026-06-03 10:00+02")
	register("Ina", "zwei@example.org", "confirmed", "1", "2026-06-01 10:00+02")
	register("Ina", "zwei@example.org", "confirmed", "2", "2026-06-02 10:00+02")
	register("Rita", "offen@example.org", "received", "77", "2026-06-01 10:00+02")

	require.NoError(t, familyDebtorNumberUp(ctx, db))

	type family struct {
		Email   string  `bun:"family_email"`
		Number  *string `bun:"debtor_number"`
		First   string  `bun:"holder_first_name"`
		Last    string  `bun:"holder_last_name"`
		Kind    string  `bun:"recipient_kind"`
		Retains bool    `bun:"retains"`
		Events  int     `bun:"events"`
	}
	var families []family
	require.NoError(t, db.NewRaw(`SELECT family.family_email, family.debtor_number, family.holder_first_name, family.holder_last_name,
   family.recipient_kind, family.last_used_on IS NOT NULL AND family.care_contract_id IS NULL AS retains,
   (SELECT count(*) FROM parent_billing.family_billing_events AS event
    WHERE event.family_billing_id = family.id AND event.action = 'save' AND event.actor_account_id IS NULL) AS events
  FROM parent_billing.family_billing AS family WHERE family.organization_id = ? ORDER BY family.family_email`, organization).
		Scan(ctx, &families))
	number := "10 4711"
	assert.Equal(t, []family{{Email: "eva.muster@example.org", Number: &number, First: "Paul", Last: "Muster", Kind: "applicant",
		Retains: true, Events: 1}}, families,
		"one family with one number remembers it with the parent who registered last; two numbers and a received registration stay out")

	require.NoError(t, familyDebtorNumberDown(ctx, db))
	var left int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM parent_billing.family_billing WHERE organization_id = ?`, organization).Scan(ctx, &left))
	assert.Zero(t, left, "a rollback deletes the rows without a contract")
}
