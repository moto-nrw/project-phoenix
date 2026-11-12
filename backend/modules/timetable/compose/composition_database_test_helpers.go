//go:build testhooks

package compose

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/timetable/internal/adapters/postgres"
	"github.com/uptrace/bun"
)

func blockDisplayDatabase(ctx context.Context, db *bun.DB) (bun.IDB, error) {
	return postgres.BlockDisplayDatabase(ctx, db)
}
