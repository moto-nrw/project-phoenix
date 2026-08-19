package testdb

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
)

// hashCommentPrefix stamps the template database's comment with the hash of
// the migration sources it was built from. The comment lives in the shared
// pg_shdescription catalog, so it is readable and writable from the
// maintenance connection without connecting to the template itself.
const hashCommentPrefix = "phx-migrations-hash:"

// This package deliberately does NOT import database/migrations: the test
// helper package imports testdb, and the migrations package's own tests
// import the test helper package — linking the registry here would close an
// import cycle. The template is therefore built via `go run . migrate`
// (exactly what CI and developers run) and verified against the migration
// filenames, whose digit prefixes are the names bun records in
// bun_migrations.
var migrationFilePattern = regexp.MustCompile(`^(\d+)_.+\.go$`)

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
		if !e.IsDir() && filepath.Ext(e.Name()) == ".go" {
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

// EnsureTemplate makes sure the template database exists and matches the
// current migration sources:
//
//   - stamped with the current hash → untouched (fast warm start)
//   - stamped with a different hash → dropped and rebuilt from migrations
//   - unstamped but migration-complete (CI snapshot restore, legacy local
//     `migrate reset` state) → adopted by stamping only
//   - missing or unstamped-and-incomplete → built from migrations
func EnsureTemplate(ctx context.Context, cfg *Config, opts ...TemplateOption) error {
	o := templateOptions{
		build:  buildTemplate,
		verify: migrationsComplete,
	}
	for _, opt := range opts {
		opt(&o)
	}
	if o.hash == "" {
		hash, err := MigrationsHash()
		if err != nil {
			return err
		}
		o.hash = hash
	}
	want := hashCommentPrefix + o.hash

	maint := openSQL(cfg.MaintenanceDSN())
	defer func() { _ = maint.Close() }()

	unlock, err := acquireLifecycleLock(ctx, maint)
	if err != nil {
		return fmt.Errorf("acquire test DB lifecycle lock: %w", err)
	}
	defer unlock()

	exists, comment, err := templateState(ctx, maint, cfg.TemplateName())
	if err != nil {
		return err
	}

	if exists {
		if comment == want {
			return nil
		}
		if len(comment) < len(hashCommentPrefix) || comment[:len(hashCommentPrefix)] != hashCommentPrefix {
			// Unstamped template (CI snapshot restore or legacy local state):
			// adopt it if it is migration-complete instead of rebuilding.
			complete, err := o.verify(ctx, cfg.TemplateDSN())
			if err != nil {
				return fmt.Errorf("verify unstamped template %q: %w", cfg.TemplateName(), err)
			}
			if complete {
				return stampTemplate(ctx, maint, cfg.TemplateName(), want)
			}
		}
		if err := dropDatabase(ctx, maint, cfg.TemplateName()); err != nil {
			return fmt.Errorf("drop outdated template %q: %w", cfg.TemplateName(), err)
		}
	}

	if _, err := maint.ExecContext(ctx, `CREATE DATABASE `+quoteIdentifier(cfg.TemplateName())); err != nil {
		return fmt.Errorf("create template database %q: %w", cfg.TemplateName(), err)
	}
	if err := o.build(ctx, cfg.TemplateDSN()); err != nil {
		return fmt.Errorf("build template database %q: %w", cfg.TemplateName(), err)
	}
	return stampTemplate(ctx, maint, cfg.TemplateName(), want)
}

func templateState(ctx context.Context, maint *sql.DB, name string) (exists bool, comment string, err error) {
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

func stampTemplate(ctx context.Context, maint *sql.DB, name, comment string) error {
	stmt := fmt.Sprintf(`COMMENT ON DATABASE %s IS '%s'`, quoteIdentifier(name), comment)
	if _, err := maint.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("stamp template %q: %w", name, err)
	}
	return nil
}

func dropDatabase(ctx context.Context, maint *sql.DB, name string) error {
	_, err := maint.ExecContext(ctx, `DROP DATABASE IF EXISTS `+quoteIdentifier(name)+` WITH (FORCE)`)
	return err
}

// buildTemplate runs all migrations against the (empty) template database via
// `go run . migrate` in the backend module root — the same entrypoint CI and
// developers use. Role-creating migrations read PHOENIX_AUTH_PASSWORD from
// the environment, so callers must load .env first.
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
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go run . migrate: %w\n%s", err, out)
	}
	return nil
}

// migrationsComplete reports whether every migration file's version (the
// digit prefix bun records as the migration name) is applied in the database
// behind dsn.
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
		if len(name) > 8 && name[len(name)-8:] == "_test.go" {
			continue
		}
		if m := migrationFilePattern.FindStringSubmatch(name); m != nil {
			wanted = append(wanted, m[1])
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
		applied[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}

	for _, w := range wanted {
		if _, ok := applied[w]; !ok {
			return false, nil
		}
	}
	return true, nil
}
