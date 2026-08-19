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
	assert.Equal(t, "1.15.301", normalizeMigrationVersion("001015301"))
	assert.Equal(t, "1.15.301", normalizeMigrationVersion("1.15.301"))
	assert.Equal(t, "0.0.0", normalizeMigrationVersion("000000"))
	assert.Equal(t, "0.1.0", normalizeMigrationVersion("000001"))
}

func TestMigrationsHashIsStableAndSourceSensitive(t *testing.T) {
	h1, err := MigrationsHash()
	require.NoError(t, err)
	h2, err := MigrationsHash()
	require.NoError(t, err)
	assert.Equal(t, h1, h2)
	assert.Len(t, h1, 64)
}

func TestMigrationSetsMatchRequiresExactSet(t *testing.T) {
	wanted := []string{"0.0.0", "1.15.301"}
	assert.True(t, migrationSetsMatch(wanted, map[string]struct{}{
		"0.0.0":    {},
		"1.15.301": {},
	}))
	assert.False(t, migrationSetsMatch(wanted, map[string]struct{}{
		"0.0.0":    {},
		"1.15.301": {},
		"1.15.302": {},
	}), "newer migrations must force a template rebuild")
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

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	require.NoError(t, EnsureServer(ctx, cfg))

	t.Cleanup(func() {
		maint := openSQL(cfg.MaintenanceDSN())
		defer func() { _ = maint.Close() }()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// Drop the base name AND every migration-scoped template derived
		// from it (<base>_<hash>) — EnsureTemplate builds those, not the base.
		derived, _ := listDatabasesByPrefix(ctx, maint, name+"_")
		for _, d := range derived {
			_ = dropDatabase(ctx, maint, d)
		}
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

	_, err := EnsureTemplate(ctx, cfg, build, WithMigrationsHash("hash1"))
	require.NoError(t, err)
	assert.Equal(t, 1, builds, "fresh template must be built")

	_, err = EnsureTemplate(ctx, cfg, build, WithMigrationsHash("hash1"))
	require.NoError(t, err)
	assert.Equal(t, 1, builds, "unchanged hash must not rebuild")

	_, err = EnsureTemplate(ctx, cfg, build, WithMigrationsHash("hash2"))
	require.NoError(t, err)
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
	_, err = EnsureTemplate(ctx, cfg,
		WithBuild(fakeBuild(&builds)),
		WithVerify(func(ctx context.Context, dsn string) (bool, error) {
			verifyCalls++
			return true, nil
		}),
		WithMigrationsHash("hash1"))
	require.NoError(t, err)

	assert.Equal(t, 0, builds, "complete unstamped template must be adopted, not rebuilt")
	assert.Equal(t, 1, verifyCalls)

	// Adopted template is now stamped: a second call must not verify again.
	_, err = EnsureTemplate(ctx, cfg,
		WithBuild(fakeBuild(&builds)),
		WithVerify(func(ctx context.Context, dsn string) (bool, error) {
			verifyCalls++
			return true, nil
		}),
		WithMigrationsHash("hash1"))
	require.NoError(t, err)
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
	_, err = EnsureTemplate(ctx, cfg,
		WithBuild(fakeBuild(&builds)),
		WithVerify(func(ctx context.Context, dsn string) (bool, error) { return false, nil }),
		WithMigrationsHash("hash1"))
	require.NoError(t, err)
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
	require.NoError(t, EnsureServer(ctx, cfg))

	// Ensure the template exists before checking it (a pristine server has
	// none yet), and hold the lifecycle lock during the check: it opens
	// connections to the template, which would otherwise fail concurrent
	// CREATE DATABASE ... TEMPLATE calls from parallel package binaries.
	templateCfg, err := EnsureTemplate(ctx, cfg)
	require.NoError(t, err)

	maint := openSQL(cfg.MaintenanceDSN())
	defer func() { _ = maint.Close() }()
	_, unlock, err := acquireLifecycleLock(ctx, maint)
	require.NoError(t, err)
	defer unlock()

	complete, err := migrationsComplete(ctx, templateCfg.TemplateDSN())
	require.NoError(t, err)
	assert.True(t, complete, "the migrated template must pass the default completeness check — otherwise CI rebuilds instead of adopting")
}

func TestCreateCloneAndSweepLifecycle(t *testing.T) {
	cfg := integrationConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	builds := 0
	templateCfg, err := EnsureTemplate(ctx, cfg, WithBuild(fakeBuild(&builds)), WithMigrationsHash("hash1"))
	require.NoError(t, err)

	runID := SanitizeRunID("")
	handle, err := CreateClone(ctx, templateCfg, runID)
	require.NoError(t, err)
	t.Cleanup(func() { _ = handle.Close() })

	require.True(t, strings.HasPrefix(handle.Name, ClonePrefix+runID+"_"))
	require.True(t, databaseExists(t, ctx, cfg, handle.Name))

	// The keeper connection pins the clone: a GC pass from a foreign run
	// (empty spare prefix) must not collect it.
	maint := openSQL(cfg.MaintenanceDSN())
	defer func() { _ = maint.Close() }()
	conn, unlock, err := acquireLifecycleLock(ctx, maint)
	require.NoError(t, err)
	_, err = gcLocked(ctx, conn, "", templateCfg.TemplateName())
	unlock()
	require.NoError(t, err)
	assert.True(t, databaseExists(t, ctx, cfg, handle.Name), "GC must spare a clone with a live keeper connection")

	// The sweep drops this run's clones WITH (FORCE), so the keeper stays
	// open until after the sweep — closing it early would open a window for
	// a concurrent foreign run's GC to collect the clone first.
	result, err := Sweep(ctx, templateCfg, SweepOptions{RunID: runID})
	require.NoError(t, err)
	assert.Contains(t, result.Dropped, handle.Name)
	assert.False(t, databaseExists(t, ctx, cfg, handle.Name), "sweep must drop this run's clone")
}

func TestSweepReportsLeftovers(t *testing.T) {
	cfg := integrationConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	builds := 0
	templateCfg, err := EnsureTemplate(ctx, cfg, WithBuild(fakeBuild(&builds)), WithMigrationsHash("hash1"))
	require.NoError(t, err)

	runID := SanitizeRunID("")
	handle, err := CreateClone(ctx, templateCfg, runID)
	require.NoError(t, err)
	t.Cleanup(func() { _ = handle.Close() })

	clone := openSQL(handle.DSN)
	_, err = clone.ExecContext(ctx, `INSERT INTO marker (id) VALUES (42)`)
	require.NoError(t, clone.Close())
	require.NoError(t, err)

	result, err := Sweep(ctx, templateCfg, SweepOptions{RunID: runID, ReportLeftovers: true})
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

func TestTemplateNameForHashDerivesOneNamePerMigrationHand(t *testing.T) {
	hashA := strings.Repeat("a", 64)
	hashB := strings.Repeat("b", 64)

	name := templateNameForHash("phoenix_test", hashA)
	assert.Equal(t, "phoenix_test_aaaaaaaaaaaa", name)
	assert.Equal(t, name, templateNameForHash("phoenix_test", hashA), "derivation must be deterministic")
	assert.NotEqual(t, name, templateNameForHash("phoenix_test", hashB),
		"two migration hands must get two templates")
	assert.True(t, strings.HasPrefix(name, "phoenix_test_"), "the DSN name stays the prefix")

	// Non-hex hashes (the WithMigrationsHash test hook) are hashed down to a
	// valid identifier instead of leaking arbitrary characters into a name.
	hooked := templateNameForHash("phoenix_test", "hash1")
	assert.Regexp(t, `^phoenix_test_[0-9a-f]{12}$`, hooked)
	assert.Equal(t, hooked, templateNameForHash("phoenix_test", "hash1"))
	assert.NotEqual(t, hooked, templateNameForHash("phoenix_test", "hash2"))

	// PostgreSQL identifiers are capped at 63 bytes.
	long := templateNameForHash(strings.Repeat("x", 200), hashA)
	assert.Len(t, long, 63)
	assert.True(t, strings.HasSuffix(long, "_aaaaaaaaaaaa"))
}

func TestConfigForMigrationsScopesTemplateOnly(t *testing.T) {
	base, err := NewConfig("postgres://user:pass@localhost:5433/phoenix_test?sslmode=disable")
	require.NoError(t, err)

	scoped := base.ForMigrations(strings.Repeat("a", 64))
	assert.Equal(t, "phoenix_test", scoped.BaseTemplateName())
	assert.Equal(t, "phoenix_test_aaaaaaaaaaaa", scoped.TemplateName())
	assert.Equal(t,
		"postgres://user:pass@localhost:5433/phoenix_test_aaaaaaaaaaaa?sslmode=disable",
		scoped.TemplateDSN())
	assert.Equal(t, "phoenix_test", base.TemplateName(), "the source config stays untouched")
}

func TestTouchedAtParsesTemplateStamp(t *testing.T) {
	stamp, ok := touchedAt(hashCommentPrefix + "abc" + touchedCommentKey + "1700000000")
	require.True(t, ok)
	assert.Equal(t, int64(1700000000), stamp.Unix())

	_, ok = touchedAt("")
	assert.False(t, ok)
	_, ok = touchedAt(hashCommentPrefix + "abc")
	assert.False(t, ok, "a stamp without a timestamp must not look collectible")
	_, ok = touchedAt("some other comment touched:1700000000")
	assert.False(t, ok, "foreign databases must never be treated as templates")
}

// TestEnsureTemplateLeavesForeignMigrationHandUntouched is the core
// multi-worktree guarantee: a run on migration hand X must not drop, rebuild,
// or reuse the template of migration hand Y.
func TestEnsureTemplateLeavesForeignMigrationHandUntouched(t *testing.T) {
	cfg := integrationConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	builds := 0
	worktreeA, err := EnsureTemplate(ctx, cfg, WithBuild(fakeBuild(&builds)), WithMigrationsHash("branch-a"))
	require.NoError(t, err)

	// Mark A's template so a rebuild would be visible in its content.
	dbA := openSQL(worktreeA.TemplateDSN())
	_, err = dbA.ExecContext(ctx, `INSERT INTO marker (id) VALUES (1)`)
	require.NoError(t, dbA.Close())
	require.NoError(t, err)

	worktreeB, err := EnsureTemplate(ctx, cfg, WithBuild(fakeBuild(&builds)), WithMigrationsHash("branch-b"))
	require.NoError(t, err)

	assert.NotEqual(t, worktreeA.TemplateName(), worktreeB.TemplateName(),
		"two migration hands must resolve to two template names")
	assert.Equal(t, 2, builds, "the second hand builds its own template")
	assert.True(t, databaseExists(t, ctx, cfg, worktreeA.TemplateName()), "A's template must survive B's run")
	assert.True(t, databaseExists(t, ctx, cfg, worktreeB.TemplateName()))

	// A's content is untouched — B neither rebuilt nor reused it.
	checkA := openSQL(worktreeA.TemplateDSN())
	defer func() { _ = checkA.Close() }()
	var rows int
	require.NoError(t, checkA.QueryRowContext(ctx, `SELECT count(*) FROM marker`).Scan(&rows))
	assert.Equal(t, 1, rows, "A's template content must be untouched")

	// And A's next run still finds its own template warm (no rebuild).
	again, err := EnsureTemplate(ctx, cfg, WithBuild(fakeBuild(&builds)), WithMigrationsHash("branch-a"))
	require.NoError(t, err)
	assert.Equal(t, worktreeA.TemplateName(), again.TemplateName())
	assert.Equal(t, 2, builds, "A's warm template must not be rebuilt after B's run")
}

func TestSweepDropsStaleTemplatesButKeepsTheCurrentOne(t *testing.T) {
	cfg := integrationConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	builds := 0
	stale, err := EnsureTemplate(ctx, cfg, WithBuild(fakeBuild(&builds)), WithMigrationsHash("old-branch"))
	require.NoError(t, err)
	current, err := EnsureTemplate(ctx, cfg, WithBuild(fakeBuild(&builds)), WithMigrationsHash("current-branch"))
	require.NoError(t, err)

	// A fresh template is never collected...
	result, err := Sweep(ctx, current, SweepOptions{})
	require.NoError(t, err)
	assert.NotContains(t, result.Dropped, stale.TemplateName(), "a recently used template must survive")
	assert.True(t, databaseExists(t, ctx, cfg, stale.TemplateName()))

	// ...until its last use falls behind the idle threshold.
	maint := openSQL(cfg.MaintenanceDSN())
	defer func() { _ = maint.Close() }()
	backdated := time.Now().Add(-templateMaxIdleAge - time.Hour).Unix()
	_, err = maint.ExecContext(ctx, fmt.Sprintf(`COMMENT ON DATABASE %s IS %s`,
		quoteIdentifier(stale.TemplateName()),
		quoteLiteral(fmt.Sprintf("%sold-branch%s%d", hashCommentPrefix, touchedCommentKey, backdated))))
	require.NoError(t, err)

	result, err = Sweep(ctx, current, SweepOptions{})
	require.NoError(t, err)
	assert.Contains(t, result.Dropped, stale.TemplateName())
	assert.False(t, databaseExists(t, ctx, cfg, stale.TemplateName()))
	assert.True(t, databaseExists(t, ctx, cfg, current.TemplateName()), "the current template is never collected")
}

// TestEnsureTemplateIgnoresForeignStampedBase pins the tighter adopt rule:
// the completeness check only compares migration VERSIONS, so a base
// database stamped by another migration hand must not be copied — its
// content may differ even when every version is present.
func TestEnsureTemplateIgnoresForeignStampedBase(t *testing.T) {
	cfg := integrationConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	maint := openSQL(cfg.MaintenanceDSN())
	defer func() { _ = maint.Close() }()
	_, err := maint.ExecContext(ctx, `CREATE DATABASE `+quoteIdentifier(cfg.TemplateName()))
	require.NoError(t, err)
	require.NoError(t, stampTemplate(ctx, maint, cfg.TemplateName(), "foreign-hand"))

	builds := 0
	_, err = EnsureTemplate(ctx, cfg,
		WithBuild(fakeBuild(&builds)),
		WithVerify(func(ctx context.Context, dsn string) (bool, error) { return true, nil }),
		WithMigrationsHash("my-hand"))
	require.NoError(t, err)
	assert.Equal(t, 1, builds, "a base stamped by another migration hand must be built from migrations, not copied")
}
