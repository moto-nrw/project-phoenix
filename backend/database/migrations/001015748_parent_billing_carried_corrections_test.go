package migrations

import (
	"context"
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParentBillingCarriedCorrectionsMigration pins #5663: the dismissals are
// row-level secured and the request role may only read and add them. Its down
// migration removes only schema it introduced and keeps released history.
func TestParentBillingCarriedCorrectionsMigration(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Träger übernommene Korrektur", Slug: fmt.Sprintf("traeger-uebernommene-korrektur-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	testpkg.OwnTraegerHolidayPeriods(t, db, organization)
	var period, registration, child, run, correction int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_periods
		(organization_id, name, registration_opens_on, registration_closes_on, status, published_at)
		VALUES (?, 'Sommerferien', '2026-01-01', '2099-12-31', 'published', NOW()) RETURNING id`, organization).Scan(ctx, &period))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registrations
		(organization_id, holiday_period_id, guardian_first_name, guardian_last_name, guardian_email, guardian_phone, privacy_consent_at, status)
		VALUES (?, ?, 'Eva', 'Muster', 'eva@example.org', '0123', NOW(), 'confirmed') RETURNING id`, organization, period).Scan(ctx, &registration))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registration_children
		(organization_id, holiday_period_id, holiday_registration_id, position, first_name, last_name, birth_date)
		VALUES (?, ?, ?, 1, 'Lia', 'Muster', '2018-03-01') RETURNING id`, organization, period, registration).Scan(ctx, &child))
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.runs (organization_id, service, holiday_period_id, period_name, service_from,
		service_until, status, released_at, issued_on, document_count, total_cents)
		VALUES (?, 'holiday', ?, 'Sommerferien', '2027-07-05', '2027-07-09', 'released', NOW(), '2027-06-01', 1, 4500) RETURNING id`, organization, period).Scan(ctx, &run))
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.run_corrections
		(organization_id, run_id, holiday_period_id, holiday_registration_id, holiday_registration_child_id,
		 amount_cents, total_before_cents, total_after_cents, reason, carried_from_number)
		VALUES (?, ?, ?, ?, ?, -500, 5000, 4500, 'Erster Tag krank', 'FE-0001') RETURNING id`,
		organization, run, period, registration, child).Scan(ctx, &correction))

	var secured bool
	require.NoError(t, db.NewRaw(`SELECT relforcerowsecurity FROM pg_class WHERE oid = 'parent_billing.run_correction_dismissals'::regclass`).Scan(ctx, &secured))
	assert.True(t, secured, "the dismissals are row-level secured for every role")
	for privilege, want := range map[string]bool{"SELECT": true, "INSERT": true, "UPDATE": false, "DELETE": false} {
		var has bool
		require.NoError(t, db.NewRaw(`SELECT has_table_privilege('phoenix_tenant', 'parent_billing.run_correction_dismissals', ?)`, privilege).Scan(ctx, &has))
		assert.Equal(t, want, has, privilege)
	}

	require.NoError(t, parentBillingCarriedCorrectionsDown(ctx, db))
	var count int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM information_schema.columns
		WHERE table_schema = 'parent_billing' AND table_name = 'run_corrections' AND column_name = 'carried_from_number'`).Scan(ctx, &count))
	assert.Zero(t, count)
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM information_schema.tables
		WHERE table_schema = 'parent_billing' AND table_name = 'run_correction_dismissals'`).Scan(ctx, &count))
	assert.Zero(t, count)
	var kept bool
	require.NoError(t, db.NewRaw(`SELECT EXISTS (
		SELECT 1 FROM parent_billing.run_corrections
		WHERE id = ? AND amount_cents = -500 AND reason = 'Erster Tag krank'
	)`, correction).Scan(ctx, &kept))
	assert.True(t, kept, "rollback keeps released correction history")
	require.NoError(t, parentBillingCarriedCorrectionsUp(ctx, db))
}
