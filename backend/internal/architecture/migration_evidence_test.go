package architecture

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateMigrationTicketAcceptsExecutableTemplate(t *testing.T) {
	t.Parallel()

	output, err := runArchitecture(t, "validate-ticket", "--ticket", "backend/architecture/migration-ticket-template.json")
	if err != nil {
		t.Fatalf("validate-ticket failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "migration ticket passed") {
		t.Fatalf("unexpected validate-ticket output:\n%s", output)
	}
}

func TestValidateMigrationTicketRejectsMissingEvidenceAndExitFields(t *testing.T) {
	t.Parallel()

	fixture := decodeTicketFixture(t)
	tests := []struct {
		name, section, field, want string
	}{
		{name: "source", section: "runtime_evidence", field: "source", want: "runtime_evidence.source is required"},
		{name: "workload", section: "runtime_evidence", field: "workload", want: "runtime_evidence.workload is required"},
		{name: "thresholds", section: "runtime_evidence", field: "thresholds", want: "runtime_evidence.thresholds is required"},
		{name: "query", section: "runtime_evidence", field: "query_count", want: "runtime_evidence.query_count is required"},
		{name: "latency p50", section: "runtime_evidence", field: "latency_p50", want: "runtime_evidence.latency_p50 is required"},
		{name: "latency p95", section: "runtime_evidence", field: "latency_p95", want: "runtime_evidence.latency_p95 is required"},
		{name: "errors", section: "runtime_evidence", field: "errors", want: "runtime_evidence.errors is required"},
		{name: "pool wait", section: "runtime_evidence", field: "pool_wait", want: "runtime_evidence.pool_wait is required"},
		{name: "lock wait", section: "runtime_evidence", field: "lock_wait", want: "runtime_evidence.lock_wait is required"},
		{name: "deadlocks", section: "runtime_evidence", field: "deadlocks", want: "runtime_evidence.deadlocks is required"},
		{name: "job duration", section: "runtime_evidence", field: "job_duration", want: "runtime_evidence.job_duration is required"},
		{name: "job retries", section: "runtime_evidence", field: "job_retries", want: "runtime_evidence.job_retries is required"},
		{name: "job backlog", section: "runtime_evidence", field: "job_backlog", want: "runtime_evidence.job_backlog is required"},
		{name: "rollback", field: "rollback_and_cleanup", want: "rollback_and_cleanup is required"},
		{name: "exit criterion", field: "exit_criterion", want: "exit_criterion is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			document := cloneTicketFixture(t, fixture)
			target := document
			if tt.section != "" {
				target = document[tt.section].(map[string]any)
			}
			delete(target, tt.field)
			output, err := runArchitecture(t, "validate-ticket", "--ticket", writeTicketFixture(t, document))
			if err == nil || !strings.Contains(output, tt.want) {
				t.Fatalf("missing field did not fail with %q: %v\n%s", tt.want, err, output)
			}
		})
	}
}

func TestValidateMigrationTicketAllowsNoRatchetKeys(t *testing.T) {
	t.Parallel()

	document := decodeTicketFixture(t)
	document["exact_ratchet_keys"] = []any{}
	output, err := runArchitecture(t, "validate-ticket", "--ticket", writeTicketFixture(t, document))
	if err != nil || !strings.Contains(output, "migration ticket passed") {
		t.Fatalf("zero-key acceptance ticket failed: %v\n%s", err, output)
	}
}

func TestValidateMigrationTicketRejectsMissingRatchetFieldAndUnknownFields(t *testing.T) {
	t.Parallel()

	t.Run("missing ratchet field", func(t *testing.T) {
		document := decodeTicketFixture(t)
		delete(document, "exact_ratchet_keys")
		output, err := runArchitecture(t, "validate-ticket", "--ticket", writeTicketFixture(t, document))
		if err == nil || !strings.Contains(output, "exact_ratchet_keys is required") {
			t.Fatalf("missing exact_ratchet_keys was accepted: %v\n%s", err, output)
		}
	})

	t.Run("unknown field", func(t *testing.T) {
		document := decodeTicketFixture(t)
		document["surprise"] = true
		output, err := runArchitecture(t, "validate-ticket", "--ticket", writeTicketFixture(t, document))
		if err == nil || !strings.Contains(output, `unknown field "surprise"`) {
			t.Fatalf("unknown field was accepted: %v\n%s", err, output)
		}
	})
}

func TestValidateMigrationTicketRejectsInvalidRatchetKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		keys []any
		want string
	}{
		{name: "empty key", keys: []any{""}, want: "contains an empty value"},
		{name: "malformed key", keys: []any{"production|imports.forbidden|source"}, want: "must use scope|rule|source|target"},
		{name: "duplicate key", keys: []any{"production|imports.forbidden|source|target", "production|imports.forbidden|source|target"}, want: "contains duplicate key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			document := decodeTicketFixture(t)
			document["exact_ratchet_keys"] = tt.keys
			output, err := runArchitecture(t, "validate-ticket", "--ticket", writeTicketFixture(t, document))
			if err == nil || !strings.Contains(output, tt.want) {
				t.Fatalf("invalid ratchet keys did not fail with %q: %v\n%s", tt.want, err, output)
			}
		})
	}
}

func TestValidateMigrationTicketRejectsOversizedInput(t *testing.T) {
	t.Parallel()

	contents, err := json.Marshal(decodeTicketFixture(t))
	if err != nil {
		t.Fatalf("marshal ticket fixture: %v", err)
	}
	contents = append(contents, []byte(strings.Repeat(" ", 1<<20))...)
	contents = append(contents, []byte("TRAILING_GARBAGE")...)
	path := filepath.Join(t.TempDir(), "oversized-ticket.json")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatalf("write oversized ticket fixture: %v", err)
	}
	output, err := runArchitecture(t, "validate-ticket", "--ticket", path)
	if err == nil || !strings.Contains(output, "migration ticket must not exceed") {
		t.Fatalf("oversized ticket was accepted: %v\n%s", err, output)
	}
}

func decodeTicketFixture(t *testing.T) map[string]any {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal([]byte(readFile(t, fixturePath(t, "migration-ticket.json"))), &document); err != nil {
		t.Fatalf("decode ticket fixture: %v", err)
	}
	return document
}

func cloneTicketFixture(t *testing.T, document map[string]any) map[string]any {
	t.Helper()
	contents, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("marshal ticket fixture: %v", err)
	}
	var clone map[string]any
	if err := json.Unmarshal(contents, &clone); err != nil {
		t.Fatalf("clone ticket fixture: %v", err)
	}
	return clone
}

func writeTicketFixture(t *testing.T, document map[string]any) string {
	t.Helper()
	contents, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("marshal ticket fixture: %v", err)
	}
	path := filepath.Join(t.TempDir(), "ticket.json")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatalf("write ticket fixture: %v", err)
	}
	return path
}
