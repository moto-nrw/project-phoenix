package test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/database"
	"github.com/moto-nrw/project-phoenix/internal/testdb"
	"github.com/spf13/viper"
	"github.com/subosito/gotenv"
	"github.com/uptrace/bun"
)

// This file wires SetupTestDB to the test-database lifecycle in
// internal/testdb (ADR 0004): the process-once setup below makes `go test`
// self-initializing (server up, template current per migrations hash, one
// run-stamped clone per package binary) and keeps the per-test path free of
// t.Setenv/viper writes so tests can run in parallel.

var (
	packageTestDBOnce sync.Once
	packageTestDBErr  error

	// packageClone pins this binary's clone via a keeper connection for the
	// whole process lifetime, so the cross-run generation GC never collects
	// a clone whose tests are still running. Deliberately never closed.
	packageClone *testdb.CloneHandle
)

// initPackageTestDB is the process-once part of SetupTestDB: environment,
// lifecycle (server/template/clone), viper config, and the one-time clone
// bootstrap. Everything after it is per-test and parallel-safe.
func initPackageTestDB() error {
	// Force test environment so database config always resolves to the test
	// DB, regardless of how `go test` was invoked. Process-wide instead of
	// t.Setenv: t.Setenv forbids t.Parallel(), and tests that need a
	// different APP_ENV set their own via t.Setenv (which overrides this).
	if err := os.Setenv("APP_ENV", "test"); err != nil {
		return err
	}

	// Load .env from project root (contains TEST_DB_DSN). Best-effort: CI
	// provides the variables directly.
	if projectRoot, err := testdb.ProjectRoot(); err == nil {
		_ = gotenv.Load(filepath.Join(projectRoot, ".env"))
	}

	viper.AutomaticEnv()

	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		return fmt.Errorf(`test database not configured.

To run integration tests, ensure .env contains:
  TEST_DB_DSN=postgres://postgres:postgres@localhost:5433/phoenix_test?sslmode=disable

The suite starts the postgres-test container and migrates the template
automatically. For CI, set TEST_DB_DSN as an environment variable`)
	}

	cfg, err := testdb.NewConfig(dsn)
	if err != nil {
		return err
	}

	// Template rebuilds run all migrations (~25s); everything else is fast.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := testdb.EnsureServer(ctx, cfg); err != nil {
		return err
	}
	if err := testdb.EnsureTemplate(ctx, cfg); err != nil {
		return err
	}
	clone, err := testdb.CreateClone(ctx, cfg, testdb.RunID())
	if err != nil {
		return err
	}
	packageClone = clone

	applyViperTestConfig()

	db, err := database.DBConn()
	if err != nil {
		return fmt.Errorf("connect to package test database: %w", err)
	}
	defer func() { _ = db.Close() }()

	// PostgreSQL roles live at cluster scope, while package clones are rebuilt
	// independently. Keep the least-privilege role used by api.New in sync
	// with the current test environment so a clone created before a password
	// change cannot make router tests fail authentication.
	password := strings.ReplaceAll(os.Getenv("PHOENIX_AUTH_PASSWORD"), "'", "''")
	if _, err := db.ExecContext(ctx, "ALTER ROLE phoenix_auth PASSWORD '"+password+"'"); err != nil {
		return fmt.Errorf("sync phoenix_auth password for package test database: %w", err)
	}

	return initCloneBootstrap(ctx, db)
}

// applyViperTestConfig points viper at the package clone. APP_ENV=test
// deliberately resolves only test_db_dsn; setting db_dsn would be ignored and
// would silently send integration tests to the shared template database.
//
// Pool caps: each SetupTestDB call still opens its own pool (tests
// `defer db.Close()`); the PR-2 sweep replaces these with one pool per
// package. 3 conns/test fits comfortably within max_connections even with
// all packages running in parallel.
func applyViperTestConfig() {
	viper.AutomaticEnv()
	viper.Set("test_db_dsn", packageClone.DSN)
	viper.Set("db_debug", false) // Set to true for SQL debugging
	viper.Set("db_max_open_conns", 3)
	viper.Set("db_max_idle_conns", 1)
}

