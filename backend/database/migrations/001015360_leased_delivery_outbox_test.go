package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLeasedDeliveryOutboxMigrationBackfillsLegacyRecipientSnapshot(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	require.NoError(t, leasedDeliveryOutboxDown(ctx, db))
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	var id int64
	err := db.NewRaw(`
		INSERT INTO platform.email_outbox (tenant_id, kind, payload, status)
		VALUES (?, 'legacy', '{"recipient_email":"legacy@example.test"}'::jsonb, 'sending')
		RETURNING id`, tenantID).Scan(ctx, &id)
	require.NoError(t, err)
	require.NoError(t, leasedDeliveryOutboxUp(ctx, db))

	var recipient, status string
	err = db.NewRaw(`
		SELECT recipient->>'address', status
		FROM platform.email_outbox WHERE id = ?`, id).Scan(ctx, &recipient, &status)
	require.NoError(t, err)
	assert.Equal(t, "legacy@example.test", recipient)
	assert.Equal(t, "pending", status)
}
