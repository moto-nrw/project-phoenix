package migrations

import (
	"context"
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParentBillingCostBearersMigrationKeepsInvoicesAndErasesWithTheTraeger
// pins #5627: existing positions stay and name no cost bearer; a position
// of a cost takeover needs the cost bearer's ID and name, any other none; the
// trail takes proof_upload; and the erasure cascade of a Träger removes its
// cost bearers together with the invoices that name them.
func TestParentBillingCostBearersMigrationKeepsInvoicesAndErasesWithTheTraeger(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Ferien Kostenträger", Slug: fmt.Sprintf("ferien-kostentraeger-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	testpkg.OwnTraegerHolidayPeriods(t, db, organization)
	testpkg.OwnTraegerContributionRules(t, db, organization)
	require.NoError(t, parentBillingCostBearersDown(ctx, db))

	var period, registration, run, document int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_periods
  (organization_id, name, registration_opens_on, registration_closes_on, status, published_at)
  VALUES (?, 'Sommerferien', '2026-01-01', '2099-12-31', 'published', now()) RETURNING id`, organization).Scan(ctx, &period))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registrations
  (organization_id, holiday_period_id, guardian_first_name, guardian_last_name, guardian_email, guardian_phone, privacy_consent_at, status)
  VALUES (?, ?, 'Eva', 'Muster', 'eva@example.org', '0151 2345678', now(), 'confirmed') RETURNING id`, organization, period).Scan(ctx, &registration))
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.runs (organization_id, service, holiday_period_id, period_name, service_from,
   service_until, status, released_at, issued_on, document_count, total_cents)
  VALUES (?, 'holiday', ?, 'Sommerferien', '2027-07-05', '2027-07-09', 'released', now(), '2027-06-01', 1, 4500) RETURNING id`,
		organization, period).Scan(ctx, &run))
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.documents (organization_id, run_id, service, number, number_year, number_sequence,
   issued_on, holiday_period_id, holiday_registration_id, recipient_first_name, recipient_last_name, recipient_email,
   service_from, service_until, total_cents)
  VALUES (?, ?, 'holiday', 'FE-0001', 0, 1, '2027-06-01', ?, ?, 'Eva', 'Muster', 'eva@example.org', '2027-07-05', '2027-07-09', 4500)
  RETURNING id`, organization, run, period, registration).Scan(ctx, &document))
	_, err := db.ExecContext(ctx, `INSERT INTO parent_billing.document_positions (organization_id, document_id, position,
   holiday_registration_child_id, child_first_name, child_last_name, kind, amount_cents, net_cents, time_model_name, week_starts_on, week_ends_on)
  VALUES (?, ?, 1, 1, 'Lia', 'Muster', 'base', 4500, 4500, 'bis 13 Uhr', '2027-07-05', '2027-07-09')`, organization, document)
	require.NoError(t, err)

	require.NoError(t, parentBillingCostBearersUp(ctx, db))

	var kept bool
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM parent_billing.document_positions
   WHERE document_id = ? AND kind = 'base' AND cost_bearer_id IS NULL AND cost_bearer_name = '')`, document).Scan(ctx, &kept))
	assert.True(t, kept, "an existing position stays and names no cost bearer")

	var bearer int64
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.cost_bearers (organization_id, name) VALUES (?, 'Jugendamt Musterstadt') RETURNING id`,
		organization).Scan(ctx, &bearer))
	_, err = db.ExecContext(ctx, `INSERT INTO parent_billing.cost_bearers (organization_id, name) VALUES (?, ' ')`, organization)
	assert.Error(t, err, "a cost bearer needs a name")
	insertPosition := func(position int, kind string, id any, name string) error {
		_, err := db.ExecContext(ctx, `INSERT INTO parent_billing.document_positions (organization_id, document_id, position,
   holiday_registration_child_id, child_first_name, child_last_name, kind, amount_cents, net_cents, cost_bearer_id, cost_bearer_name)
  VALUES (?, ?, ?, 1, 'Lia', 'Muster', ?, -4500, -4500, ?, ?)`, organization, document, position, kind, id, name)
		return err
	}
	assert.Error(t, insertPosition(2, "cost_takeover", nil, "Jugendamt"), "a takeover names its cost bearer")
	assert.Error(t, insertPosition(2, "sibling", bearer, "Jugendamt"), "another position names none")
	require.NoError(t, insertPosition(2, "cost_takeover", bearer, "Jugendamt Musterstadt"))
	_, err = db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_audit
  (organization_id, holiday_period_id, holiday_registration_id, action, details)
  VALUES (?, ?, ?, 'proof_upload', '{"proof_id":1}')`, organization, period, registration)
	require.NoError(t, err, "the trail takes the upload of a proof by the Träger-Büro")

	_, err = db.ExecContext(ctx, `DELETE FROM platform.organizations WHERE id = ?`, organization)
	require.NoError(t, err, "the erasure cascade removes the cost bearers with the invoices that name them")
	var left int
	require.NoError(t, db.NewRaw(`SELECT (SELECT count(*) FROM parent_billing.cost_bearers WHERE organization_id = ?)
   + (SELECT count(*) FROM parent_billing.document_positions WHERE organization_id = ?)`, organization, organization).Scan(ctx, &left))
	assert.Zero(t, left)
}
