package postgres

import (
	"context"
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication/internal/domain"
	"github.com/uptrace/bun"
)

type messageQueryStatsKey struct{}
type messageQueryCountedKey struct{}

type MessageQueryStats struct {
	mu    sync.Mutex
	stats domain.OperationStats
}

func (s *MessageQueryStats) add(event *bun.QueryEvent) {
	rows := int64(0)
	if event.Result != nil {
		rows, _ = event.Result.RowsAffected()
	}
	s.mu.Lock()
	s.stats.Queries++
	s.stats.Rows += rows
	s.stats.StatementDuration += time.Since(event.StartTime)
	s.mu.Unlock()
}

func (s *MessageQueryStats) Snapshot() domain.OperationStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stats
}

type messageQueryHook struct{}

func (messageQueryHook) BeforeQuery(ctx context.Context, _ *bun.QueryEvent) context.Context {
	return ctx
}

func (messageQueryHook) AfterQuery(ctx context.Context, event *bun.QueryEvent) {
	stats, ok := ctx.Value(messageQueryStatsKey{}).(*MessageQueryStats)
	if !ok {
		return
	}
	if event.Stash == nil {
		event.Stash = make(map[any]any)
	}
	if _, counted := event.Stash[messageQueryCountedKey{}]; counted {
		return
	}
	event.Stash[messageQueryCountedKey{}] = struct{}{}
	stats.add(event)
}

// InstallMessageQueryInstrumentation registers the query hook used by the
// public message facade. A per-event marker prevents duplicate registrations
// from counting the same statement more than once.
func InstallMessageQueryInstrumentation(db *bun.DB) {
	db.AddQueryHook(messageQueryHook{})
}

func WithMessageQueryStats(ctx context.Context) (context.Context, *MessageQueryStats) {
	stats := &MessageQueryStats{}
	return context.WithValue(ctx, messageQueryStatsKey{}, stats), stats
}
