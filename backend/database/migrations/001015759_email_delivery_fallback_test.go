package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEmailDeliveryFallbackRoundTrip pins 1.15.759 (#5933) in both directions.
func TestEmailDeliveryFallbackRoundTrip(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	columns := func() int {
		var count int
		require.NoError(t, db.NewRaw(`SELECT count(*) FROM information_schema.columns
			WHERE table_schema = 'platform' AND table_name = 'email_outbox'
				AND column_name IN ('fallback_requested_at', 'delivery_checked_at')`).Scan(ctx, &count))
		return count
	}
	assert.Equal(t, 2, columns())

	require.NoError(t, emailDeliveryFallbackDown(ctx, db))
	assert.Zero(t, columns())

	require.NoError(t, emailDeliveryFallbackUp(ctx, db))
	assert.Equal(t, 2, columns())
}
