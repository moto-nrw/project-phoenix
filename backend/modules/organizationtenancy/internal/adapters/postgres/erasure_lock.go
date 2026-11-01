package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

type erasureLockKey struct{}

type erasureLock struct {
	conn          bun.Conn
	lastConfirmed time.Time
	lost          error
}

// CheckSchoolLock is a step boundary, not a heartbeat: it cannot fence work
// already in progress when the server terminates the coordination session.
// Calls outside a school workflow (receipt reads and preflight) have no lock.
func (s *ErasureStore) CheckSchoolLock(ctx context.Context) error {
	lock, ok := ctx.Value(erasureLockKey{}).(*erasureLock)
	if !ok {
		return nil
	}
	return lock.check(ctx)
}

func (l *erasureLock) check(ctx context.Context) error {
	if l.lost != nil {
		return l.lost
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := l.conn.PingContext(ctx); err != nil {
		l.lost = fmt.Errorf("erasure: coordination session unavailable; lock protection last confirmed at %s; exact loss time unknown; stopping before further work: %w", l.lastConfirmed.UTC().Format(time.RFC3339Nano), err)
		return l.lost
	}
	l.lastConfirmed = time.Now()
	return nil
}

func (l *erasureLock) release(ctx context.Context, id int64) error {
	if err := l.check(ctx); err != nil {
		return err
	}
	var unlocked bool
	if err := l.conn.NewRaw(`SELECT pg_advisory_unlock(hashtextextended('school-erasure-cli:' || ?::text,0))`, id).Scan(ctx, &unlocked); err != nil {
		return fmt.Errorf("erasure: coordination release failed: %w", err)
	}
	if !unlocked {
		return errors.New("erasure: coordination lock was lost before release")
	}
	return nil
}
