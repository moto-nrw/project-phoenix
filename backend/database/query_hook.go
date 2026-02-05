package database

import (
	"context"
	"log/slog"
	"time"

	"github.com/uptrace/bun"
)

const slowQueryThreshold = 5 * time.Millisecond

// QueryHook is a bun.QueryHook that logs queries through slog.
type QueryHook struct {
	logger *slog.Logger
}

// NewQueryHook creates a QueryHook that logs SQL queries via the given logger.
func NewQueryHook(logger *slog.Logger) *QueryHook {
	return &QueryHook{logger: logger}
}

func (h *QueryHook) BeforeQuery(ctx context.Context, _ *bun.QueryEvent) context.Context {
	return ctx
}

func (h *QueryHook) AfterQuery(_ context.Context, event *bun.QueryEvent) {
	dur := time.Since(event.StartTime)
	query := event.Query
	if len(query) > 200 {
		query = query[:200] + "..."
	}

	attrs := []slog.Attr{
		slog.String("operation", event.Operation()),
		slog.Duration("duration", dur),
		slog.String("query", query),
	}

	if event.Err != nil {
		attrs = append(attrs, slog.String("error", event.Err.Error()))
		h.logger.LogAttrs(context.Background(), slog.LevelError, "query error", attrs...)
		return
	}

	if dur >= slowQueryThreshold {
		attrs = append(attrs, slog.Bool("slow_query", true))
		h.logger.LogAttrs(context.Background(), slog.LevelWarn, "slow query", attrs...)
		return
	}

	h.logger.LogAttrs(context.Background(), slog.LevelDebug, "query", attrs...)
}
