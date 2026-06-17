package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/uptrace/bun"
)

const (
	parentAuthoredExceptionsVersion     = "1.15.136"
	parentAuthoredExceptionsDescription = "Allow guardian-authored pickup/arrival exceptions (source + created_by_guardian, FK SET NULL)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentAuthoredExceptionsVersion,
		Description: parentAuthoredExceptionsDescription,
		DependsOn: []string{
			createPickupSchedulesVersion,  // schedule.student_pickup_exceptions
			createArrivalSchedulesVersion, // schedule.student_arrival_exceptions
			"1.0.1",                       // auth.accounts (created_by_guardian FK)
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return parentAuthoredExceptionsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return parentAuthoredExceptionsDown(ctx, db)
		},
	)
}

// parentAuthoredExceptionsTables are the two date-specific exception tables a
// guardian may now write to from the parents portal. Both gain the same set of
// changes so a parent-authored row is first-class rather than masquerading as a
// staff edit.
var parentAuthoredExceptionsTables = []string{
	"schedule.student_pickup_exceptions",
	"schedule.student_arrival_exceptions",
}

func parentAuthoredExceptionsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.136: Adding source + created_by_guardian to schedule exception tables...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	for _, table := range parentAuthoredExceptionsTables {
		short := strings.TrimPrefix(table, "schedule.")

		// created_by stops being mandatory: a guardian-authored row has no
		// staff author, so it stores NULL there and references the account via
		// created_by_guardian instead.
		//
		// created_by_guardian is ON DELETE SET NULL, not CASCADE: deleting a
		// parent account must NOT erase the pickup/arrival exceptions they
		// authored — the child still needs collecting at the right time, and
		// staff rely on those future days. Account deletion only clears the
		// "authored by" link, leaving an orphaned-but-intact guardian row. The
		// constraint is named explicitly so the down migration can drop it
		// without depending on Postgres's auto-generated name.
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`
			ALTER TABLE %s
				ALTER COLUMN created_by DROP NOT NULL,
				ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'staff',
				ADD COLUMN IF NOT EXISTS created_by_guardian BIGINT,
				DROP CONSTRAINT IF EXISTS fk_%s_created_by_guardian,
				ADD CONSTRAINT fk_%s_created_by_guardian
					FOREIGN KEY (created_by_guardian) REFERENCES auth.accounts(id) ON DELETE SET NULL;
		`, table, short, short)); err != nil {
			return fmt.Errorf("failed altering %s for parent authoring: %w", table, err)
		}

		// source is constrained to the two known authors. chk_exception_author
		// requires created_by on a staff row, but tolerates a NULL
		// created_by_guardian on a guardian row: insert-time integrity is
		// enforced in the application (model Validate requires a guardian id on
		// write); the relaxed DB constraint only has to allow the
		// account-deletion-induced orphan the SET NULL FK can produce.
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`
			ALTER TABLE %s
				DROP CONSTRAINT IF EXISTS chk_exception_source,
				ADD CONSTRAINT chk_exception_source CHECK (source IN ('staff', 'guardian')),
				DROP CONSTRAINT IF EXISTS chk_exception_author,
				ADD CONSTRAINT chk_exception_author CHECK (
					(source = 'staff' AND created_by IS NOT NULL)
					OR source = 'guardian'
				);
		`, table)); err != nil {
			return fmt.Errorf("failed adding author constraints to %s: %w", table, err)
		}
	}

	return tx.Commit()
}

func parentAuthoredExceptionsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.136: Removing source + created_by_guardian from schedule exception tables...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	for _, table := range parentAuthoredExceptionsTables {
		short := strings.TrimPrefix(table, "schedule.")

		// Guardian-authored rows have a NULL created_by and cannot survive the
		// NOT NULL restore — drop them before reinstating the staff-only shape.
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE source = 'guardian';`, table)); err != nil {
			return fmt.Errorf("failed clearing guardian-authored rows from %s: %w", table, err)
		}

		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`
			ALTER TABLE %s
				DROP CONSTRAINT IF EXISTS chk_exception_author,
				DROP CONSTRAINT IF EXISTS chk_exception_source,
				DROP CONSTRAINT IF EXISTS fk_%s_created_by_guardian,
				DROP COLUMN IF EXISTS created_by_guardian,
				DROP COLUMN IF EXISTS source,
				ALTER COLUMN created_by SET NOT NULL;
		`, table, short)); err != nil {
			return fmt.Errorf("failed reverting %s parent authoring: %w", table, err)
		}
	}

	return tx.Commit()
}
