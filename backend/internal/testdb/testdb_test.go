package testdb

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/subosito/gotenv"
)

// ---------------------------------------------------------------------------
// Unit tests (no database)
// ---------------------------------------------------------------------------

func TestParsePostgresDSNRejectsInvalidInput(t *testing.T) {
	for _, tc := range []struct {
		name string
		dsn  string
	}{
		{name: "empty", dsn: ""},
		{name: "wrong scheme", dsn: "mysql://user:pass@localhost/db"},
		{name: "missing database", dsn: "postgres://user:pass@localhost:5433/"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parsePostgresDSN(tc.dsn)
			assert.Error(t, err)
		})
	}
}

func TestConfigDerivesDSNs(t *testing.T) {
	cfg, err := NewConfig("postgres://user:pass@localhost:5433/phoenix_test?sslmode=disable&application_name=tests")
	require.NoError(t, err)

	assert.Equal(t, "phoenix_test", cfg.TemplateName())
	assert.Equal(t,
		"postgres://user:pass@localhost:5433/postgres?sslmode=disable&application_name=tests",
		cfg.MaintenanceDSN())
	assert.Equal(t,
		"postgres://user:pass@localhost:5433/phx_test_pkg_abc?sslmode=disable&application_name=tests",
		cfg.DatabaseDSN("phx_test_pkg_abc"))
}

func TestQuoteIdentifierEscapesDoubleQuotes(t *testing.T) {
	assert.Equal(t, `"plain"`, quoteIdentifier("plain"))
	assert.Equal(t, `"has""quote"`, quoteIdentifier(`has"quote`))
}

func TestSanitizeRunID(t *testing.T) {
	assert.Equal(t, "abc123", SanitizeRunID("abc123"))

	hashed := SanitizeRunID("Not/A valid-ID that is way too long")
	assert.Regexp(t, `^[a-f0-9]{12}$`, hashed)
	assert.Equal(t, hashed, SanitizeRunID("Not/A valid-ID that is way too long"), "sanitizing must be deterministic")

	random := SanitizeRunID("")
	assert.Regexp(t, `^[a-z0-9]{1,16}$`, random)
}

func TestCloneNameIsValidPostgresIdentifier(t *testing.T) {
	name := CloneName("run1", "/some/pkg/dir")
	assert.True(t, strings.HasPrefix(name, ClonePrefix+"run1_"))
	assert.LessOrEqual(t, len(name), 63)
	assert.NotContains(t, name, "-")
	assert.NotContains(t, name, "/")

	assert.NotEqual(t, name, CloneName("run1", "/other/pkg/dir"), "different packages get different clones")
	assert.NotEqual(t, name, CloneName("run2", "/some/pkg/dir"), "different runs get different clones")
}

func TestMigrationFilePatternMatchesOnlyRealMigrations(t *testing.T) {
	// Regression guard: 00_migrations.go (registry infrastructure) must NOT
	// count as a wanted migration — its "00" is never a bun_migrations name,
	// so matching it would make migrationsComplete permanently false and turn
	// every CI adopt into a full template rebuild.
	assert.Nil(t, migrationFilePattern.FindStringSubmatch("00_migrations.go"))
	assert.Nil(t, migrationFilePattern.FindStringSubmatch("main.go"))

	assert.Equal(t, "000001", migrationFilePattern.FindStringSubmatch("000001_core_functions.go")[1])
	assert.Equal(t, "001015302", migrationFilePattern.FindStringSubmatch("001015302_pickup_change_requests.go")[1])
}

func TestMigrationsHashIsStableAndSourceSensitive(t *testing.T) {
	h1, err := MigrationsHash()
	require.NoError(t, err)
	h2, err := MigrationsHash()
	require.NoError(t, err)
	assert.Equal(t, h1, h2)
	assert.Len(t, h1, 64)
}

// ---------------------------------------------------------------------------
// Integration tests (need TEST_DB_DSN; use private template names so the
// real phoenix_test template is never touched)
// ---------------------------------------------------------------------------

var selfTestSeq atomic.Int64

// loadEnvForIntegration best-effort loads the project root .env when
// TEST_DB_DSN is not already set (CI sets it directly).
func loadEnvForIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv("TEST_DB_DSN") != "" {
		return
	}
	if root, err := ProjectRoot(); err == nil {
		_ = gotenv.Load(filepath.Join(root, ".env"))
	}
}

