package testdb

import (
	"context"
	"crypto/sha1" //nolint:gosec // identifier derivation, not security
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// hashCommentPrefix marks a template database as fully built and records
// when it was last used. The comment lives in the shared pg_shdescription
// catalog, so it is readable and writable from the maintenance connection
// without connecting to the template itself. Since the template NAME carries
// the migrations hash (#2419), the comment's remaining jobs are (a) "this
// build finished" — an unstamped template may be half built — and (b) the
// last-used timestamp the template GC collects stale templates by.
const hashCommentPrefix = "phx-migrations-hash:"

// touchedCommentKey separates the hash from the last-used unix timestamp in
// the template comment: "phx-migrations-hash:<hash> touched:<unix>".
const touchedCommentKey = " touched:"

// AuthRolePassword is the password of the cluster-global phoenix_auth role on
// the TEST server. Roles are cluster-global, not database-bound, so every
// worktree sharing one postgres-test container must agree on this value —
// otherwise whoever migrates last silently changes it for everyone. This is a
// fixture for a local throwaway test database, not a credential: no
// production or staging system uses it, and it only ever reaches the test
// cluster on port 5433 / the CI service container.
const AuthRolePassword = "phoenix_auth_test"

// templateHashLen is how many hex characters of the migrations hash go into
// the template name. 48 bits of collision resistance for a name that only
// has to distinguish a handful of concurrently checked out branches.
const templateHashLen = 12

// maxIdentifierLen is PostgreSQL's identifier limit (NAMEDATALEN-1 bytes).
const maxIdentifierLen = 63

// templateMaxIdleAge is how long a template survives without being used
// before the sweep drops it. Long enough to keep a branch you return to
// mid-week warm, short enough that abandoned branches do not accumulate.
const templateMaxIdleAge = 7 * 24 * time.Hour

var hexHash = regexp.MustCompile(`^[0-9a-f]+$`)

// templateNameForHash derives the migration-scoped template name
// <base>_<12 hex of hash>, truncating base so the result fits PostgreSQL's
// 63-byte identifier limit.
func templateNameForHash(base, hash string) string {
	suffix := hash
	if !hexHash.MatchString(suffix) || len(suffix) < templateHashLen {
		// Test hooks (WithMigrationsHash) pass arbitrary strings; hash them
		// down so the name stays a valid, deterministic identifier.
		sum := sha1.Sum([]byte(hash)) //nolint:gosec // identifier derivation, not security
		suffix = hex.EncodeToString(sum[:])
	}
	suffix = suffix[:templateHashLen]

	if maxBase := maxIdentifierLen - templateHashLen - 1; len(base) > maxBase {
		base = base[:maxBase]
	}
	return base + "_" + suffix
}

// This package deliberately does NOT import database/migrations: the test
// helper package imports testdb, and the migrations package's own tests
// import the test helper package — linking the registry here would close an
// import cycle. The template is therefore built via `go run . migrate`
// (exactly what CI and developers run) and verified against the migration
// filenames, whose digit prefixes are the names bun records in
// bun_migrations. At least 6 digits: real migrations are 000001_… /
// 001015302_…; the 2-digit 00_migrations.go is registry infrastructure and
// registers no migration — matching it would make the completeness check
// permanently false and turn every CI adopt into a full rebuild.
var migrationFilePattern = regexp.MustCompile(`^(\d{6,})_.+\.go$`)

// TemplateOption customizes EnsureTemplate. The build and verify hooks exist
// for the lifecycle's own tests; production callers use the defaults (run
// `go run . migrate` / compare bun_migrations against the migration files).
type TemplateOption func(*templateOptions)

type templateOptions struct {
	hash   string
	build  func(ctx context.Context, templateDSN string) error
	verify func(ctx context.Context, templateDSN string) (bool, error)
}

// WithMigrationsHash overrides the migrations-source hash (tests only).
func WithMigrationsHash(hash string) TemplateOption {
	return func(o *templateOptions) { o.hash = hash }
}

// WithBuild overrides how a freshly created template database is populated
// (tests only; default runs all migrations via `go run . migrate`).
func WithBuild(build func(ctx context.Context, templateDSN string) error) TemplateOption {
	return func(o *templateOptions) { o.build = build }
}

// WithVerify overrides the completeness check used to adopt an unstamped
// template (tests only; default checks bun_migrations against the migration
// filenames).
func WithVerify(verify func(ctx context.Context, templateDSN string) (bool, error)) TemplateOption {
	return func(o *templateOptions) { o.verify = verify }
}

func migrationsDir() (string, error) {
	backend, err := backendRoot()
	if err != nil {
		return "", fmt.Errorf("locate backend module root: %w", err)
	}
	return filepath.Join(backend, "database", "migrations"), nil
}

// MigrationsHash returns a stable hash over all migration sources in
// backend/database/migrations. Any edit, addition, or removal changes it.
func MigrationsHash() (string, error) {
	dir, err := migrationsDir()
	if err != nil {
		return "", err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("read migrations directory: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		// _test.go files never affect the schema; hashing them would force
		// a pointless ~25s template rebuild after editing a migration test.
		if !e.IsDir() && filepath.Ext(e.Name()) == ".go" && !strings.HasSuffix(e.Name(), "_test.go") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	h := sha256.New()
	for _, name := range names {
		content, err := os.ReadFile(filepath.Join(dir, name)) //nolint:gosec // paths come from ReadDir of a repo directory
		if err != nil {
			return "", fmt.Errorf("read migration %s: %w", name, err)
		}
		_, _ = fmt.Fprintf(h, "%s\x00%d\x00", name, len(content))
		h.Write(content)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// EnsureTemplate makes sure the migration-scoped template database exists
// and is fully built, and returns the resolved config whose TemplateName is
// that template (<base>_<12 hex of the migrations hash>). Two worktrees on
// different migration states therefore own two templates side by side —
// neither can drop or overwrite the other's.
//
//   - stamped template for this hash → touched only (fast warm start)
//   - unstamped template for this hash (a build that died halfway) →
//     adopted if migration-complete, otherwise dropped and rebuilt
//   - no template for this hash, but the legacy base database exists and is
//     migration-complete (CI pre-migrates phoenix_test, local clusters carry
//     one from before hashed naming) → cloned from it instead of migrating
//   - otherwise → created and built from migrations
func EnsureTemplate(ctx context.Context, cfg *Config, opts ...TemplateOption) (*Config, error) {
	o, err := resolveTemplateOptions(opts)
	if err != nil {
		return nil, err
	}
	tcfg := cfg.ForMigrations(o.hash)
	name := tcfg.TemplateName()

	maint := openSQL(cfg.MaintenanceDSN())
	defer func() { _ = maint.Close() }()

	// Fresh templates skip the lifecycle lock. The role pin remains because a
	// different worktree may have changed the cluster-global password.
	if fresh, err := templateIsFresh(ctx, maint, name, o.hash); err != nil {
		return nil, err
	} else if fresh {
		if err := pinAuthRolePassword(ctx, maint); err != nil {
			return nil, err
		}
		return tcfg, nil
	}

	conn, unlock, err := acquireLifecycleLock(ctx, maint)
	if err != nil {
		return nil, fmt.Errorf("acquire test DB lifecycle lock: %w", err)
	}
	defer unlock()

	if err := ensureAuthRolePassword(ctx, conn); err != nil {
		return nil, err
	}
	if err := ensureTemplateLocked(ctx, conn, cfg, tcfg, o); err != nil {
		return nil, err
	}
	return tcfg, nil
}

func resolveTemplateOptions(opts []TemplateOption) (templateOptions, error) {
	o := templateOptions{build: buildTemplate, verify: migrationsComplete}
	for _, opt := range opts {
		opt(&o)
	}
	if o.hash != "" {
		return o, nil
	}
	hash, err := MigrationsHash()
	if err != nil {
		return templateOptions{}, err
	}
	o.hash = hash
	return o, nil
}

func ensureTemplateLocked(ctx context.Context, conn sqlExecutor, cfg, tcfg *Config, o templateOptions) error {
	name := tcfg.TemplateName()
	exists, comment, err := templateState(ctx, conn, name)
	if err != nil {
		return err
	}
	if exists {
		ready, err := prepareExistingTemplate(ctx, conn, tcfg, comment, o)
		if err != nil || ready {
			return err
		}
	} else if adopted, err := adoptBaseTemplate(ctx, conn, cfg, tcfg, o.hash, o.verify); err != nil {
		return err
	} else if adopted {
		return stampTemplate(ctx, conn, name, o.hash)
	}

	if _, err := conn.ExecContext(ctx, `CREATE DATABASE `+quoteIdentifier(name)); err != nil {
		return fmt.Errorf("create template database %q: %w", name, err)
	}
	if err := o.build(ctx, tcfg.TemplateDSN()); err != nil {
		return fmt.Errorf("build template database %q: %w", name, err)
	}
	return stampTemplate(ctx, conn, name, o.hash)
}

func prepareExistingTemplate(ctx context.Context, conn sqlExecutor, tcfg *Config, comment string, o templateOptions) (bool, error) {
	name := tcfg.TemplateName()
	if strings.HasPrefix(comment, hashCommentPrefix) {
		return true, stampTemplate(ctx, conn, name, o.hash)
	}
	// An unstamped template is either a build that died halfway or a database
	// created outside the lifecycle. Adopt it only when migration-complete.
	complete, err := o.verify(ctx, tcfg.TemplateDSN())
	if err != nil {
		return false, fmt.Errorf("verify unstamped template %q: %w", name, err)
	}
	if complete {
		return true, stampTemplate(ctx, conn, name, o.hash)
	}
	if err := dropDatabase(ctx, conn, name); err != nil {
		return false, fmt.Errorf("drop incomplete template %q: %w", name, err)
	}
	return false, nil
}

// templateStampInterval is how long a template's "touched" stamp is trusted
// before EnsureTemplate refreshes it under the lock. It only feeds the
// template GC (templateMaxIdleAge, a week), so refreshing it once a minute
// per run instead of once per test binary loses nothing.
const templateStampInterval = time.Minute

// templateIsFresh reports whether the template exists, carries this
// migrations hash, and was stamped within templateStampInterval.
func templateIsFresh(ctx context.Context, maint sqlExecutor, name, hash string) (bool, error) {
	exists, comment, err := templateState(ctx, maint, name)
	if err != nil || !exists {
		return false, err
	}
	if !strings.HasPrefix(comment, hashCommentPrefix+hash+touchedCommentKey) {
		return false, nil
	}
	touched, ok := touchedAt(comment)
	return ok && time.Since(touched) < templateStampInterval, nil
}

// adoptBaseTemplate copies the legacy base database (the name in
// TEST_DB_DSN, e.g. CI's pre-migrated phoenix_test) into the hashed template
// when it is migration-complete — a file copy instead of a ~25s migrate run.
// The base itself is never modified or dropped. Reports whether the hashed
// template now exists; a failed copy falls back to a regular build.
//
// A base carrying a FOREIGN hash stamp is never adopted: the completeness
// check only compares migration versions, so it cannot see that another
// branch edited the content of an existing migration.
func adoptBaseTemplate(ctx context.Context, conn sqlExecutor, cfg, tcfg *Config, hash string,
	verify func(context.Context, string) (bool, error)) (bool, error) {
	base := cfg.BaseTemplateName()
	if base == tcfg.TemplateName() {
		return false, nil
	}
	exists, comment, err := templateState(ctx, conn, base)
	if err != nil || !exists {
		return false, err
	}
	if strings.HasPrefix(comment, hashCommentPrefix) && !strings.HasPrefix(comment, hashCommentPrefix+hash) {
		return false, nil
	}
	complete, err := verify(ctx, cfg.DatabaseDSN(base))
	if err != nil {
		return false, fmt.Errorf("verify base template %q: %w", base, err)
	}
	if !complete {
		return false, nil
	}
	if _, err := conn.ExecContext(ctx,
		`CREATE DATABASE `+quoteIdentifier(tcfg.TemplateName())+` TEMPLATE `+quoteIdentifier(base)); err != nil {
		// Most likely someone holds a connection to the base database.
		// Building from migrations always works, so this is never fatal.
		_ = dropDatabase(ctx, conn, tcfg.TemplateName())
		return false, nil
	}
	return true, nil
}

// pinAuthRolePassword is ensureAuthRolePassword for the callers that hold no
// lifecycle lock. ALTER ROLE writes a pg_authid tuple, and two sessions doing
// that at the same moment fail with "tuple concurrently updated" (XX000) — on
// CI that took 1041 tests down at once, because the pin runs inside
// SetupTestDB and a failure there fails the whole package binary.
//
// The lock is taken inside ensureAuthRolePassword so the lifecycle path and
// this lock-free path cannot accidentally use different exclusion domains.
func pinAuthRolePassword(ctx context.Context, db *sql.DB) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("open connection for phoenix_auth password pin: %w", err)
	}
	defer func() { _ = conn.Close() }()
	return ensureAuthRolePassword(ctx, conn)
}

// ensureAuthRolePassword pins the cluster-global phoenix_auth role of the
// TEST server to AuthRolePassword. Migration 1.14.1 only creates the role IF
// NOT EXISTS, so without this the first worktree to migrate decides the
// password for every other worktree on the same container (#2419). No-op
// when the role does not exist yet — the migration creates it with the same
// value, because buildTemplate exports it.
//
// The auth-role lock is session-scoped and deliberately taken here, on the
// same connection as ALTER ROLE. A lifecycle lock alone does not exclude the
// lock-free fast path because it uses a different advisory-lock key.
func ensureAuthRolePassword(ctx context.Context, conn *sql.Conn) error {
	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, authRoleAdvisoryLockKey); err != nil {
		return fmt.Errorf("lock phoenix_auth password pin: %w", err)
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, authRoleAdvisoryLockKey)
	}()

	_, err := conn.ExecContext(ctx, fmt.Sprintf(`
		DO $$ BEGIN
			IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'phoenix_auth') THEN
				ALTER ROLE phoenix_auth WITH PASSWORD %s;
			END IF;
		END $$`, quoteLiteral(AuthRolePassword)))
	if err != nil {
		return fmt.Errorf("pin phoenix_auth test password: %w", err)
	}
	return nil
}

func quoteLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

type sqlExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func templateState(ctx context.Context, maint sqlExecutor, name string) (exists bool, comment string, err error) {
	var nullComment sql.NullString
	err = maint.QueryRowContext(ctx, `
		SELECT sd.description
		FROM pg_database d
		LEFT JOIN pg_shdescription sd ON sd.objoid = d.oid AND sd.classoid = 'pg_database'::regclass
		WHERE d.datname = $1`, name).Scan(&nullComment)
	if err == sql.ErrNoRows {
		return false, "", nil
	}
	if err != nil {
		return false, "", fmt.Errorf("read template state for %q: %w", name, err)
	}
	return true, nullComment.String, nil
}

// stampTemplate marks the template as fully built and records the current
// time as its last use; the sweep's template GC reads that timestamp.
func stampTemplate(ctx context.Context, maint sqlExecutor, name, hash string) error {
	comment := hashCommentPrefix + hash + touchedCommentKey + strconv.FormatInt(time.Now().Unix(), 10)
	stmt := fmt.Sprintf(`COMMENT ON DATABASE %s IS %s`, quoteIdentifier(name), quoteLiteral(comment))
	if _, err := maint.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("stamp template %q: %w", name, err)
	}
	return nil
}

