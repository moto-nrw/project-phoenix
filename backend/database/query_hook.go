package database

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
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

func (h *QueryHook) AfterQuery(ctx context.Context, event *bun.QueryEvent) {
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
		if errors.Is(event.Err, sql.ErrNoRows) {
			h.logger.LogAttrs(ctx, slog.LevelDebug, "query no rows", attrs...)
			return
		}
		attrs = append(attrs, slog.String("error", event.Err.Error()))
		var pgErr pgdriver.Error
		if errors.As(event.Err, &pgErr) {
			if code := pgErr.Field('C'); code != "" {
				attrs = append(attrs, slog.String("sqlstate", code))
			}
			if constraint := pgErr.Field('n'); constraint != "" {
				attrs = append(attrs, slog.String("constraint", constraint))
			}
		}
		h.logger.LogAttrs(ctx, slog.LevelError, "query error", attrs...)
		return
	}

	if dur >= slowQueryThreshold {
		attrs = append(attrs, slog.Bool("slow_query", true))
		h.logger.LogAttrs(ctx, slog.LevelWarn, "slow query", attrs...)
		return
	}

	h.logger.LogAttrs(ctx, slog.LevelDebug, "query", attrs...)
}
