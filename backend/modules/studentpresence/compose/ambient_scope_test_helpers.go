//go:build testhooks

package compose

import (
	"context"

	"github.com/uptrace/bun"
)

func PersistenceDatabaseForTest(db *bun.DB) func(context.Context) (bun.IDB, int64, error) {
	return databaseRuntime(db)
}
