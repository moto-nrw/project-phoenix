package test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	// a clone whose tests are still running. Released by dropPackageClone at
	// the very end of the run.
	packageClone *testdb.CloneHandle

	// packageCloneCfg is the resolved lifecycle config the clone was created
	// from; dropPackageClone needs its maintenance DSN.
	packageCloneCfg *testdb.Config

	// sharedTestDB is the one connection pool per test binary (#2419 PR 2).
	// Every SetupTestDB call returns this handle; tests never close it — the
	// pool dies with the process. Deliberately never closed.
	sharedTestDB *bun.DB

	// viperHealMu serializes the viper self-heal in SetupTestDB against
	// parallel tests in packages that also call viper.Reset().
	viperHealMu sync.Mutex
)

// Pin service configuration before any test can build a router. SetupTestDB
// is lazy, but a router may be constructed before the first DB fixture and
// must not capture AUTH_JWT_SECRET from the developer's shell in that window.
func init() {
	viper.AutomaticEnv()
	applyStaticViperTestConfig()
}

// initPackageTestDB is the process-once part of SetupTestDB: environment,
// lifecycle (server/template/clone), viper config, and the one-time clone
// bootstrap. Everything after it is per-test and parallel-safe.
func initPackageTestDB() error {
	cfg, err := packageTestConfig()
	if err != nil {
		return err
	}

	// Template rebuilds run all migrations (~25s); everything else is fast.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	templateCfg, err := resolvePackageTemplate(ctx, cfg)
	if err != nil {
		return err
	}
	clone, err := testdb.CreateClone(ctx, templateCfg, testdb.RunID())
	if err != nil {
		return err
	}
	packageClone = clone
	packageCloneCfg = templateCfg

	applyViperTestConfig()
	sharedTestDB, err = openBootstrappedPackageDB(ctx, clone)
	if err != nil {
		return err
	}
	if err := bindPackageTenantRuntime(sharedTestDB); err != nil {
		return fmt.Errorf("bind package tenant runtime: %w", err)
	}
	if err := testdb.SnapshotSharedBaseline(ctx, clone.DSN); err != nil {
		return fmt.Errorf("snapshot clone baseline: %w", err)
	}
	return nil
}

func packageTestConfig() (*testdb.Config, error) {
	// Process-wide because t.Setenv forbids t.Parallel().
	if err := os.Setenv("APP_ENV", "test"); err != nil {
		return nil, err
	}
	dsn, err := loadTestDSN()
	if err != nil {
		return nil, err
	}
	// phoenix_auth is cluster-global, so every worktree uses one test value.
	if err := os.Setenv("PHOENIX_AUTH_PASSWORD", testdb.AuthRolePassword); err != nil {
		return nil, err
	}
	viper.AutomaticEnv()
	if dsn == "" {
		return nil, fmt.Errorf(`test database not configured.

To run integration tests, ensure .env contains:
  TEST_DB_DSN=postgres://postgres:postgres@localhost:5433/phoenix_test?sslmode=disable

The suite starts the postgres-test container and migrates the template
automatically. For CI, set TEST_DB_DSN as an environment variable`)
	}
	return testdb.NewConfig(dsn)
}

func loadTestDSN() (string, error) {
	if dsn := os.Getenv("TEST_DB_DSN"); dsn != "" {
		return dsn, nil
	}
	// Read only TEST_DB_DSN. Loading the whole .env would let a developer's
	// JWT secret change test behavior through viper.AutomaticEnv.
	projectRoot, err := testdb.ProjectRoot()
	if err != nil {
		return "", nil
	}
	env, err := gotenv.Read(filepath.Join(projectRoot, ".env"))
	if err != nil || env["TEST_DB_DSN"] == "" {
		return "", nil
	}
	dsn := env["TEST_DB_DSN"]
	if err := os.Setenv("TEST_DB_DSN", dsn); err != nil {
		return "", err
	}
	return dsn, nil
}

func resolvePackageTemplate(ctx context.Context, cfg *testdb.Config) (*testdb.Config, error) {
	// The wrapper resolves the template once per run. Naked `go test` calls
	// these steps in each binary and remains self-initializing.
	if name := os.Getenv(testdb.TemplateEnv); name != "" {
		return cfg.WithTemplate(name), nil
	}
	if err := testdb.EnsureServer(ctx, cfg); err != nil {
		return nil, err
	}
	return testdb.EnsureTemplate(ctx, cfg)
}

