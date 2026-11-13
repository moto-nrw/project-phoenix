package test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/moto-ogs/project-phoenix/internal/testdb"
	transactionruntime "github.com/moto-ogs/project-phoenix/modules/transactionruntime/compose"
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
	// from; isolated clones copy its template.
	packageCloneCfg *testdb.Config

	// packageMaintenance is the binary's lasting maintenance session. Every
	// clone the binary creates or drops goes through it (#6962).
	packageMaintenance *testdb.Maintenance

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
// lifecycle (server/template/clone) and viper config. The template already
// carries the bootstrap rows (testdb.prepareTemplate). Everything after it is
// per-test and parallel-safe.
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
	// The generation GC runs once per binary, here; isolated clones skip it.
	packageMaintenance = testdb.OpenMaintenance(templateCfg)
	if err := packageMaintenance.CollectDeadRuns(ctx, templateCfg, testdb.RunID()); err != nil {
		return err
	}
	clone, err := packageMaintenance.CreateClone(ctx, templateCfg, testdb.RunID())
	if err != nil {
		return err
	}
	packageClone = clone
	packageCloneCfg = templateCfg

	if err := applyDatabaseTestConfig(clone.DSN); err != nil {
		return fmt.Errorf("configure package test database: %w", err)
	}
	applyViperTestConfig()
	sharedTestDB, err = openPackageDB()
	if err != nil {
		return err
	}
	if err := bindPackageTenantRuntime(sharedTestDB); err != nil {
		return fmt.Errorf("bind package tenant runtime: %w", err)
	}
	// The clone inherited the template's start snapshot; only a missing or
	// outdated one is taken again here.
	if err := testdb.EnsureSharedBaseline(ctx, clone.DSN); err != nil {
		return fmt.Errorf("ensure clone baseline: %w", err)
	}
	return nil
}

// applyDatabaseTestConfig points database.DBConn at this package's clone and
// supplies the explicit pool configuration required by the production
// database boundary. These are test-process settings, not runtime fallbacks.
func applyDatabaseTestConfig(dsn string) error {
	settings := map[string]string{
		"TEST_DB_DSN":           dsn,
		"DB_MAX_OPEN_CONNS":     strconv.Itoa(poolSize()),
		"DB_MAX_IDLE_CONNS":     strconv.Itoa(poolSize()),
		"DB_CONN_MAX_LIFETIME":  "30m",
		"DB_CONN_MAX_IDLE_TIME": "10m",
	}
	for name, value := range settings {
		if err := os.Setenv(name, value); err != nil {
			return fmt.Errorf("set %s: %w", name, err)
		}
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

Use TEST_DB_DSN from the server-owning checkout's root .env, not
backend/dev.env.example. An exported TEST_DB_DSN takes precedence.
See docs/agents/operations.md#test-database.

The suite starts the postgres-test container and migrates its template
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
	// Protect the server before connecting, even when the wrapper prepared the
	// template. An idle server may have stopped during a long compilation.
	if err := testdb.EnsureServer(ctx, cfg); err != nil {
		return nil, err
	}
	if name := os.Getenv(testdb.TemplateEnv); name != "" {
		return cfg.WithTemplate(name), nil
	}
	return testdb.EnsureTemplate(ctx, cfg)
}

// openPackageDB opens the binary's shared pool on its clone. The clone got its
// search_path from the maintenance session before any connection existed, so
// every pooled connection inherits it.
func openPackageDB() (*bun.DB, error) {
	db, err := transactionruntime.OpenPool()
	if err != nil {
		return nil, fmt.Errorf("open shared package test pool: %w", err)
	}
	attachTestQueryHooks(db)
	return db, nil
}

// applyViperTestConfig restores the fixed service config and points viper at
// the package clone. APP_ENV=test deliberately resolves only test_db_dsn;
// setting db_dsn would be ignored and would silently send integration tests
// to the shared template database.
func applyViperTestConfig() {
	viper.AutomaticEnv()
	applyStaticViperTestConfig()
	viper.Set("test_db_dsn", packageClone.DSN)
	viper.Set("db_debug", false) // Set to true for SQL debugging
}

func applyStaticViperTestConfig() {
	viper.Set("auth_jwt_secret", TestJWTSecret)
	viper.Set("health_data_keys", TestHealthDataKeys)
	viper.Set("bank_data_keys", TestBankDataKeys)
	viper.Set("auth_jwt_expiry", "15m")
	viper.Set("auth_jwt_refresh_expiry", "1h")
	viper.Set("frontend_url", "http://tenant.invalid")
	viper.Set("parents_url", "http://parents.invalid")
	viper.Set("school_url", "http://school.invalid")
	viper.Set("caterer_url", "http://caterer.invalid")
	viper.Set("traeger_url", "http://traeger.invalid")
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
