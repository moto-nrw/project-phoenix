package postgres

import (
	"context"

	"github.com/uptrace/bun"
)

func PlanningTrackAdministrationOrderPlanningTracksBinding(db *bun.DB, execute0 func(ctx context.Context, _ *bun.DB, rawID int64, fn func(context.Context, bun.Tx) error) error) func(context.Context, int64, func(context.Context) error) error {
	return func(ctx context.Context, tenantID int64, callback func(context.Context) error) error {
		return execute0(ctx, db, tenantID, func(txCtx context.Context, _ bun.Tx) error { return callback(txCtx) })
	}
}
