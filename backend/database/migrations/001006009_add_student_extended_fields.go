package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	AddStudentExtendedFieldsVersion     = "1.6.9"
	AddStudentExtendedFieldsDescription = "Add birthday, supervisor notes, and health info fields to students"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[AddStudentExtendedFieldsVersion] = &Migration{
		Version:     AddStudentExtendedFieldsVersion,
		Description: AddStudentExtendedFieldsDescription,
		DependsOn:   []string{"1.3.5", "1.2.1"}, // Depends on students and persons tables
	}

	// Migration 1.6.9: Add extended fields to students
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addStudentExtendedFieldsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return addStudentExtendedFieldsDown(ctx, db)
		},
	)
}

// addStudentExtendedFieldsUp adds birthday to persons and notes fields to students
func addStudentExtendedFieldsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.6.9: Adding extended fields to students and persons...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Add birthday field to persons table (since all students are persons)
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.persons
		ADD COLUMN IF NOT EXISTS birthday DATE;

		COMMENT ON COLUMN users.persons.birthday IS 'Date of birth of the person';
	`)
	if err != nil {
		return fmt.Errorf("error adding birthday to persons table: %w", err)
	}

	// Add supervisor_notes field to students table (editable by supervisors)
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.students
		ADD COLUMN IF NOT EXISTS supervisor_notes TEXT;

		COMMENT ON COLUMN users.students.supervisor_notes IS 'Notes about the student that can be edited by supervisors (Betreuernotizen)';
	`)
	if err != nil {
		return fmt.Errorf("error adding supervisor_notes to students table: %w", err)
	}

	// Add health_info field to students table (static health/medical information)
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.students
		ADD COLUMN IF NOT EXISTS health_info TEXT;

		COMMENT ON COLUMN users.students.health_info IS 'Static health and medical information about the student (Gesundheitsinfos)';
	`)
	if err != nil {
		return fmt.Errorf("error adding health_info to students table: %w", err)
	}

	// Create index on birthday for potential age-based queries
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_persons_birthday ON users.persons(birthday);
	`)
	if err != nil {
		return fmt.Errorf("error creating birthday index: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// addStudentExtendedFieldsDown removes the added fields
func addStudentExtendedFieldsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.6.9: Removing extended fields from students and persons...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Drop the index first
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS users.idx_persons_birthday;
	`)
	if err != nil {
		return fmt.Errorf("error dropping birthday index: %w", err)
	}

	// Remove fields from tables
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.persons
		DROP COLUMN IF EXISTS birthday;

		ALTER TABLE users.students
		DROP COLUMN IF EXISTS supervisor_notes;

		ALTER TABLE users.students
		DROP COLUMN IF EXISTS health_info;
	`)
	if err != nil {
		return fmt.Errorf("error dropping columns: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}