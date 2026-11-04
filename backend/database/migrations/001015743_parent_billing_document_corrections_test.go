package migrations

import (
	"context"
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParentBillingDocumentCorrectionsMigrationKeepsInvoicesAndGuardsTheKinds
// pins #4690: an existing invoice stays an invoice without reference or
// reason; a Storno and a Gutschrift need a reference, a reason and their
// minus; an invoice is cancelled and replaced at most once, and a
// registration keeps one first invoice; no document ever changes.
func TestParentBillingDocumentCorrectionsMigrationKeepsInvoicesAndGuardsTheKinds(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Ferien Storno", Slug: fmt.Sprintf("ferien-storno-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	testpkg.OwnTraegerHolidayPeriods(t, db, organization)
	testpkg.OwnTraegerContributionRules(t, db, organization)
	require.NoError(t, parentBillingDocumentCorrectionsDown(ctx, db))

	var period, run, invoice int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_periods
  (organization_id, name, registration_opens_on, registration_closes_on, status, published_at)
  VALUES (?, 'Winterferien', '2026-01-01', '2099-12-31', 'published', now()) RETURNING id`, organization).Scan(ctx, &period))
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.runs (organization_id, service, holiday_period_id, period_name, service_from,
   service_until, status, released_at, issued_on, document_count, total_cents)
  VALUES (?, 'holiday', ?, 'Winterferien', '2027-01-04', '2027-01-08', 'released', now(), '2026-12-01', 1, 4500) RETURNING id`,
		organization, period).Scan(ctx, &run))
	insert := func(number string, sequence int, kind string, reference *int64, reason string, total int64) (int64, error) {
		var id int64
		err := db.NewRaw(`INSERT INTO parent_billing.documents (organization_id, run_id, service, number, number_year, number_sequence,
   issued_on, holiday_period_id, holiday_registration_id, recipient_first_name, recipient_last_name, recipient_email,
   service_from, service_until, total_cents, kind, reference_document_id, reason)
  VALUES (?, ?, 'holiday', ?, 0, ?, '2026-12-01', ?, 77, 'Eva', 'Muster', 'eva@example.org', '2027-01-04', '2027-01-08', ?, ?, ?, ?)
  RETURNING id`, organization, run, number, sequence, period, total, kind, reference, reason).Scan(ctx, &id)
		return id, err
	}
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.documents (organization_id, run_id, service, number, number_year, number_sequence,
   issued_on, holiday_period_id, holiday_registration_id, recipient_first_name, recipient_last_name, recipient_email,
   service_from, service_until, total_cents)
  VALUES (?, ?, 'holiday', 'FE-0001', 0, 1, '2026-12-01', ?, 77, 'Eva', 'Muster', 'eva@example.org', '2027-01-04', '2027-01-08', 4500)
  RETURNING id`, organization, run, period).Scan(ctx, &invoice))

	require.NoError(t, parentBillingDocumentCorrectionsUp(ctx, db))

	var kept bool
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM parent_billing.documents
   WHERE id = ? AND kind = 'invoice' AND reference_document_id IS NULL AND reason = '' AND total_cents = 4500)`, invoice).Scan(ctx, &kept))
	assert.True(t, kept, "an existing invoice stays an invoice without reference or reason")

	_, err := insert("FE-0002", 2, "invoice", nil, "", 4500)
	assert.Error(t, err, "a registration has one first invoice")
	_, err = insert("FE-0002", 2, "cancellation", nil, "Abgemeldet", -4500)
	assert.Error(t, err, "a Storno names its invoice")
	_, err = insert("FE-0002", 2, "cancellation", &invoice, " ", -4500)
	assert.Error(t, err, "a Storno has a reason")
	_, err = insert("FE-0002", 2, "credit", &invoice, "Krank", 500)
	assert.Error(t, err, "a Gutschrift has its minus")
	cancellation, err := insert("FE-0002", 2, "cancellation", &invoice, "Abgemeldet", -4500)
	require.NoError(t, err)
	_, err = insert("FE-0003", 3, "cancellation", &invoice, "Doppelt", -4500)
	assert.Error(t, err, "an invoice is cancelled once")
	_, err = insert("FE-0003", 3, "invoice", &invoice, "", 4000)
	require.NoError(t, err, "the cancelled invoice is issued again")
	_, err = insert("FE-0004", 4, "invoice", &invoice, "", 4000)
	assert.Error(t, err, "and replaced once")
	_, err = insert("FE-0004", 4, "credit", &invoice, "Krank", -500)
	require.NoError(t, err, "a Gutschrift with a reason and its minus")

	_, err = db.ExecContext(ctx, `UPDATE parent_billing.documents SET reason = 'x' WHERE id = ?`, cancellation)
	assert.ErrorContains(t, err, "never changes", "a Storno never changes")
	_, err = db.ExecContext(ctx, `INSERT INTO parent_billing.run_events (organization_id, run_id, action) VALUES (?, ?, 'cancel'), (?, ?, 'credit')`,
		organization, run, organization, run)
	require.NoError(t, err, "the trail takes a Storno and a Gutschrift")
}
