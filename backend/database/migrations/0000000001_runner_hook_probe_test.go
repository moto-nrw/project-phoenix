package migrations

import (
	"context"
	"errors"
	"strings"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
)

// This file is NOT a migration. It is named like one on purpose: bun derives a
// Go migration's name from the file that calls Migrations.Register, so the only
// way to register one from a test — and therefore the only way to prove that
// bun runs the #3300 hooks on the Go path all 498 real migrations take — is from
// a file whose name matches bun's `<digits>_<description>` rule.
//
// It registers into throwaway migrate.Migrations values, never into the package's
// Migrations, and the `_test.go` suffix keeps it out of the binary and out of
// every migration source scan.

// probeMigrations returns a one-migration set whose up function is fn. Each call
// needs its own set: bun names both probes after this file, so two registrations
// in one set would collide.
func probeMigrations(fn func(ctx context.Context, db *bun.DB) error) *migrate.Migrations {
	probe := migrate.NewMigrations()
	probe.MustRegister(fn, func(context.Context, *bun.DB) error { return nil })
	return probe
}

// probeMigrator keeps the probe's bookkeeping out of the real bun_migrations
// table, which the test clone arrives with fully populated.
func probeMigrator(db *bun.DB, probe *migrate.Migrations, runLog *runnerLog, table string) *migrate.Migrator {
	options := []migrate.MigratorOption{
		migrate.WithMarkAppliedOnSuccess(true),
		migrate.WithTableName(table),
		migrate.WithLocksTableName(table + "_locks"),
	}
	return migrate.NewMigrator(db, probe, append(options, runLog.migratorOptions()...)...)
}

// TestMigratorHooksLogEveryAppliedGoMigration proves the hooks are actually
// wired and that bun calls them for Go migrations. Every assertion elsewhere
// about the log line's shape rests on this: without it the runner could be
// perfectly correct and still say nothing.
func TestMigratorHooksLogEveryAppliedGoMigration(t *testing.T) {
	// Safe in parallel: TestMain enables PerTestDatabases, so this test gets its
	// own disposable clone and its own bookkeeping table inside it.
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	ran := false
	probe := probeMigrations(func(ctx context.Context, db *bun.DB) error {
		ran = true
		_, err := db.ExecContext(ctx, `SELECT 1`)
		return err
	})

	logger, records := captureLogger(t)
	runLog := newRunnerLog(logger)
	migrator := probeMigrator(db, probe, runLog, "bun_migrations_hook_probe")
	if err := migrator.Init(ctx); err != nil {
		t.Fatalf("init probe migrator: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = db.ExecContext(cleanupCtx, `DROP TABLE IF EXISTS bun_migrations_hook_probe CASCADE`)
		_, _ = db.ExecContext(cleanupCtx, `DROP TABLE IF EXISTS bun_migrations_hook_probe_locks CASCADE`)
	})

	if _, err := migrator.Migrate(ctx); err != nil {
		t.Fatalf("run probe migration: %v", err)
	}
	if !ran {
		t.Fatal("probe migration did not run")
	}

	logged := records()
	if len(logged) != 1 {
		t.Fatalf("got %d log lines, want exactly one per applied migration: %v", len(logged), logged)
	}
	if logged[0]["msg"] != "migration applied" {
		t.Errorf("msg = %v, want %q", logged[0]["msg"], "migration applied")
	}
	// No registry entry exists for the probe, so it logs what bun knows.
	if logged[0]["version"] != "0000000001" {
		t.Errorf("version = %v, want the probe's bun name", logged[0]["version"])
	}
	if _, ok := logged[0]["duration_ms"]; !ok {
		t.Errorf("duration_ms missing from %v", logged[0])
	}
}

// TestMigratorHooksLeaveTheFailedMigrationToTheRunner pins the division of
// labour behind logFailure: bun returns without calling the after hook, so the
// failing migration produces no line of its own.
func TestMigratorHooksLeaveTheFailedMigrationToTheRunner(t *testing.T) {
	// Safe in parallel: TestMain enables PerTestDatabases.
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	wantErr := errors.New("probe migration failed")
	probe := probeMigrations(func(context.Context, *bun.DB) error { return wantErr })

	logger, records := captureLogger(t)
	runLog := newRunnerLog(logger)
	migrator := probeMigrator(db, probe, runLog, "bun_migrations_fail_probe")
	if err := migrator.Init(ctx); err != nil {
		t.Fatalf("init probe migrator: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = db.ExecContext(cleanupCtx, `DROP TABLE IF EXISTS bun_migrations_fail_probe CASCADE`)
		_, _ = db.ExecContext(cleanupCtx, `DROP TABLE IF EXISTS bun_migrations_fail_probe_locks CASCADE`)
	})

	_, err := migrator.Migrate(ctx)
	if !errors.Is(err, wantErr) {
		t.Fatalf("Migrate error = %v, want it to wrap %v", err, wantErr)
	}
	if logged := records(); len(logged) != 0 {
		t.Fatalf("bun logged %d lines for a failed migration; the runner owns that line: %v", len(logged), logged)
	}

	runLog.logFailure(ctx, err)
	logged := records()
	if len(logged) != 1 {
		t.Fatalf("got %d log lines after logFailure, want 1: %v", len(logged), logged)
	}
	if logged[0]["level"] != "ERROR" {
		t.Errorf("level = %v, want ERROR", logged[0]["level"])
	}
	if logged[0]["version"] != "0000000001" {
		t.Errorf("version = %v, want the migration that was in flight", logged[0]["version"])
	}
	if msg, _ := logged[0]["error"].(string); !strings.Contains(msg, wantErr.Error()) {
		t.Errorf("error = %v, want it to name %v", logged[0]["error"], wantErr)
	}
}
