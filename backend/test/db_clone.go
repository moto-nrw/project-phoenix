package test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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

	// sharedTestDB is the one connection pool per test binary (#2419 PR 2).
	// Every SetupTestDB call returns this handle; tests never close it — the
	// pool dies with the process. Deliberately never closed.
	sharedTestDB *bun.DB

	// viperHealMu serializes the viper self-heal in SetupTestDB against
	// parallel tests in packages that also call viper.Reset().
	viperHealMu sync.Mutex
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

	// Bake search_path into the clone so every pooled connection gets it.
	// BUN's Relation() JOIN generation sometimes uses unqualified table names,
	// which fails when the target schema isn't in search_path. A per-session
	// SET would only reach one pooled connection; ALTER DATABASE reaches all
	// connections opened after it — the shared pool below is opened lazily,
	// so its connections all inherit this.
	if _, err := db.ExecContext(ctx, fmt.Sprintf(
		`ALTER DATABASE %q SET search_path TO public, platform, auth, users, education, facilities, activities, active, schedule, iot, feedback, config, meta, audit`,
		clone.Name)); err != nil {
		_ = db.Close()
		return fmt.Errorf("set clone search_path: %w", err)
	}

	if err := initCloneBootstrap(ctx, db); err != nil {
		_ = db.Close()
		return err
	}
	// The bootstrap pool predates the ALTER DATABASE above, so its existing
	// connections may lack the search_path. Recycle it for the shared pool.
	if err := db.Close(); err != nil {
		return fmt.Errorf("close bootstrap pool: %w", err)
	}

	sharedTestDB, err = database.DBConn()
	if err != nil {
		return fmt.Errorf("open shared package test pool: %w", err)
	}
	return nil
}

// applyViperTestConfig points viper at the package clone. APP_ENV=test
// deliberately resolves only test_db_dsn; setting db_dsn would be ignored and
// would silently send integration tests to the shared template database.
//
// Pool budget: one pool per test binary, capped at min(GOMAXPROCS, 8) open
// connections, with idle == open so the pool holds what it opens instead of
// re-dialing per test. `go test` runs up to GOMAXPROCS package binaries at
// once, so the steady-state cost is GOMAXPROCS × (pool cap + 1 keeper):
// 8 × 9 = 72 on the 8-vCPU CI runner, against postgres:17's default
// max_connections=100. The cap also matches `-parallel` (default
// GOMAXPROCS), so parallel tests rarely queue.
//
// Two caveats worth knowing before raising anything here:
//   - The per-binary cap stops growing at 8, but the number of concurrent
//     binaries does not. Past ~21 cores the local budget (cores × 9) exceeds
//     the compose file's max_connections=200; cap `go test -p` on such a
//     machine rather than raising the server limit.
//   - SetupClosableTestDB opens additional private pools (same cap) for the
//     handful of tests that close their database on purpose. They are
//     short-lived, but they are on top of this budget.
func applyViperTestConfig() {
	viper.AutomaticEnv()
	viper.Set("test_db_dsn", packageClone.DSN)
	viper.Set("db_debug", false) // Set to true for SQL debugging
	viper.Set("db_max_open_conns", min(runtime.GOMAXPROCS(0), 8))
	viper.Set("db_max_idle_conns", min(runtime.GOMAXPROCS(0), 8))
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
