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
	return reportPreflightChecks(ctx, output, pendingPreconditions(status), db)
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
// the order the migrations would run.
func pendingPreconditions(status migrate.MigrationSlice) []preconditionCheck {
	var checks []preconditionCheck
	for index := range status {
		entry := &status[index]
		if entry.MigratedAt.Unix() > 0 {
			continue
		}
		registered, ok := MigrationByBunName(entry.Name)
		if !ok || registered.Precondition == nil {
			continue
		}
		version, _ := migrationIdentity(entry)
		checks = append(checks, preconditionCheck{version: version, migration: registered})
	}
	return checks
}
