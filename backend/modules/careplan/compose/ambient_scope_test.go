package compose

import (
	"context"
	"testing"

	postgresNative "github.com/moto-nrw/project-phoenix/modules/careplan/internal/adapters/postgres"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/uptrace/bun"
)

func TestPersistenceRequiresAmbientTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	resolve := postgresNative.GuardianPickupDatabase(db)
	testpkg.AssertAmbientDatabase(t, db, func(ctx context.Context) (bun.IDB, error) { tx, _, err := resolve(ctx); return tx, err })
}
