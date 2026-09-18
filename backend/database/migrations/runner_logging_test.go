package migrations

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/uptrace/bun/migrate"
)

// logRecord is one decoded slog JSON line. Decoding rather than substring
// matching is what makes the assertions about field names meaningful: the point
// of #3300 is that every migration line carries version, description and
// duration_ms as fields a Loki query can select on.
type logRecord map[string]any

// captureLogger returns a JSON logger at debug level and a function that decodes
// everything written to it so far.
func captureLogger(t *testing.T) (*slog.Logger, func() []logRecord) {
	t.Helper()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	return logger, func() []logRecord {
		t.Helper()

		var records []logRecord
		decoder := json.NewDecoder(bytes.NewReader(buf.Bytes()))
		for decoder.More() {
			var record logRecord
			if err := decoder.Decode(&record); err != nil {
				t.Fatalf("decode log output %q: %v", buf.String(), err)
			}
			records = append(records, record)
		}
		return records
	}
}

// TestBunNameIndexCoversEveryRegisteredMigration guards the index the runner
// resolves its version from. callerBunName reads the stack, so a silent miss
// would not fail any build — it would just make every log line fall back to the
// filename prefix. Three migrations legitimately have no registry entry; any
// fourth means the index stopped working.
func TestBunNameIndexCoversEveryRegisteredMigration(t *testing.T) {
	t.Parallel()

	// 001006006 is the one migration that calls Migrations.MustRegister without a
	// MigrationRegistry entry; it carries its dependencies in a bespoke
	// Dependencies001006006 var instead.
	unregistered := map[string]bool{"001006006": true}

	var missing []string
	for _, bunMigration := range Migrations.Sorted() {
		if _, ok := MigrationByBunName(bunMigration.Name); ok {
			continue
		}
		if unregistered[bunMigration.Name] {
			continue
		}
		missing = append(missing, bunMigration.Name)
	}
	if len(missing) > 0 {
		t.Errorf("no registry entry reachable by bun name for %v — "+
			"the runner would log these migrations by filename prefix instead of version", missing)
	}

	if len(migrationsByBunName) != len(MigrationRegistry) {
		t.Errorf("bun name index holds %d entries, MigrationRegistry holds %d; "+
			"callerBunName failed to identify the calling file for %d migration(s)",
			len(migrationsByBunName), len(MigrationRegistry),
			len(MigrationRegistry)-len(migrationsByBunName))
	}
}

// TestMigrationIdentityResolvesBunNameToRegisteredVersion pins the mapping the
// whole feature rests on. 0010060171 is the case that rules out deriving the
// version from the prefix arithmetically: it packs the four segments of 1.6.17.1.
func TestMigrationIdentityResolvesBunNameToRegisteredVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		bunName         string
		bunComment      string
		wantVersion     string
		wantDescription string
	}{
		{
			name:        "four segment version packed into one prefix",
			bunName:     "0010060171",
			wantVersion: AddPositionToInvitationTokensVersion,
		},
		{
			name:        "three segment version",
			bunName:     "001015392",
			wantVersion: roomsRetireAtSchoolColorVersion,
		},
		{
			name:            "migration without a registry entry falls back to what bun knows",
			bunName:         "001006006",
			bunComment:      "update_activity_categories",
			wantVersion:     "001006006",
			wantDescription: "update_activity_categories",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			version, description := migrationIdentity(&migrate.Migration{
				Name:    test.bunName,
				Comment: test.bunComment,
			})
			if version != test.wantVersion {
				t.Errorf("version = %q, want %q", version, test.wantVersion)
			}
			if test.wantDescription != "" && description != test.wantDescription {
				t.Errorf("description = %q, want %q", description, test.wantDescription)
			}
			if test.wantDescription == "" && description == "" {
				t.Error("description is empty; the registry entry carries one")
			}
		})
	}
}

// TestRunnerLogWritesOneLinePerAppliedMigration is the first acceptance criterion
// of #3300: one slog line per applied migration, carrying version, description
// and duration_ms.
func TestRunnerLogWritesOneLinePerAppliedMigration(t *testing.T) {
	t.Parallel()

	logger, records := captureLogger(t)
	runLog := newRunnerLog(logger)
	ctx := context.Background()
	migration := &migrate.Migration{Name: "001015392"}

	if err := runLog.before(ctx, nil, migration); err != nil {
		t.Fatalf("before hook: %v", err)
	}
	if err := runLog.after(ctx, nil, migration); err != nil {
		t.Fatalf("after hook: %v", err)
	}

	logged := records()
	if len(logged) != 1 {
		t.Fatalf("got %d log lines, want exactly 1: %v", len(logged), logged)
	}

	line := logged[0]
	if line["msg"] != "migration applied" {
		t.Errorf("msg = %v, want %q", line["msg"], "migration applied")
	}
	if line["level"] != "INFO" {
		t.Errorf("level = %v, want INFO", line["level"])
	}
	if line["component"] != migrationLogComponent {
		t.Errorf("component = %v, want %q", line["component"], migrationLogComponent)
	}
	if line["version"] != roomsRetireAtSchoolColorVersion {
		t.Errorf("version = %v, want %q", line["version"], roomsRetireAtSchoolColorVersion)
	}
	if line["description"] != roomsRetireAtSchoolColorDescription {
		t.Errorf("description = %v, want the registered description", line["description"])
	}
	if _, ok := line["duration_ms"]; !ok {
		t.Errorf("duration_ms missing from %v", line)
	}
}

