package migrations

import (
	"context"
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHolidayGuardianAddressMigrationKeepsFormsRegistrationsAndInvoices pins
// #5625: an existing form version asks for no address, an existing
// registration and an issued invoice have none and stay unchanged, the
// CHECKs take all three parts with a five-digit postal code or none, and an
// issued invoice still refuses every change.
func TestHolidayGuardianAddressMigrationKeepsFormsRegistrationsAndInvoices(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Ferienanschrift", Slug: fmt.Sprintf("ferien-anschrift-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	testpkg.OwnTraegerHolidayPeriods(t, db, organization)
	testpkg.OwnTraegerContributionRules(t, db, organization)
	require.NoError(t, holidayGuardianAddressDown(ctx, db))

	var period, registration, run, document int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_periods
  (organization_id, name, registration_opens_on, registration_closes_on, status, published_at)
  VALUES (?, 'Winterferien', '2026-01-01', '2099-12-31', 'published', now()) RETURNING id`, organization).Scan(ctx, &period))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registrations
  (organization_id, holiday_period_id, guardian_first_name, guardian_last_name, guardian_email, guardian_phone, privacy_consent_at)
  VALUES (?, ?, 'Eva', 'Muster', 'eva@example.org', '0151 2345678', now()) RETURNING id`, organization, period).Scan(ctx, &registration))
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.runs (organization_id, service, holiday_period_id, period_name, service_from,
   service_until, status, released_at, issued_on, document_count, total_cents)
  VALUES (?, 'holiday', ?, 'Winterferien', '2027-01-04', '2027-01-08', 'released', now(), '2026-12-01', 1, 4500) RETURNING id`,
		organization, period).Scan(ctx, &run))
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.documents (organization_id, run_id, service, number, number_year, number_sequence,
   issued_on, holiday_period_id, holiday_registration_id, recipient_first_name, recipient_last_name, recipient_email,
   service_from, service_until, total_cents)
  VALUES (?, ?, 'holiday', 'FE-0001', 0, 1, '2026-12-01', ?, ?, 'Eva', 'Muster', 'eva@example.org', '2027-01-04', '2027-01-08', 4500)
  RETURNING id`, organization, run, period, registration).Scan(ctx, &document))

	require.NoError(t, holidayGuardianAddressUp(ctx, db))

	var preserved bool
	require.NoError(t, db.NewRaw(`SELECT
   (SELECT bool_and(address_requirement = 'off') FROM enrollment.holiday_period_forms WHERE holiday_period_id = ?)
   AND EXISTS (SELECT 1 FROM enrollment.holiday_registrations WHERE id = ? AND guardian_email = 'eva@example.org'
    AND guardian_address_street IS NULL AND guardian_address_postal_code IS NULL AND guardian_address_city IS NULL)
   AND EXISTS (SELECT 1 FROM parent_billing.documents WHERE id = ? AND number = 'FE-0001' AND total_cents = 4500
    AND recipient_address_street IS NULL AND recipient_address_postal_code IS NULL AND recipient_address_city IS NULL)`,
		period, registration, document).Scan(ctx, &preserved))
	assert.True(t, preserved, "existing versions ask nothing, existing registrations and invoices have no address")

	_, err := db.ExecContext(ctx, `UPDATE enrollment.holiday_period_forms SET address_requirement = 'sometimes' WHERE holiday_period_id = ?`, period)
	assert.ErrorContains(t, err, "chk_holiday_period_forms_address_requirement")
	for _, address := range [][3]any{
		{"Musterweg 1", nil, nil}, {nil, "49143", "Bissendorf"}, {"Musterweg 1", "4914", "Bissendorf"},
		{"Musterweg 1", "4914a", "Bissendorf"}, {" ", "49143", "Bissendorf"}, {"Musterweg 1", "49143", " "},
	} {
		_, err := db.ExecContext(ctx, `UPDATE enrollment.holiday_registrations SET guardian_address_street = ?,
   guardian_address_postal_code = ?, guardian_address_city = ? WHERE id = ?`, address[0], address[1], address[2], registration)
		assert.ErrorContains(t, err, "chk_holiday_registrations_guardian_address", "%v", address)
	}
	_, err = db.ExecContext(ctx, `UPDATE enrollment.holiday_registrations SET guardian_address_street = 'Musterweg 1',
   guardian_address_postal_code = '01234', guardian_address_city = 'Bissendorf' WHERE id = ?`, registration)
	require.NoError(t, err, "a full address with a leading zero is stored")

	_, err = db.ExecContext(ctx, `INSERT INTO parent_billing.documents (organization_id, run_id, service, number, number_year, number_sequence,
   issued_on, holiday_period_id, holiday_registration_id, recipient_first_name, recipient_last_name, recipient_email,
   service_from, service_until, total_cents, recipient_address_street)
  VALUES (?, ?, 'holiday', 'FE-0002', 0, 2, '2026-12-01', ?, 99, 'Tom', 'Muster', '', '2027-01-04', '2027-01-08', 0, 'Musterweg 1')`,
		organization, run, period)
	assert.ErrorContains(t, err, "chk_documents_recipient_address", "an invoice copies all three parts or none")
	_, err = db.ExecContext(ctx, `UPDATE parent_billing.documents SET recipient_address_street = 'Musterweg 1',
   recipient_address_postal_code = '49143', recipient_address_city = 'Bissendorf' WHERE id = ?`, document)
	assert.ErrorContains(t, err, "never changes", "an issued invoice still refuses every change")
}
