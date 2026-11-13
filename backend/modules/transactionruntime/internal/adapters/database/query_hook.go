package database

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/transactionruntime/protocol"
	"github.com/uptrace/bun"
)

const slowQueryThreshold = 5 * time.Millisecond

var (
	// byteaLiteral is a bytea value bun inlines into the logged SQL: sealed
	// columns (the IBAN and account holder of a SEPA mandate, health data)
	// and hashes. The log never repeats them (#4695).
	byteaLiteral = regexp.MustCompile(`'\\x[0-9a-fA-F]*'`)
	// ibanLike is a value shaped like an IBAN that reached the SQL in the
	// clear; the log names it only (#4695).
	ibanLike = regexp.MustCompile(`\b[A-Z]{2}[0-9]{2}[A-Z0-9]{11,30}\b`)
	// stringLiteral is any SQL string literal, a doubled quote inside it
	// included. Only statements marked with protocol.RedactLiteralsInLog
	// lose them: there they carry what a person typed (ADR 0230).
	stringLiteral = regexp.MustCompile(`'(?:[^']|'')*'`)
)

// redactQuery removes bank data and sealed values from a query before it is
// logged, and every string literal of a statement marked as carrying typed
// text.
func redactQuery(query string) string {
	if strings.Contains(query, strings.TrimSpace(protocol.RedactLiteralsInLog)) {
		query = stringLiteral.ReplaceAllString(query, "'[redacted]'")
	}
	query = byteaLiteral.ReplaceAllString(query, "'[redacted]'")
	return ibanLike.ReplaceAllString(query, "[redacted]")
}

// QueryHook is a bun.QueryHook that logs queries through slog.
type QueryHook struct {
	logger *slog.Logger
}

// NewQueryHook creates a QueryHook that logs SQL queries via the given logger.
func NewQueryHook(logger *slog.Logger) *QueryHook {
	return &QueryHook{logger: logger}
}

// AttachQueryHook adds hook to db itself, so it reaches every component that
// already holds the pool.
func AttachQueryHook(db *bun.DB, hook bun.QueryHook) {
	db.AddQueryHook(hook) //nolint:staticcheck // SA1019: hooks must reach the shared *bun.DB other components already hold; WithQueryHook returns a clone.
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
	query := redactQuery(event.query)
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
