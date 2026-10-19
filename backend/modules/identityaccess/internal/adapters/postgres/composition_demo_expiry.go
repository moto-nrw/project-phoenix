package postgres

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

func ExpireDemoAccessesBinding(ctx context.Context, db bun.IDB, now func() time.Time, execute func(context.Context, bun.IDB, func(context.Context, bun.Tx) error) error) (int, []string, error) {
	var count int
	var slugs []string
	err := execute(ctx, db, func(txCtx context.Context, tx bun.Tx) error {
		var err error
		count, slugs, err = NewDemoAccessExpiryStore(tx).DeleteExpiredDemoAccesses(txCtx, now())
		return err
	})
	return count, slugs, err
}
