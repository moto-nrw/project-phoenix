package migrations

import (
	"context"
	"fmt"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// People who already work with moto never see the first steps: the backfill
// hides the checklist for every active account of a school, and only for
// those. An invitation still pending at migration time is a person who has not
// started yet and keeps the checklist, as does an account that left.
func TestStaffOnboardingsBackfillHidesTheChecklistForActiveAccountsOnly(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	require.NoError(t, staffOnboardingsDown(ctx, db))

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	var orgID, schoolID int64
	_, err := db.NewRaw(`INSERT INTO platform.organizations (name, slug, active, created_at, updated_at)
		VALUES (?, ?, true, NOW(), NOW()) RETURNING id`, "Onboarding Org "+suffix, "onboarding-org-"+suffix).Exec(ctx, &orgID)
	require.NoError(t, err)
	slug := "onboarding-" + suffix
	_, err = db.NewRaw(`INSERT INTO platform.schools (name, slug, subdomain, organization_id, active, created_at, updated_at)
		VALUES (?, ?, ?, ?, true, NOW(), NOW()) RETURNING id`, "Onboarding School "+suffix, slug, slug, orgID).Exec(ctx, &schoolID)
	require.NoError(t, err)

	member := func(label, status string) int64 {
		var accountID int64
		_, aerr := db.NewRaw(`INSERT INTO auth.accounts (email, active) VALUES (?, TRUE) RETURNING id`,
			label+"-"+suffix+"@ogs-beispiel.de").Exec(ctx, &accountID)
		require.NoError(t, aerr)
		_, merr := db.NewRaw(`INSERT INTO auth.account_tenants (account_id, tenant_id, status) VALUES (?, ?, ?)`,
			accountID, schoolID, status).Exec(ctx)
		require.NoError(t, merr)
		return accountID
	}
	active := member("active", "active")
	pending := member("pending", "pending")
	inactive := member("inactive", "inactive")

	require.NoError(t, staffOnboardingsUp(ctx, db))

	var dismissed []int64
	require.NoError(t, db.NewRaw(`SELECT account_id FROM config.staff_onboardings
		WHERE tenant_id = ? AND dismissed_at IS NOT NULL ORDER BY account_id`, schoolID).Scan(ctx, &dismissed))
	assert.Equal(t, []int64{active}, dismissed, "only the active account is backfilled")
	assert.NotContains(t, dismissed, pending, "a pending invitation keeps the checklist")
	assert.NotContains(t, dismissed, inactive)

	var forced bool
	require.NoError(t, db.NewRaw(`SELECT relforcerowsecurity FROM pg_class
		WHERE oid = 'config.staff_onboardings'::regclass`).Scan(ctx, &forced))
	assert.True(t, forced, "the table is tenant-isolated by forced RLS")

	// Running up again changes nothing: the backfill skips existing rows.
	require.NoError(t, staffOnboardingsUp(ctx, db))
	var rows int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM config.staff_onboardings WHERE tenant_id = ?`, schoolID).Scan(ctx, &rows))
	assert.Equal(t, 1, rows)
}