func dropDatabase(ctx context.Context, maint sqlExecutor, name string) error {
	_, err := maint.ExecContext(ctx, `DROP DATABASE IF EXISTS `+quoteIdentifier(name)+` WITH (FORCE)`)
	return err
}

// buildTemplate runs all migrations against the (empty) template database via
// `go run . migrate` in the backend module root — the same entrypoint CI and
// developers use. The role-creating migration reads PHOENIX_AUTH_PASSWORD;
// it is deliberately NOT inherited from the worktree environment but pinned
// to AuthRolePassword, because the role is cluster-global and every worktree
// on the shared test container must end up with the same password (#2419).
func buildTemplate(ctx context.Context, templateDSN string) error {
	backend, err := backendRoot()
	if err != nil {
		return fmt.Errorf("locate backend module root: %w", err)
	}

	cmd := exec.CommandContext(ctx, "go", "run", ".", "migrate")
	cmd.Dir = backend
	cmd.Env = append(os.Environ(),
		"APP_ENV=test",
		"TEST_DB_DSN="+templateDSN,
		"PHOENIX_AUTH_PASSWORD="+AuthRolePassword,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go run . migrate: %w\n%s", err, out)
	}
	return nil
}

// normalizeMigrationVersion makes filename prefixes (001015301) and Bun's
// semantic migration names (1.15.301) comparable.
func normalizeMigrationVersion(version string) string {
	if strings.Contains(version, ".") {
		parts := strings.Split(version, ".")
		for i, part := range parts {
			n, err := strconv.ParseInt(part, 10, 64)
			if err != nil {
				return version
			}
			parts[i] = strconv.FormatInt(n, 10)
		}
		return strings.Join(parts, ".")
	}
	if len(version)%3 != 0 {
		return version
	}
	if len(version) == 6 {
		return normalizeMigrationVersion(version[:3] + "." + version[3:] + ".0")
	}
	parts := make([]string, 0, len(version)/3)
	for i := 0; i < len(version); i += 3 {
		n, err := strconv.ParseInt(version[i:i+3], 10, 64)
		if err != nil {
			return version
		}
		parts = append(parts, strconv.FormatInt(n, 10))
	}
	return strings.Join(parts, ".")
}

