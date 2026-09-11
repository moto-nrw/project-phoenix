package database

import (
	"context"
	"database/sql"
	"errors"
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

func (h *QueryHook) AfterQuery(ctx context.Context, event *bun.QueryEvent) {
	h.afterQuery(ctx, event.Operation(), event.Query, event.StartTime, event.Err)
}

func (h *QueryHook) afterQuery(ctx context.Context, operation, query string, started time.Time, err error) {
	data := queryLogData{operation: operation, query: query, started: started, err: err}
	if err != nil {
		data.noRows = errors.Is(err, sql.ErrNoRows)
		var pgErr interface {
			error
			Field(byte) string
		}
		if errors.As(err, &pgErr) {
			data.sqlState = pgErr.Field('C')
			data.constraint = pgErr.Field('n')
		}
	}
	h.logQuery(ctx, data)
}

type queryLogData struct {
	operation  string
	query      string
	started    time.Time
	err        error
	noRows     bool
	sqlState   string
	constraint string
}

func (h *QueryHook) logQuery(ctx context.Context, event queryLogData) {
	dur := time.Since(event.started)
	query := event.query
	if len(query) > 200 {
		query = query[:200] + "..."
	}

	attrs := []slog.Attr{
		slog.String("operation", event.operation),
		slog.Duration("duration", dur),
		slog.String("query", query),
	}

	if event.err != nil {
		if event.noRows {
			h.logger.LogAttrs(ctx, slog.LevelDebug, "query no rows", attrs...)
			return
		}
		attrs = append(attrs, slog.String("error", event.err.Error()))
		if event.sqlState != "" {
			attrs = append(attrs, slog.String("sqlstate", event.sqlState))
		}
		if event.constraint != "" {
			attrs = append(attrs, slog.String("constraint", event.constraint))
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