// TestRunnerLogReportsTheMigrationThatFailed covers the gap bun leaves: it skips
// the after hook when a migration returns an error, so without logFailure the run
// would end without ever naming the migration that broke.
func TestRunnerLogReportsTheMigrationThatFailed(t *testing.T) {
	t.Parallel()

	logger, records := captureLogger(t)
	runLog := newRunnerLog(logger)
	ctx := context.Background()

	if err := runLog.before(ctx, nil, &migrate.Migration{Name: "001015392"}); err != nil {
		t.Fatalf("before hook: %v", err)
	}
	runLog.logFailure(ctx, errors.New("relation already exists"))

	logged := records()
	if len(logged) != 1 {
		t.Fatalf("got %d log lines, want exactly 1: %v", len(logged), logged)
	}

	line := logged[0]
	if line["level"] != "ERROR" {
		t.Errorf("level = %v, want ERROR", line["level"])
	}
	if line["msg"] != "migration failed" {
		t.Errorf("msg = %v, want %q", line["msg"], "migration failed")
	}
	if line["version"] != roomsRetireAtSchoolColorVersion {
		t.Errorf("version = %v, want %q", line["version"], roomsRetireAtSchoolColorVersion)
	}
	if line["error"] != "relation already exists" {
		t.Errorf("error = %v, want the migration's error", line["error"])
	}
}

// TestRunnerLogStaysSilentWithoutAnInFlightMigration keeps a successful run from
// emitting a spurious failure line: Migrate can return an error before any
// migration started, for instance while loading migration status.
func TestRunnerLogStaysSilentWithoutAnInFlightMigration(t *testing.T) {
	t.Parallel()

	logger, records := captureLogger(t)
	runLog := newRunnerLog(logger)

	runLog.logFailure(context.Background(), errors.New("connection refused"))

	if logged := records(); len(logged) != 0 {
		t.Errorf("got %d log lines, want none: %v", len(logged), logged)
	}
}

// TestPendingVersionRangeNamesBothEnds pins the one-line plan summary that
// replaced printing all ~500 planned migrations on every server start.
func TestPendingVersionRangeNamesBothEnds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		pending   migrate.MigrationSlice
		wantFirst string
		wantLast  string
	}{
		{
			name:    "nothing pending",
			pending: nil,
		},
		{
			name:      "single migration is both ends",
			pending:   migrate.MigrationSlice{{Name: "001015392"}},
			wantFirst: roomsRetireAtSchoolColorVersion,
			wantLast:  roomsRetireAtSchoolColorVersion,
		},
		{
			name: "range spans the run",
			pending: migrate.MigrationSlice{
				{Name: "0010060171"},
				{Name: "001015391"},
				{Name: "001015392"},
			},
			wantFirst: AddPositionToInvitationTokensVersion,
			wantLast:  roomsRetireAtSchoolColorVersion,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			first, last := pendingVersionRange(test.pending)
			if first != test.wantFirst {
				t.Errorf("first = %q, want %q", first, test.wantFirst)
			}
			if last != test.wantLast {
				t.Errorf("last = %q, want %q", last, test.wantLast)
			}
		})
	}
}

// TestMigrateWritesOneLineWhenNothingIsPending is the second acceptance
// criterion of #3300. Compose and the native dev start run `migrate` before
// every server start, so this is the common case: before the change it printed
// a ~500-line migration plan into the startup log every time.
func TestMigrateWritesOneLineWhenNothingIsPending(t *testing.T) {
	// Safe in parallel: TestMain enables PerTestDatabases, and a run with
	// nothing pending writes nothing to the clone anyway.
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	logger, records := captureLogger(t)
	if err := migrateWithLogger(context.Background(), db, logger); err != nil {
		t.Fatalf("migrate against an up-to-date database: %v", err)
	}

	logged := records()
	if len(logged) != 1 {
		t.Fatalf("got %d log lines, want exactly 1: %v", len(logged), logged)
	}
	if logged[0]["msg"] != "no pending migrations" {
		t.Errorf("msg = %v, want %q", logged[0]["msg"], "no pending migrations")
	}
	if logged[0]["component"] != migrationLogComponent {
		t.Errorf("component = %v, want %q", logged[0]["component"], migrationLogComponent)
	}
	// The count proves the test really ran against a migrated database rather
	// than an empty one, where "nothing pending" would be trivially true.
	applied, ok := logged[0]["applied"].(float64)
	if !ok || int(applied) != len(Migrations.Sorted()) {
		t.Errorf("applied = %v, want all %d migrations", logged[0]["applied"], len(Migrations.Sorted()))
	}
}