// migrationsComplete reports whether every migration file's version is
// applied in the database behind dsn.
func migrationsComplete(ctx context.Context, dsn string) (bool, error) {
	dir, err := migrationsDir()
	if err != nil {
		return false, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, fmt.Errorf("read migrations directory: %w", err)
	}

	var wanted []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		if m := migrationFilePattern.FindStringSubmatch(name); m != nil {
			wanted = append(wanted, normalizeMigrationVersion(m[1]))
		}
	}
	if len(wanted) == 0 {
		return false, fmt.Errorf("no migration files found in %s", dir)
	}

	db := openSQL(dsn)
	defer func() { _ = db.Close() }()

	var tableExists bool
	if err := db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_tables WHERE schemaname = 'public' AND tablename = 'bun_migrations')`,
	).Scan(&tableExists); err != nil {
		return false, fmt.Errorf("check bun_migrations table: %w", err)
	}
	if !tableExists {
		return false, nil
	}

	rows, err := db.QueryContext(ctx, `SELECT name FROM bun_migrations`)
	if err != nil {
		return false, fmt.Errorf("read applied migrations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	applied := make(map[string]struct{})
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return false, err
		}
		applied[normalizeMigrationVersion(name)] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}

	return migrationSetsMatch(wanted, applied), nil
}

func migrationSetsMatch(wanted []string, applied map[string]struct{}) bool {
	expected := make(map[string]struct{}, len(wanted))
	for _, version := range wanted {
		expected[version] = struct{}{}
	}
	if len(expected) != len(applied) {
		return false
	}
	for version := range expected {
		if _, ok := applied[version]; !ok {
			return false
		}
	}
	return true
}
