package compose_test

import (
	"context"
	"testing"

	fileCompose "github.com/moto-nrw/project-phoenix/modules/filestorage/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/uptrace/bun"
)

func TestPersistenceRequiresAmbientTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	resolve := fileCompose.PersistenceDatabaseForTest(db)
	testpkg.AssertAmbientDatabase(t, db, func(ctx context.Context) (bun.IDB, error) { tx, _, err := resolve(ctx); return tx, err })
}
