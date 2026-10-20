package migrations

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/uptrace/bun"
)

// monitorBackfillLocks is shared by the Presence and Staff storage copies.
func monitorBackfillLocks(ctx context.Context, db *bun.DB, pid int) (func() (time.Duration, error), time.Duration, error) {
	acquireCtx, acquireCancel := context.WithTimeout(ctx, time.Second)
	started := time.Now()
	conn, err := db.Conn(acquireCtx)
	acquireCancel()
	poolWait := time.Since(started)
	if err != nil {
		return nil, poolWait, fmt.Errorf("backfill needs a separate connection for lock sampling: %w", err)
	}
	sampleCtx, cancel := context.WithCancel(ctx)
	type sampleResult struct {
		wait time.Duration
		err  error
	}
	result := make(chan sampleResult, 1)
	go func() {
		defer func() { _ = conn.Close() }()
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		var observed sampleResult
		last := time.Now()
		for {
			select {
			case <-sampleCtx.Done():
				result <- observed
				return
			case now := <-ticker.C:
				var waiting bool
				err := conn.NewRaw(`SELECT coalesce(wait_event_type = 'Lock', false) FROM pg_stat_activity WHERE pid = ?`, pid).Scan(sampleCtx, &waiting)
				if err != nil {
					if sampleCtx.Err() == nil {
						observed.err = fmt.Errorf("sample backfill lock waits: %w", err)
					}
					result <- observed
					return
				}
				if waiting {
					observed.wait += now.Sub(last)
				}
				last = now
			}
		}
	}()
	var once sync.Once
	var final sampleResult
	stop := func() (time.Duration, error) {
		once.Do(func() { cancel(); final = <-result })
		return final.wait, final.err
	}
	return stop, poolWait, nil
}
