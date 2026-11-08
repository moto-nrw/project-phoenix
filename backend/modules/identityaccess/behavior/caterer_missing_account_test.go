package behavior_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestCatererMissingAccountPrecedesSupplyRead(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	caterers := setupAuthFactory(t, db).Caterers()
	account := testpkg.CreateTestAccount(t, db, "not-a-caterer")
	blocker, err := db.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, blocker.Rollback()) }()
	_, err = blocker.NewRaw("LOCK TABLE auth.caterer_schools IN ACCESS EXCLUSIVE MODE").Exec(t.Context())
	require.NoError(t, err)
	err = testpkg.WithAdminTx(t, testpkg.Ctx(t), db, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.NewRaw("SET LOCAL lock_timeout = '10ms'").Exec(ctx)
		require.NoError(t, err)
		_, _, err = caterers.CatererSchools(ctx, account.ID)
		require.ErrorIs(t, err, identityaccess.ErrAccountNotCaterer)
		return nil
	})
	require.NoError(t, err)
}