func integrationConfig(t *testing.T) *Config {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping DB integration test in -short mode")
	}

	loadEnvForIntegration(t)
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		t.Skip("TEST_DB_DSN not set")
	}

	base, err := NewConfig(dsn)
	require.NoError(t, err)

	// Rewrite the template name to a private per-test one. Deliberately NOT
	// the clone prefix: concurrent package binaries GC unconnected
	// clone-prefixed databases, and a template cannot be pinned by a keeper
	// connection (CREATE DATABASE refuses a source with active sessions).
	// t.Cleanup drops it; only a killed process can leak one.
	name := fmt.Sprintf("phx_test_selftmpl_%d_%d", os.Getpid()%100000, selfTestSeq.Add(1))
	cfg, err := NewConfig(base.DatabaseDSN(name))
	require.NoError(t, err)

	t.Cleanup(func() {
		maint := openSQL(cfg.MaintenanceDSN())
		defer func() { _ = maint.Close() }()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = dropDatabase(ctx, maint, name)
	})

	return cfg
}

// fakeBuild creates a single marker table instead of running migrations.
func fakeBuild(counter *int) func(ctx context.Context, dsn string) error {
	return func(ctx context.Context, dsn string) error {
		*counter++
		db := openSQL(dsn)
		defer func() { _ = db.Close() }()
		_, err := db.ExecContext(ctx, `CREATE TABLE marker (id bigint primary key)`)
		return err
	}
}

func TestEnsureTemplateBuildsOnlyOnHashChange(t *testing.T) {
	cfg := integrationConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	builds := 0
	build := WithBuild(fakeBuild(&builds))

	require.NoError(t, EnsureTemplate(ctx, cfg, build, WithMigrationsHash("hash1")))
	assert.Equal(t, 1, builds, "fresh template must be built")

	require.NoError(t, EnsureTemplate(ctx, cfg, build, WithMigrationsHash("hash1")))
	assert.Equal(t, 1, builds, "unchanged hash must not rebuild")

	require.NoError(t, EnsureTemplate(ctx, cfg, build, WithMigrationsHash("hash2")))
	assert.Equal(t, 2, builds, "changed hash must rebuild")
}

func TestEnsureTemplateAdoptsUnstampedCompleteDatabase(t *testing.T) {
	cfg := integrationConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Simulate the CI path: the database exists and is migration-complete,
	// but carries no hash stamp.
	maint := openSQL(cfg.MaintenanceDSN())
	defer func() { _ = maint.Close() }()
	_, err := maint.ExecContext(ctx, `CREATE DATABASE `+quoteIdentifier(cfg.TemplateName()))
	require.NoError(t, err)

	builds := 0
	verifyCalls := 0
	require.NoError(t, EnsureTemplate(ctx, cfg,
		WithBuild(fakeBuild(&builds)),
		WithVerify(func(ctx context.Context, dsn string) (bool, error) {
			verifyCalls++
			return true, nil
		}),
		WithMigrationsHash("hash1")))

	assert.Equal(t, 0, builds, "complete unstamped template must be adopted, not rebuilt")
	assert.Equal(t, 1, verifyCalls)

	// Adopted template is now stamped: a second call must not verify again.
	require.NoError(t, EnsureTemplate(ctx, cfg,
		WithBuild(fakeBuild(&builds)),
		WithVerify(func(ctx context.Context, dsn string) (bool, error) {
			verifyCalls++
			return true, nil
		}),
		WithMigrationsHash("hash1")))
	assert.Equal(t, 1, verifyCalls, "stamped template must skip verification")
	assert.Equal(t, 0, builds)
}

func TestEnsureTemplateRebuildsUnstampedIncompleteDatabase(t *testing.T) {
	cfg := integrationConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	maint := openSQL(cfg.MaintenanceDSN())
	defer func() { _ = maint.Close() }()
	_, err := maint.ExecContext(ctx, `CREATE DATABASE `+quoteIdentifier(cfg.TemplateName()))
	require.NoError(t, err)

	builds := 0
	require.NoError(t, EnsureTemplate(ctx, cfg,
		WithBuild(fakeBuild(&builds)),
		WithVerify(func(ctx context.Context, dsn string) (bool, error) { return false, nil }),
		WithMigrationsHash("hash1")))
	assert.Equal(t, 1, builds, "incomplete unstamped template must be rebuilt")
}

