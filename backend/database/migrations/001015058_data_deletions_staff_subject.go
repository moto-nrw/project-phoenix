package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	dataDeletionsStaffSubjectVersion     = "1.15.58"
	dataDeletionsStaffSubjectDescription = "Allow staff-scoped deletions in audit.data_deletions (subject is student OR staff)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     dataDeletionsStaffSubjectVersion,
		Description: dataDeletionsStaffSubjectDescription,
		// 1.3.7 created audit.data_deletions; users.staff comes from 1.1.x
		// (already in 1.10.x dependency chain).
		DependsOn: []string{"1.3.7", "1.1.5"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addDataDeletionsStaffSubject(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return removeDataDeletionsStaffSubject(ctx, db)
		},
	)
}

// addDataDeletionsStaffSubject extends audit.data_deletions so the same table
// can record both student-scoped deletions (visit_retention, timetable
// cleanup) AND staff-scoped deletions (time-tracking retention). The
// student_id column becomes nullable, a new staff_id column joins users.staff,
// and a check constraint guarantees exactly one of the two is set per row.
//
// Existing rows are untouched — they all carry a non-null student_id, which
// the check constraint accepts as the student-subject path. New consumers
// (active.TimeTrackingCleanupService) populate staff_id only.
func addDataDeletionsStaffSubject(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.58: extending audit.data_deletions for staff subjects...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		ALTER TABLE audit.data_deletions
			ALTER COLUMN student_id DROP NOT NULL;

		ALTER TABLE audit.data_deletions
			ADD COLUMN IF NOT EXISTS staff_id BIGINT;

		ALTER TABLE audit.data_deletions
			ADD CONSTRAINT fk_data_deletions_staff
				FOREIGN KEY (staff_id)
				REFERENCES users.staff(id)
				ON DELETE CASCADE;

		-- Exactly one of student_id / staff_id must be set. Both-null or
		-- both-set rows would lose audit semantics — a deletion has one
		-- subject, never two.
		ALTER TABLE audit.data_deletions
			ADD CONSTRAINT chk_data_deletions_subject_xor
				CHECK ((student_id IS NOT NULL) <> (staff_id IS NOT NULL));

		CREATE INDEX IF NOT EXISTS idx_data_deletions_staff_id
			ON audit.data_deletions(staff_id);
	`)
	if err != nil {
		return fmt.Errorf("error extending audit.data_deletions: %w", err)
	}

	fmt.Println("Migration 1.15.58: audit.data_deletions extended for staff subjects")
	return tx.Commit()
}

func removeDataDeletionsStaffSubject(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.58: audit.data_deletions staff subject...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Rollback is unsafe if any staff-only rows exist — they would violate
	// the re-added NOT NULL on student_id. We refuse to roll back rather
	// than silently corrupt audit history.
	var staffOnlyCount int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM audit.data_deletions WHERE staff_id IS NOT NULL
	`).Scan(&staffOnlyCount); err != nil {
		return fmt.Errorf("error checking staff-scoped rows: %w", err)
	}
	if staffOnlyCount > 0 {
		return fmt.Errorf("cannot roll back: %d staff-scoped deletion rows exist; restore manually or drop them first", staffOnlyCount)
	}

	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS audit.idx_data_deletions_staff_id;

		ALTER TABLE audit.data_deletions
			DROP CONSTRAINT IF EXISTS chk_data_deletions_subject_xor;

		ALTER TABLE audit.data_deletions
			DROP CONSTRAINT IF EXISTS fk_data_deletions_staff;

		ALTER TABLE audit.data_deletions
			DROP COLUMN IF EXISTS staff_id;

		ALTER TABLE audit.data_deletions
			ALTER COLUMN student_id SET NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("error reverting audit.data_deletions: %w", err)
	}

	fmt.Println("Migration 1.15.58: rolled back")
	return tx.Commit()
}
