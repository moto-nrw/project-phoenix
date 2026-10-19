package parentstore_test

import (
	"context"
	"testing"

	store "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/uptrace/bun"
)

func TestPersistenceRequiresAmbientTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	resolve := store.ProjectionDatabase(db)
	testpkg.AssertAmbientDatabase(t, db, func(ctx context.Context) (bun.IDB, error) { tx, _, err := resolve(ctx); return tx, err })
}

func TestMain(m *testing.M) { testpkg.PerTestTenants(); testpkg.Run(m) }
