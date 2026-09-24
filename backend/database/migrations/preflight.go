package migrations

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
)

// MigratePreflight asks every pending migration that declares one whether its
// data precondition holds, and changes nothing.
//
// A migration that refuses on data a human has to correct used to announce that
// refusal from inside the deployment, after the application was already stopped
// and the release backup taken, so the only way out was a full restore. The same
// question answered here costs nothing: the deployment runs this against the
// live database while the old release is still serving, and a failure is an
// ordinary pre-migration abort that leaves the environment untouched.
//
// It runs no migration. It is not literally read-only: initialising the migrator
// creates bun's own bookkeeping tables when they are absent, which is what
// `migrate` itself would do moments later anyway.
//
// Only pending migrations are asked. An applied migration's precondition
// describes a state its own Up has already consumed, so re-asking it would
// report a failure about work that is finished.
func MigratePreflight(ctx context.Context, db *bun.DB) error {
	return migratePreflightTo(ctx, db, os.Stdout)
}

func migratePreflightTo(ctx context.Context, db *bun.DB, output io.Writer) error {
	if db == nil {
		return fmt.Errorf("migration database is required")
	}
	migrator := newMigrator(db, nil)
	if err := migrator.Init(ctx); err != nil {
		return fmt.Errorf("initialize migrations: %w", err)
	}
	status, err := migrator.MigrationsWithStatus(ctx)
	if err != nil {
		return fmt.Errorf("load migration status: %w", err)
	}
	ctx, err = studentContractRunContext(ctx, db)
	if err != nil {
		return err
	}
	pending := status.Unapplied()
	return reportPreflightChecks(withPendingMigrations(ctx, pending), output, pendingPreconditions(pending), db)
}

type pendingMigrationsKey struct{}

// withPendingMigrations tells every precondition which migrations run before it
// in the same release. A precondition asks the database as it is now, so a state
// an earlier pending migration's Up creates is not there yet; the precondition
// has to know that Up is still coming rather than report its absence.
func withPendingMigrations(ctx context.Context, pending migrate.MigrationSlice) context.Context {
	versions := make(map[string]bool, len(pending))
	for index := range pending {
		version, _ := migrationIdentity(&pending[index])
		versions[version] = true
	}
	return context.WithValue(ctx, pendingMigrationsKey{}, versions)
}

// migrationPending reports whether version is pending in the preflight that
// called the precondition. Outside a preflight nothing is known to be pending.
func migrationPending(ctx context.Context, version string) bool {
	versions, _ := ctx.Value(pendingMigrationsKey{}).(map[string]bool)
	return versions[version]
}

// reportPreflightChecks runs every selected precondition, prints one line per
// migration and joins the failures. Every check runs even after one fails, so a
// single deployment attempt reports all the data that needs correcting rather
// than surfacing it one release at a time.
func reportPreflightChecks(ctx context.Context, output io.Writer, checks []preconditionCheck, db *bun.DB) error {
	if len(checks) == 0 {
		_, err := fmt.Fprintln(output, "migration preflight: no pending migration declares a data precondition")
		return err
	}
	var failures []error
	for _, check := range checks {
		err := check.migration.Precondition(ctx, db)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s (%s): %w", check.version, check.migration.Description, err))
		}
		verdict := "OK"
		if err != nil {
			verdict = "FAILED"
		}
		if _, writeErr := fmt.Fprintf(output, "migration preflight %s %s - %s\n",
			verdict, check.version, check.migration.Description); writeErr != nil {
			return writeErr
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("migration preflight failed for %d pending migration(s): %w",
			len(failures), errors.Join(failures...))
	}
	_, err := fmt.Fprintf(output, "migration preflight passed: %d pending migration(s) checked\n", len(checks))
	return err
}

type preconditionCheck struct {
	version   string
	migration *Migration
}

// pendingPreconditions keeps the migrator's order, so the checks are reported in
// the order the migrations would run. It takes the unapplied slice rather than
// filtering again: an applied migration's precondition describes a state its own
// Up has already consumed.
func pendingPreconditions(pending migrate.MigrationSlice) []preconditionCheck {
	var checks []preconditionCheck
	for index := range pending {
		entry := &pending[index]
		registered, ok := MigrationByBunName(entry.Name)
		if !ok || registered.Precondition == nil {
			continue
		}
		version, _ := migrationIdentity(entry)
		checks = append(checks, preconditionCheck{version: version, migration: registered})
	}
	return checks
}
