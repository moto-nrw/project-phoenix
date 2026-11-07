package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEmailAddressMarksRoundTrip pins 1.15.772 (#5934) in both directions.
func TestEmailAddressMarksRoundTrip(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	columns := func() int {
		var count int
		require.NoError(t, db.NewRaw(`SELECT count(*) FROM information_schema.columns
			WHERE table_schema = 'users' AND table_name = 'guardian_profiles' AND column_name = 'emails_stopped_at'`).Scan(ctx, &count))
		return count
	}
	indexes := func() int {
		var count int
		require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_indexes
			WHERE schemaname = 'platform' AND indexname = 'idx_email_outbox_school_kind_address'`).Scan(ctx, &count))
		return count
	}
	tenantMayWriteTheList := func() bool {
		var allowed bool
		require.NoError(t, db.NewRaw(`SELECT has_table_privilege('phoenix_tenant', 'platform.email_suppressions', 'INSERT')`).Scan(ctx, &allowed))
		return allowed
	}
	assert.Equal(t, 1, columns())
	assert.Equal(t, 1, indexes())
	assert.False(t, tenantMayWriteTheList(), "only reported events fill the list")

	require.NoError(t, emailAddressMarksDown(ctx, db))
	assert.Zero(t, columns())
	assert.Zero(t, indexes())

	require.NoError(t, emailAddressMarksUp(ctx, db))
	assert.Equal(t, 1, columns())
	assert.Equal(t, 1, indexes())
}
