package database

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/uptrace/bun"
)

func newTestQueryHook(t *testing.T) (*QueryHook, *bytes.Buffer) {
	t.Helper()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	return NewQueryHook(logger), &buf
}

func TestQueryHookAfterQuery_DemotesNoRowsToDebug(t *testing.T) {
	hook, buf := newTestQueryHook(t)

	hook.AfterQuery(context.Background(), &bun.QueryEvent{
		Query:     `SELECT * FROM users.profiles WHERE account_id = 1`,
		StartTime: time.Now(),
		Err:       sql.ErrNoRows,
	})

	output := buf.String()
	if !strings.Contains(output, `"level":"DEBUG"`) {
		t.Fatalf("expected debug level log, got %q", output)
	}
	if !strings.Contains(output, `"msg":"query no rows"`) {
		t.Fatalf("expected no-rows message, got %q", output)
	}
	if strings.Contains(output, `"msg":"query error"`) {
		t.Fatalf("expected query error message to be skipped, got %q", output)
	}
}

func TestQueryHookAfterQuery_LogsNonNoRowsErrorsAtError(t *testing.T) {
	hook, buf := newTestQueryHook(t)

	hook.AfterQuery(context.Background(), &bun.QueryEvent{
		Query:     `SELECT * FROM users.profiles WHERE account_id = 1`,
		StartTime: time.Now(),
		Err:       errors.New("boom"),
	})

	output := buf.String()
	if !strings.Contains(output, `"level":"ERROR"`) {
		t.Fatalf("expected error level log, got %q", output)
	}
	if !strings.Contains(output, `"msg":"query error"`) {
		t.Fatalf("expected query error message, got %q", output)
	}
	if !strings.Contains(output, `"error":"boom"`) {
		t.Fatalf("expected original error in log, got %q", output)
	}
}

func TestQueryHookAfterQuery_LogsSlowQueriesAtWarn(t *testing.T) {
	hook, buf := newTestQueryHook(t)

	hook.AfterQuery(context.Background(), &bun.QueryEvent{
		Query:     `SELECT * FROM users.profiles WHERE account_id = 1`,
		StartTime: time.Now().Add(-10 * time.Millisecond),
	})

	output := buf.String()
	if !strings.Contains(output, `"level":"WARN"`) {
		t.Fatalf("expected warn level log, got %q", output)
	}
	if !strings.Contains(output, `"msg":"slow query"`) {
		t.Fatalf("expected slow query message, got %q", output)
	}
	if !strings.Contains(output, `"slow_query":true`) {
		t.Fatalf("expected slow_query marker, got %q", output)
	}
}