// initCloneBootstrap seeds the per-package clone once: sequence offsets, the
// default tenant (school 1), the default room, and the system staff fixture.
// The fixed bootstrap entities disappear with the PR-2 per-test-tenant sweep.
func initCloneBootstrap(ctx context.Context, db *bun.DB) error {
	if err := applySequenceOffsets(ctx, db); err != nil {
		return fmt.Errorf("apply test sequence offsets: %w", err)
	}

	// Ensure the default tenant (school ID 1) exists in platform.schools.
	// Legacy fixtures use tenant_id=1, which requires a FK target row.
	if err := ensureBootstrapTenant(ctx, db); err != nil {
		return fmt.Errorf("ensure default test tenant: %w", err)
	}

	// Ensure default fallback room exists (ID 1).
	// session_service.go:determineRoomIDWithStrategy uses hardcoded fallback
	// to room 1 when no room is specified and no planned room exists.
	if _, err := db.ExecContext(ctx, `
		INSERT INTO facilities.rooms (id, tenant_id, name, building)
		VALUES (1, 1, 'Default Room', 'Default')
		ON CONFLICT (id) DO UPDATE
		SET tenant_id = EXCLUDED.tenant_id, name = EXCLUDED.name, building = EXCLUDED.building`); err != nil {
		return fmt.Errorf("ensure default room fixture (id=1): %w", err)
	}

	// Ensure default system staff exists (person ID 1, staff ID 1) for legacy
	// tests that hardcode CreatedBy: 1 to satisfy created_by FK constraints.
	// Each ID is reconciled so an adopted legacy template cannot retain a
	// bootstrap row from another tenant.
	if err := db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, e := tx.ExecContext(ctx, `
			INSERT INTO users.persons (id, tenant_id, first_name, last_name)
			VALUES (1, 1, 'System', 'Test')
			ON CONFLICT (id) DO UPDATE
			SET tenant_id = EXCLUDED.tenant_id, first_name = EXCLUDED.first_name, last_name = EXCLUDED.last_name`); e != nil {
			return fmt.Errorf("ensure system person fixture (id=1): %w", e)
		}
		if _, e := tx.ExecContext(ctx, `
			INSERT INTO users.staff (id, tenant_id, person_id)
			VALUES (1, 1, 1)
			ON CONFLICT (id) DO UPDATE
			SET tenant_id = EXCLUDED.tenant_id, person_id = EXCLUDED.person_id`); e != nil {
			return fmt.Errorf("ensure system staff fixture (id=1): %w", e)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("ensure system person/staff fixtures (id=1): %w", err)
	}

	// Advance the BIGSERIAL sequences past the explicitly-inserted IDs.
	if _, err := db.ExecContext(ctx, `
		SELECT setval('facilities.rooms_id_seq', GREATEST((SELECT last_value FROM facilities.rooms_id_seq), (SELECT COALESCE(MAX(id), 1) FROM facilities.rooms))),
		       setval('users.persons_id_seq', GREATEST((SELECT last_value FROM users.persons_id_seq), (SELECT COALESCE(MAX(id), 1) FROM users.persons))),
		       setval('users.staff_id_seq', GREATEST((SELECT last_value FROM users.staff_id_seq), (SELECT COALESCE(MAX(id), 1) FROM users.staff)))`); err != nil {
		return fmt.Errorf("advance bootstrap sequences: %w", err)
	}

	return nil
}

// ensureBootstrapTenant reconciles ID 1 from a legacy template before its
// dependent bootstrap fixtures are created. Unlike ordinary test tenants,
// this fixed ID is an invariant of legacy fixtures and cannot retain data
// owned by another tenant.
func ensureBootstrapTenant(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `
		INSERT INTO platform.organizations (id, name, slug, active)
		VALUES (1, 'Test Org 1', 'test-org-1', true)
		ON CONFLICT (id) DO UPDATE
		SET name = EXCLUDED.name, slug = EXCLUDED.slug, active = EXCLUDED.active`); err != nil {
		return fmt.Errorf("ensure bootstrap organization: %w", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO platform.schools (id, organization_id, name, slug, subdomain, active)
		VALUES (1, 1, 'Test School 1', 'test-school-1', 't1', true)
		ON CONFLICT (id) DO UPDATE
		SET organization_id = EXCLUDED.organization_id, name = EXCLUDED.name,
			slug = EXCLUDED.slug, subdomain = EXCLUDED.subdomain, active = EXCLUDED.active`); err != nil {
		return fmt.Errorf("ensure bootstrap school: %w", err)
	}
	if _, err := db.ExecContext(ctx, `
		SELECT setval(pg_get_serial_sequence('platform.organizations', 'id'),
			GREATEST((SELECT last_value FROM platform.organizations_id_seq), 1)),
		       setval(pg_get_serial_sequence('platform.schools', 'id'),
			GREATEST((SELECT last_value FROM platform.schools_id_seq), 1))`); err != nil {
		return fmt.Errorf("advance bootstrap tenant sequences: %w", err)
	}
	return nil
}
