package migrations

import (
	"context"
	"database/sql/driver"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

// lockStorageBackfill takes the session-wide advisory lock of one storage
// backfill of the architecture migration (#2580), keyed by the backfill's own
// name so the backfills never exclude each other.
//
// A run carries pass state across transactions. Keep other runs of the same
// backfill, its reset and its rollback out until that state and its sequences
// are finalized. The returned connection owns only the session lock; worker and
// sampler connections remain independent.
func lockStorageBackfill(ctx context.Context, db *bun.DB, name string) (func(), error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	var locked bool
	if err := conn.NewRaw(`SELECT pg_try_advisory_lock(hashtextextended(?, 0))`, name).Scan(ctx, &locked); err != nil || !locked {
		if err != nil {
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
		_ = conn.Close()
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%s backfill: another run, reset or rollback is active", name)
	}
	return func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var unlocked bool
		err := conn.NewRaw(`SELECT pg_advisory_unlock(hashtextextended(?, 0))`, name).Scan(cleanupCtx, &unlocked)
		if err != nil || !unlocked {
			// Never return a session holding our advisory lock to the pool.
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
		_ = conn.Close()
	}, nil
}
