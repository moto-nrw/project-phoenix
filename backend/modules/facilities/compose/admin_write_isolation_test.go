package compose

import (
	"context"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Check cross-tenant behavior without RLS, and the emitted write statements
// so a successful facade pre-read cannot mask a missing write predicate.
func TestRoomCommandsScopeWritesInAdminTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, func(Observation) {})
	own := testpkg.CreateTestRoom(t, db, "Own room")
	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)
	foreign := testpkg.CreateTestRoomForTenant(t, db, otherTenant, "Foreign room")

	counter := testpkg.CaptureQueriesForContext(t, db)
	require.NoError(t, testpkg.WithAdminTx(t, counter.Context(context.Background()), db, func(ctx context.Context, _ bun.Tx) error {
		ctx = tenant.WithTenantID(ctx, testpkg.Tenant(t))
		_, err := module.UpdateRoom(ctx, facilities.UpdateRoom{ID: foreign.ID, Name: "Changed"})
		require.ErrorIs(t, err, facilities.ErrRoomNotFound)
		require.ErrorIs(t, module.DeleteRoom(ctx, foreign.ID), facilities.ErrRoomNotFound)
		updated, err := module.UpdateRoom(ctx, facilities.UpdateRoom{ID: own.ID, Name: "Updated own room"})
		require.NoError(t, err)
		assert.Equal(t, "Updated own room", updated.Name)
		return module.DeleteRoom(ctx, own.ID)
	}))
	var unchangedName string
	require.NoError(t, db.NewSelect().TableExpr("facilities.rooms").Column("name").Where("id = ?", foreign.ID).Scan(context.Background(), &unchangedName))
	assert.Equal(t, foreign.Name, unchangedName)
	for _, operation := range []string{"UPDATE", "DELETE"} {
		queries := counter.Operation(operation)
		require.Len(t, queries, 1)
		assert.Contains(t, queries[0], fmt.Sprintf(`AND ("room".tenant_id = %d)`, testpkg.Tenant(t)))
	}
}
