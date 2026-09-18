package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// requestChildStorageBackfillName keys this backfill's advisory session lock.
// Unlike the later backfills it keeps its own checkpoint table, so the name is
// a lock key only, not a column value.
const requestChildStorageBackfillName = "request-child-storage"

func assertRequestChildStorageSource(ctx context.Context, db *bun.DB) error {
	var kind string
	if err := db.NewRaw(`SELECT relkind::text FROM pg_class WHERE oid = 'enrollment.request_child_offerings'::regclass`).Scan(ctx, &kind); err != nil {
		return err
	}
	if kind != "r" {
		return fmt.Errorf("request child storage backfill: legacy source is not a base table; targets may be authoritative")
	}
	return nil
}
