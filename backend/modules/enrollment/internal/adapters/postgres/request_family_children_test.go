package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/internal/application"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// SetRequestFamilyChildren names the school in its own filter (#6335): run
// below row level security, a request of another school stays untouched.
// Removing the tenant filter turns this test red.
func TestSetRequestFamilyChildrenKeepsTheSchoolBelowRLS(t *testing.T) {
	t.Parallel()
	db, owner, own, ownPhase := setupRequestRepoTest(t)
	_, _, other, otherPhase := setupRequestRepoTest(t)
	ownRequest := makeOwnerRequest(ownPhase, uniqueToken("family-own"), "eva@example.org")
	require.NoError(t, runInTenantTx(t, db, own, func(ctx context.Context) error { return owner.InsertRequest(ctx, ownRequest) }))
	otherRequest := makeOwnerRequest(otherPhase, uniqueToken("family-other"), "olga@example.org")
	require.NoError(t, runInTenantTx(t, db, other, func(ctx context.Context) error { return owner.InsertRequest(ctx, otherRequest) }))
	store := postgres.NewStore(postgres.NewDatabase(), application.NewTenantID())

	stored := func(id int64) *int {
		var children *int
		require.NoError(t, db.NewRaw(`SELECT family_children FROM enrollment.requests WHERE id = ?`, id).Scan(context.Background(), &children))
		return children
	}
	err := runInTenantTx(t, db, own, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, ok := testpkg.TransactionFromContext(ctx)
		require.True(t, ok)
		var bypasses bool
		require.NoError(t, tx.(bun.Tx).NewRaw(`SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user`).Scan(ctx, &bypasses))
		require.True(t, bypasses, "the test runs below row level security")
		require.ErrorIs(t, store.SetRequestFamilyChildren(ctx, otherRequest.ID, 4), capability.ErrRequestNotFound)
		return store.SetRequestFamilyChildren(ctx, ownRequest.ID, 3)
	})
	require.NoError(t, err)
	assert.Nil(t, stored(otherRequest.ID), "the other school's request keeps no number")
	require.NotNil(t, stored(ownRequest.ID))
	assert.Equal(t, 3, *stored(ownRequest.ID))
}
