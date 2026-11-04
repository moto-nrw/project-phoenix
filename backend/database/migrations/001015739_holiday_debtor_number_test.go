package migrations

import (
	"context"
	"fmt"
	"strings"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHolidayDebtorNumberMigrationKeepsRegistrationsInvoicesAndTrail pins
// #5621: an existing registration, an issued invoice and the audit trail
// stay unchanged and have no debtor number; the CHECKs take a trimmed number
// of at most 35 characters without a line break; the trail takes the action
// "debtor_number"; an issued invoice still refuses every change.
func TestHolidayDebtorNumberMigrationKeepsRegistrationsInvoicesAndTrail(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Ferien Debitor", Slug: fmt.Sprintf("ferien-debitor-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	testpkg.OwnTraegerHolidayPeriods(t, db, organization)
	testpkg.OwnTraegerContributionRules(t, db, organization)
	require.NoError(t, holidayDebtorNumberDown(ctx, db))

	var period, registration, run, document int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_periods
  (organization_id, name, registration_opens_on, registration_closes_on, status, published_at)
  VALUES (?, 'Winterferien', '2026-01-01', '2099-12-31', 'published', now()) RETURNING id`, organization).Scan(ctx, &period))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registrations
  (organization_id, holiday_period_id, guardian_first_name, guardian_last_name, guardian_email, guardian_phone, privacy_consent_at, status)
  VALUES (?, ?, 'Eva', 'Muster', 'eva@example.org', '0151 2345678', now(), 'confirmed') RETURNING id`, organization, period).Scan(ctx, &registration))
	_, err := db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_audit
  (organization_id, holiday_period_id, holiday_registration_id, action, details)
  VALUES (?, ?, ?, 'confirm', '{"from":"received"}')`, organization, period, registration)
	require.NoError(t, err)
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.runs (organization_id, service, holiday_period_id, period_name, service_from,
   service_until, status, released_at, issued_on, document_count, total_cents)
  VALUES (?, 'holiday', ?, 'Winterferien', '2027-01-04', '2027-01-08', 'released', now(), '2026-12-01', 1, 4500) RETURNING id`,
		organization, period).Scan(ctx, &run))
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.documents (organization_id, run_id, service, number, number_year, number_sequence,
   issued_on, holiday_period_id, holiday_registration_id, recipient_first_name, recipient_last_name, recipient_email,
   service_from, service_until, total_cents)
  VALUES (?, ?, 'holiday', 'FE-0001', 0, 1, '2026-12-01', ?, ?, 'Eva', 'Muster', 'eva@example.org', '2027-01-04', '2027-01-08', 4500)
  RETURNING id`, organization, run, period, registration).Scan(ctx, &document))

	require.NoError(t, holidayDebtorNumberUp(ctx, db))

	var preserved bool
	require.NoError(t, db.NewRaw(`SELECT
   EXISTS (SELECT 1 FROM enrollment.holiday_registrations WHERE id = ? AND status = 'confirmed' AND debtor_number IS NULL)
   AND EXISTS (SELECT 1 FROM parent_billing.documents WHERE id = ? AND number = 'FE-0001' AND total_cents = 4500 AND debtor_number IS NULL)
   AND (SELECT count(*) FROM enrollment.holiday_registration_audit WHERE holiday_registration_id = ? AND action = 'confirm') = 1`,
		registration, document, registration).Scan(ctx, &preserved))
	assert.True(t, preserved, "existing registrations, invoices and their trail stay unchanged and have no debtor number")

	for _, refused := range []string{"", " 4711", "4711 ", "10\n4711", "10\r4711", strings.Repeat("9", 36)} {
		_, err := db.ExecContext(ctx, `UPDATE enrollment.holiday_registrations SET debtor_number = ? WHERE id = ?`, refused, registration)
		assert.Error(t, err, "%q", refused)
	}
	_, err = db.ExecContext(ctx, `UPDATE enrollment.holiday_registrations SET debtor_number = ? WHERE id = ?`, strings.Repeat("ä", 35), registration)
	require.NoError(t, err, "35 characters fit")
	_, err = db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_audit
  (organization_id, holiday_period_id, holiday_registration_id, action, details)
  VALUES (?, ?, ?, 'debtor_number', '{"debtor_number":{"before":"","after":"4711"}}')`, organization, period, registration)
	require.NoError(t, err, "the trail takes a change of the debtor number")
	_, err = db.ExecContext(ctx, `UPDATE parent_billing.documents SET debtor_number = '4711' WHERE id = ?`, document)
	assert.ErrorContains(t, err, "never changes", "an issued invoice still refuses every change")
}
