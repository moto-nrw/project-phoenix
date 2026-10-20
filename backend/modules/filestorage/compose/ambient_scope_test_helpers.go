//go:build testhooks

package compose

import (
	"context"

	postgresNative "github.com/moto-nrw/project-phoenix/modules/filestorage/internal/adapters/postgres"
	"github.com/uptrace/bun"
)

func PersistenceDatabaseForTest(db *bun.DB) func(context.Context) (bun.IDB, int64, error) {
	return postgresNative.DatabaseRuntime(db)
}
