package compose

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/uptrace/bun"
)

func TestBlockDisplayRequiresAmbientTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	testpkg.AssertAmbientDatabase(t, db, func(ctx context.Context) (bun.IDB, error) { return blockDisplayDatabase(ctx, db) })
}
