package database

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
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
	t.Parallel()

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
	t.Parallel()

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

func TestQueryHookAfterQuery_LogsPostgresErrorMetadata(t *testing.T) {
	t.Parallel()

	hook, buf := newTestQueryHook(t)

	hook.AfterQuery(context.Background(), &bun.QueryEvent{
		Query:     `INSERT INTO schedule.activity_instances (...) VALUES (...)`,
		StartTime: time.Now(),
		Err:       newPgErrorWithConstraint("23505", "idx_activity_instances_template_unique"),
	})

	output := buf.String()
	if !strings.Contains(output, `"sqlstate":"23505"`) {
		t.Fatalf("expected sqlstate in log, got %q", output)
	}
	if !strings.Contains(output, `"constraint":"idx_activity_instances_template_unique"`) {
		t.Fatalf("expected constraint in log, got %q", output)
	}
}

func TestQueryHookAfterQuery_LogsSlowQueriesAtWarn(t *testing.T) {
	t.Parallel()

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

func newPgErrorWithConstraint(code, constraintName string) error {
	pgErr := pgdriver.Error{}
	v := reflect.ValueOf(&pgErr).Elem()
	mField := v.FieldByName("m")
	ptr := unsafe.Pointer(mField.UnsafeAddr()) //nolint:gosec
	*(*map[byte]string)(ptr) = map[byte]string{'C': code, 'n': constraintName}
	return pgErr
}
