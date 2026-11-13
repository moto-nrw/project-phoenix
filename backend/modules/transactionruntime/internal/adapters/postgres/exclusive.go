package postgres

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/uptrace/bun"
)

// WithinExclusive pins only a connection, never a transaction. A session lock
// covers the workflow's successive commits and is released on every exit.
// An uncertain acquire/unlock discards the connection instead of returning a
// possibly locked session to the pool.
func (r *PostgresUnitOfWork) WithinExclusive(ctx context.Context, key string, fn func(context.Context) error) (result error) {
	// Reserve pool capacity for the callback's transactions even when many
	// independent workflows wait for their session mutexes concurrently.
	if r.db.Stats().MaxOpenConnections == 1 {
		return errors.New("exclusive workflow requires at least two pool connections")
	}
	gate := exclusivePoolGate(r.db.DB)
	select {
	case gate <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-gate }()
	conn, err := r.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("exclusive workflow: acquire connection: %w", err)
	}
	defer func() {
		if err := conn.Close(); err != nil && !errors.Is(err, sql.ErrConnDone) {
			result = errors.Join(result, err)
		}
	}()
	discard := func() { _ = conn.Raw(func(any) error { return driver.ErrBadConn }) }
	if err := acquireExclusiveLock(ctx, conn, key); err != nil {
		discard()
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		var unlocked bool
		if err := conn.NewRaw("SELECT pg_advisory_unlock(hashtextextended(?, 0))", key).Scan(cleanup, &unlocked); err != nil || !unlocked {
			discard()
			if err == nil {
				err = errors.New("session did not hold the lock")
			}
			result = errors.Join(result, fmt.Errorf("exclusive workflow: unlock: %w", err))
		}
	}()
	return fn(ctx)
}

func acquireExclusiveLock(ctx context.Context, conn bun.Conn, key string) error {
	for {
		var acquired bool
		if err := conn.NewRaw("SELECT pg_try_advisory_lock(hashtextextended(?, 0))", key).Scan(ctx, &acquired); err != nil {
			return fmt.Errorf("exclusive workflow: acquire lock: %w", err)
		}
		if acquired {
			break
		}
		// The driver cannot interrupt a blocking advisory-lock query on
		// context cancellation. Keep waits outside the database instead.
		timer := time.NewTimer(25 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return nil
}

// A runtime may be composed more than once over the same process-owned pool.
// Share admission across those instances; half its connections remain available
// to callbacks and to ordinary request transactions.
var exclusivePoolGates sync.Map

func exclusivePoolGate(db *sql.DB) chan struct{} {
	if existing, ok := exclusivePoolGates.Load(db); ok {
		return existing.(chan struct{})
	}
	size := db.Stats().MaxOpenConnections / 2
	if size < 1 {
		size = 1
	}
	gate := make(chan struct{}, size)
	actual, _ := exclusivePoolGates.LoadOrStore(db, gate)
	return actual.(chan struct{})
}
