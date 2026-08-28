package migrations

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestParentLetterDeliveryRLS(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	tenantA := testpkg.UniqueTestTenantID(t)
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantA)
	testpkg.EnsureTestTenant(t, db, tenantB)
	t.Cleanup(func() {
		ctx := context.Background()
		_, err := db.NewDelete().Table("platform.email_delivery").
			Where("tenant_id IN (?, ?)", tenantA, tenantB).Exec(ctx)
		require.NoError(t, err)
		testpkg.CleanupTenantTestData(t, db, tenantA)
		testpkg.CleanupTenantTestData(t, db, tenantB)
	})

	ctx := context.Background()
	for _, tenantID := range []int64{tenantA, tenantB} {
		_, err := db.NewRaw(`
			INSERT INTO platform.email_delivery
				(tenant_id, related_entity_type, related_entity_id, recipient_email)
			VALUES (?, 'rls-test', ?, 'guardian@example.test')
		`, tenantID, tenantID).Exec(ctx)
		require.NoError(t, err)
	}

	var enabled, forced bool
	require.NoError(t, db.NewRaw(`
		SELECT relrowsecurity, relforcerowsecurity FROM pg_class
		WHERE oid = 'platform.email_delivery'::regclass
	`).Scan(ctx, &enabled, &forced))
	assert.True(t, enabled, "platform.email_delivery must enable RLS")
	assert.True(t, forced, "platform.email_delivery must force RLS")

	err := tenant.WithTenantTx(ctx, db, tenantA, func(ctx context.Context, tx bun.Tx) error {
		var visible int
		if err := tx.NewRaw(`SELECT count(*) FROM platform.email_delivery`).Scan(ctx, &visible); err != nil {
			return err
		}
		assert.Equal(t, 1, visible, "tenant queries without a WHERE clause must not see other schools")
		return nil
	})
	require.NoError(t, err)
}