func openBootstrappedPackageDB(ctx context.Context, clone *testdb.CloneHandle) (*bun.DB, error) {
	db, err := database.DBConn()
	if err != nil {
		return nil, fmt.Errorf("connect to package test database: %w", err)
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
		return nil, fmt.Errorf("set clone search_path: %w", err)
	}

	if err := initCloneBootstrap(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	// The bootstrap pool predates the ALTER DATABASE above, so its existing
	// connections may lack the search_path. Recycle it for the shared pool.
	if err := db.Close(); err != nil {
		return nil, fmt.Errorf("close bootstrap pool: %w", err)
	}
	sharedDB, err := database.DBConn()
	if err != nil {
		return nil, fmt.Errorf("open shared package test pool: %w", err)
	}
	return sharedDB, nil
}

// applyViperTestConfig restores the fixed service config and points viper at
// the package clone. APP_ENV=test deliberately resolves only test_db_dsn;
// setting db_dsn would be ignored and would silently send integration tests
// to the shared template database.
//
// Pool budget: one pool per test binary, sized from the binary's OWN
// parallelism (poolSize), with idle == open so the pool holds what it opens
// instead of re-dialing per test.
func applyViperTestConfig() {
	viper.AutomaticEnv()
	applyStaticViperTestConfig()
	viper.Set("test_db_dsn", packageClone.DSN)
	viper.Set("db_debug", false) // Set to true for SQL debugging
	viper.Set("db_max_open_conns", poolSize())
	viper.Set("db_max_idle_conns", poolSize())
}

func applyStaticViperTestConfig() {
	viper.Set("auth_jwt_secret", TestJWTSecret)
	viper.Set("auth_jwt_expiry", "15m")
	viper.Set("auth_jwt_refresh_expiry", "1h")
	viper.Set("frontend_url", "http://tenant.invalid")
	viper.Set("parents_url", "http://parents.invalid")
	viper.Set("school_url", "http://school.invalid")
	viper.Set("tenant_domain", "tenant.invalid")
	viper.Set("next_public_operator_hostname", "operator.invalid")
}

// nestedConnHeadroom is the slack above -parallel. A test that holds a tenant
// transaction and then opens a second one needs two connections at the same
// time; without headroom, `-parallel` such tests take every connection and
// then wait forever for the next one. The failure looks nothing like a pool
// problem — every affected test just fails on its own 5s context deadline —
// which is why the size is derived rather than guessed.
const nestedConnHeadroom = 4

// poolSizeCap bounds the derived size. `go test` runs up to `-p` (default
// GOMAXPROCS) package binaries at once, so the server-side cost is
// p × (pool + 1 keeper). The cap keeps a 16-core machine under the compose
// file's max_connections; CI and the wrapper additionally pin -p/-parallel
// (see scripts/test-backend.sh).
const poolSizeCap = 16

// poolSize returns the per-binary connection budget: this binary's own
// -parallel plus headroom for tests that hold more than one connection.
// Reading the flag rather than GOMAXPROCS is the point — the pool must cover
// the tests that actually run at the same time, and `go test -parallel N`
// changes exactly that number.
func poolSize() int {
	return min(parallelism()+nestedConnHeadroom, poolSizeCap)
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

	// Ensure the legacy bootstrap room (ID 1) exists: older fixtures hardcode
	// room_id=1 and need a FK target row. It disappears with the PR-2
	// per-test-tenant sweep, like the other fixed bootstrap entities.
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
	// Lift both tenant sequences clear of the test-tenant band before the
	// first test runs. Two things depend on it: a service-path school
	// (platform.CreateSchool) can never collide with an ID CreateTestTenant
	// hands out explicitly, and — because every tenant a test creates is then
	// at or above testdb.TenantIDBase — the leftover gate can tell a test's
	// own tenant from shared state by looking at the ID alone.
	if _, err := db.ExecContext(ctx, `
		SELECT setval(pg_get_serial_sequence('platform.organizations', 'id'),
			GREATEST((SELECT last_value FROM platform.organizations_id_seq), ?)),
		       setval(pg_get_serial_sequence('platform.schools', 'id'),
			GREATEST((SELECT last_value FROM platform.schools_id_seq), ?))`,
		testdb.TenantIDCeiling, testdb.TenantIDCeiling); err != nil {
		return fmt.Errorf("advance bootstrap tenant sequences: %w", err)
	}
	return nil
}
