package database

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/transactionruntime/protocol"
)

func newTestQueryHook(t *testing.T) (*QueryHook, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return NewQueryHook(logger), &buf
}

func TestQueryLogLevelsAndMetadata(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data queryLogData
		want []string
	}{
		{name: "no rows", data: queryLogData{noRows: true, err: errors.New("no rows")}, want: []string{`"level":"DEBUG"`, `"msg":"query no rows"`}},
		{name: "postgres error", data: queryLogData{err: errors.New("duplicate"), sqlState: "23505", constraint: "unique_row"}, want: []string{`"level":"ERROR"`, `"sqlstate":"23505"`, `"constraint":"unique_row"`}},
		{name: "slow", data: queryLogData{started: time.Now().Add(-10 * time.Millisecond)}, want: []string{`"level":"WARN"`, `"slow_query":true`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook, buf := newTestQueryHook(t)
			if tt.data.started.IsZero() {
				tt.data.started = time.Now()
			}
			hook.logQuery(context.Background(), tt.data)
			for _, want := range tt.want {
				if !strings.Contains(buf.String(), want) {
					t.Fatalf("log %q does not contain %q", buf.String(), want)
				}
			}
		})
	}
}

type testNoRowsError struct{}

func (testNoRowsError) Error() string { return "wrapped repository miss" }
func (testNoRowsError) Is(target error) bool {
	return target != nil && target.Error() == "sql: no rows in result set"
}

type testPostgresError map[byte]string

func (e testPostgresError) Error() string         { return "postgres error" }
func (e testPostgresError) Field(key byte) string { return e[key] }

func TestAfterQueryClassifiesRawAdapterErrors(t *testing.T) {
	t.Parallel()

	t.Run("no rows", func(t *testing.T) {
		hook, buf := newTestQueryHook(t)
		hook.afterQuery(context.Background(), "SELECT", "SELECT 1", time.Now(), testNoRowsError{})
		if output := buf.String(); !strings.Contains(output, `"msg":"query no rows"`) {
			t.Fatalf("expected raw no-rows error to be demoted, got %q", output)
		}
	})

	t.Run("postgres metadata", func(t *testing.T) {
		hook, buf := newTestQueryHook(t)
		hook.afterQuery(context.Background(), "INSERT", "INSERT", time.Now(), testPostgresError{'C': "23505", 'n': "unique_row"})
		output := buf.String()
		for _, want := range []string{`"sqlstate":"23505"`, `"constraint":"unique_row"`} {
			if !strings.Contains(output, want) {
				t.Fatalf("log %q does not contain %q", output, want)
			}
		}
	})
}

// A SEPA mandate's sealed IBAN and account holder, and an IBAN that reached
// the SQL in the clear, never appear in the query log (#4695).
func TestQueryLogRedactsBankData(t *testing.T) {
	t.Parallel()
	hook, buf := newTestQueryHook(t)
	query := `INSERT INTO sepa_mandates (reference, account_holder_sealed, iban_sealed) ` +
		`VALUES ('SJR-1', '\x0102abcdef', '\x01ff00') -- DE89370400440532013000`
	hook.logQuery(context.Background(), queryLogData{operation: "INSERT", query: query, started: time.Now()})
	logged := buf.String()
	for _, secret := range []string{"0102abcdef", "01ff00", "DE89370400440532013000"} {
		if strings.Contains(logged, secret) {
			t.Fatalf("the query log repeats %q: %s", secret, logged)
		}
	}
	if !strings.Contains(logged, "[redacted]") || !strings.Contains(logged, "SJR-1") {
		t.Fatalf("the query log should keep the rest of the query: %s", logged)
	}
}

// TestMarkedStatementsLoseTheirTypedTextInTheLog keeps a search text out of
// the slow-query log: bun writes bound values into the logged SQL (ADR 0231).
func TestMarkedStatementsLoseTheirTypedTextInTheLog(t *testing.T) {
	t.Parallel()
	hook, buf := newTestQueryHook(t)
	hook.logQuery(context.Background(), queryLogData{
		operation: "SELECT",
		query:     protocol.RedactLiteralsInLog + `SELECT 1 FROM education.groups AS grp CROSS JOIN (SELECT '%' || public.f_unaccent('O''Brien 0170 1234567') || '%' AS p) AS q`,
		started:   time.Now().Add(-10 * time.Millisecond),
	})
	logged := buf.String()
	for _, typed := range []string{"Brien", "0170", "1234567"} {
		if strings.Contains(logged, typed) {
			t.Fatalf("log %q still names the typed text %q", logged, typed)
		}
	}
	if !strings.Contains(logged, "education.groups") {
		t.Fatalf("log %q lost the statement itself", logged)
	}

	unmarked, buf := newTestQueryHook(t)
	unmarked.logQuery(context.Background(), queryLogData{
		operation: "SELECT", query: `SELECT 1 FROM rooms WHERE name = 'Atelier'`,
		started: time.Now().Add(-10 * time.Millisecond),
	})
	if !strings.Contains(buf.String(), "Atelier") {
		t.Fatalf("an unmarked statement keeps its literals for diagnosis: %q", buf.String())
	}
}