// TestMigrationsCompleteAgainstRealTemplate pins the CI-adopt contract: the
// actual migrated template must satisfy the default completeness check. If a
// migration naming scheme ever drifts away from the filename-prefix
// convention, this fails loudly instead of CI silently rebuilding the
// template on every run.
func TestMigrationsCompleteAgainstRealTemplate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB integration test in -short mode")
	}
	loadEnvForIntegration(t)
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		t.Skip("TEST_DB_DSN not set")
	}
	cfg, err := NewConfig(dsn)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Ensure the template exists before checking it (a pristine server has
	// none yet), and hold the lifecycle lock during the check: it opens
	// connections to the template, which would otherwise fail concurrent
	// CREATE DATABASE ... TEMPLATE calls from parallel package binaries.
	require.NoError(t, EnsureTemplate(ctx, cfg))

	maint := openSQL(cfg.MaintenanceDSN())
	defer func() { _ = maint.Close() }()
	unlock, err := acquireLifecycleLock(ctx, maint)
	require.NoError(t, err)
	defer unlock()

	complete, err := migrationsComplete(ctx, cfg.TemplateDSN())
	require.NoError(t, err)
	assert.True(t, complete, "the migrated template must pass the default completeness check — otherwise CI rebuilds instead of adopting")
}

func TestCreateCloneAndSweepLifecycle(t *testing.T) {
	cfg := integrationConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	builds := 0
	require.NoError(t, EnsureTemplate(ctx, cfg, WithBuild(fakeBuild(&builds)), WithMigrationsHash("hash1")))

	runID := SanitizeRunID("")
	handle, err := CreateClone(ctx, cfg, runID)
	require.NoError(t, err)
	t.Cleanup(func() { _ = handle.Close() })

	require.True(t, strings.HasPrefix(handle.Name, ClonePrefix+runID+"_"))
	require.True(t, databaseExists(t, ctx, cfg, handle.Name))

	// The keeper connection pins the clone: a GC pass from a foreign run
	// (empty spare prefix) must not collect it.
	maint := openSQL(cfg.MaintenanceDSN())
	defer func() { _ = maint.Close() }()
	unlock, err := acquireLifecycleLock(ctx, maint)
	require.NoError(t, err)
	_, err = gcLocked(ctx, maint, "", cfg.TemplateName())
	unlock()
	require.NoError(t, err)
	assert.True(t, databaseExists(t, ctx, cfg, handle.Name), "GC must spare a clone with a live keeper connection")

	// The sweep drops this run's clones WITH (FORCE), so the keeper stays
	// open until after the sweep — closing it early would open a window for
	// a concurrent foreign run's GC to collect the clone first.
	result, err := Sweep(ctx, cfg, SweepOptions{RunID: runID})
	require.NoError(t, err)
	assert.Contains(t, result.Dropped, handle.Name)
	assert.False(t, databaseExists(t, ctx, cfg, handle.Name), "sweep must drop this run's clone")
}

func TestSweepReportsLeftovers(t *testing.T) {
	cfg := integrationConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	builds := 0
	require.NoError(t, EnsureTemplate(ctx, cfg, WithBuild(fakeBuild(&builds)), WithMigrationsHash("hash1")))

	runID := SanitizeRunID("")
	handle, err := CreateClone(ctx, cfg, runID)
	require.NoError(t, err)
	t.Cleanup(func() { _ = handle.Close() })

	clone := openSQL(handle.DSN)
	_, err = clone.ExecContext(ctx, `INSERT INTO marker (id) VALUES (42)`)
	require.NoError(t, clone.Close())
	require.NoError(t, err)

	result, err := Sweep(ctx, cfg, SweepOptions{RunID: runID, ReportLeftovers: true})
	require.NoError(t, err)
	require.Len(t, result.Leftovers, 1)
	assert.Equal(t, handle.Name, result.Leftovers[0].Clone)
	require.Len(t, result.Leftovers[0].Tables, 1)
	assert.Equal(t, "public.marker", result.Leftovers[0].Tables[0].Table)
	assert.EqualValues(t, 0, result.Leftovers[0].Tables[0].TemplateRows)
	assert.EqualValues(t, 1, result.Leftovers[0].Tables[0].CloneRows)
	assert.Contains(t, result.Dropped, handle.Name)
}

func databaseExists(t *testing.T, ctx context.Context, cfg *Config, name string) bool {
	t.Helper()
	maint := openSQL(cfg.MaintenanceDSN())
	defer func() { _ = maint.Close() }()

	var exists bool
	err := maint.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, name).Scan(&exists)
	require.NoError(t, err)
	return exists
}
