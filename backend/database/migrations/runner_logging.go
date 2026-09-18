package migrations

import (
	"context"
	"log/slog"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
)

// migrationLogComponent labels every line the migration runner writes, so a Loki
// query can select migration output without matching on message text.
const migrationLogComponent = "migrations"

// runnerLog turns bun's per-migration hooks into one structured line per
// migration (#3300).
//
// Before this the runner itself said nothing about individual migrations and
// every migration printed its own progress, in four styles across stdout and
// stderr, so the log could not be relied on to say which migration was running
// or how long it took.
//
// Migrator.Migrate runs migrations one at a time and calls the hooks inline, so
// the in-flight field is only ever touched from the goroutine running the
// migration and needs no synchronization.
type runnerLog struct {
	logger   *slog.Logger
	inFlight *inFlightMigration
}

// inFlightMigration is what the before hook records so the after hook — or, when
// the migration fails, the runner — can complete the line.
type inFlightMigration struct {
	version     string
	description string
	started     time.Time
}

// newRunnerLog scopes logger to the migration runner and returns the hook set.
func newRunnerLog(logger *slog.Logger) *runnerLog {
	return &runnerLog{logger: logger.With("component", migrationLogComponent)}
}

// migratorOptions wires the hooks into a migrator.
func (r *runnerLog) migratorOptions() []migrate.MigratorOption {
	return []migrate.MigratorOption{
		migrate.BeforeMigration(r.before),
		migrate.AfterMigration(r.after),
	}
}

func (r *runnerLog) before(_ context.Context, _ bun.IConn, migration *migrate.Migration) error {
	version, description := migrationIdentity(migration)
	r.inFlight = &inFlightMigration{
		version:     version,
		description: description,
		started:     time.Now(),
	}
	return nil
}

func (r *runnerLog) after(ctx context.Context, _ bun.IConn, _ *migrate.Migration) error {
	if r.inFlight == nil {
		return nil
	}
	r.logger.InfoContext(ctx, "migration applied",
		"version", r.inFlight.version,
		"description", r.inFlight.description,
		"duration_ms", time.Since(r.inFlight.started).Milliseconds(),
	)
	r.inFlight = nil
	return nil
}

// logFailure reports the migration that was still running when the migrator
// returned an error. bun skips the after hook on failure, so without this the
// run's last migration line would be missing entirely and the error would name
// the migration only by its bun name, inside a wrapped message.
func (r *runnerLog) logFailure(ctx context.Context, err error) {
	if r.inFlight == nil {
		return
	}
	r.logger.ErrorContext(ctx, "migration failed",
		"version", r.inFlight.version,
		"description", r.inFlight.description,
		"duration_ms", time.Since(r.inFlight.started).Milliseconds(),
		"error", err,
	)
	r.inFlight = nil
}

// migrationIdentity resolves what bun knows about a migration — a
// filename-derived name and comment — to the version and description its
// registry entry carries.
//
// The three migrations that never registered fall back to the bun name, which is
// also what bun_migrations and `migrate status` show for them, rather than
// logging an empty version.
func migrationIdentity(migration *migrate.Migration) (version, description string) {
	if registered, ok := MigrationByBunName(migration.Name); ok {
		return registered.Version, registered.Description
	}
	return migration.Name, migration.Comment
}

// pendingVersionRange names the ends of an ascending run of pending migrations,
// so one line can say what the run covers instead of listing every step. Both
// values are the same version for a single pending migration, and empty for an
// empty run.
func pendingVersionRange(pending migrate.MigrationSlice) (first, last string) {
	if len(pending) == 0 {
		return "", ""
	}
	first, _ = migrationIdentity(&pending[0])
	last, _ = migrationIdentity(&pending[len(pending)-1])
	return first, last
}

// migrationLogger falls back to the process logger when none was injected, the
// nil-safe pattern the backend uses for structs tests construct bare.
func migrationLogger(logger *slog.Logger) *slog.Logger {
	if logger == nil {
		return slog.Default()
	}
	return logger
}
