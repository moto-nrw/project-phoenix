package database

import (
	"context"
	"strings"
	"time"

	"github.com/uptrace/bun"
)

// LockWaitQueryHook records the elapsed time of SQL statements that can wait
// for PostgreSQL row, table, or advisory locks. PostgreSQL does not expose the
// wait-only portion per statement, so this is intentionally the acquisition
// query duration and therefore an upper bound on lock wait.
type LockWaitQueryHook struct {
	observe func(context.Context, time.Duration)
}

func NewLockWaitQueryHook(observe func(context.Context, time.Duration)) *LockWaitQueryHook {
	return &LockWaitQueryHook{observe: observe}
}

func (h *LockWaitQueryHook) BeforeQuery(ctx context.Context, _ *bun.QueryEvent) context.Context {
	return ctx
}

func (h *LockWaitQueryHook) AfterQuery(ctx context.Context, event *bun.QueryEvent) {
	if h == nil || h.observe == nil || !canWaitForLock(event.Query) {
		return
	}
	h.observe(ctx, time.Since(event.StartTime))
}

func canWaitForLock(query string) bool {
	query = strings.ToLower(query)
	return strings.Contains(query, "pg_advisory_xact_lock") ||
		strings.Contains(query, " for update") ||
		strings.Contains(query, " for no key update") ||
		strings.Contains(query, " for share") ||
		strings.Contains(query, " for key share") ||
		strings.HasPrefix(strings.TrimSpace(query), "lock table ")
}
