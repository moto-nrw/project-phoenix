package migrations

import (
	"context"
	"database/sql/driver"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

// The run carries pass state across transactions. Keep other runs, reset and
// down out until that state and the sequences are finalized. This connection
// owns only the session lock; worker and sampler connections remain independent.
func lockStudentOwnerBackfill(ctx context.Context, db *bun.DB) (func(), error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	var locked bool
	if err := conn.NewRaw(`SELECT pg_try_advisory_lock(hashtextextended(?, 0))`, StudentOwnerBackfillName).Scan(ctx, &locked); err != nil || !locked {
		if err != nil {
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
		_ = conn.Close()
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("student owner backfill: another run, reset or rollback is active")
	}
	return func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var unlocked bool
		err := conn.NewRaw(`SELECT pg_advisory_unlock(hashtextextended(?, 0))`, StudentOwnerBackfillName).Scan(cleanupCtx, &unlocked)
		if err != nil || !unlocked {
			// Never return a session holding our advisory lock to the pool.
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
		_ = conn.Close()
	}, nil
}
