package postgres

import (
	"context"

	"github.com/uptrace/bun"
)

func NewFeedHistoryDatabase(database func(context.Context) (bun.IDB, error), tenantID func(context.Context) int64) func(ctx context.Context) (bun.IDB, int64, error) {
	return func(ctx context.Context) (bun.IDB, int64, error) {
		tx, err := database(ctx)
		return tx, tenantID(ctx), err
	}
}
