package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParentBillingCarriedCorrectionsMigration pins #5663: the dismissals are
// row-level secured and the request role may only read and add them, and the
// way down and up again leaves the same shape.
func TestParentBillingCarriedCorrectionsMigration(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()

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
	require.NoError(t, parentBillingCarriedCorrectionsUp(ctx, db))
}
