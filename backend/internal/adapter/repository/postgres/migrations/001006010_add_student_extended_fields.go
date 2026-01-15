package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/uptrace/bun"
)

const (
	AddStudentExtendedFieldsVersion     = "1.6.10"
	AddStudentExtendedFieldsDescription = "Add supervisor notes and health info fields to students"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[AddStudentExtendedFieldsVersion] = &Migration{
		Version:     AddStudentExtendedFieldsVersion,
		Description: AddStudentExtendedFieldsDescription,
		DependsOn:   []string{"1.3.5"}, // Depends on students table
	}

	// Migration 1.6.10: Add extended fields to students
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addStudentExtendedFieldsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return addStudentExtendedFieldsDown(ctx, db)
		},
	)
}

// addStudentExtendedFieldsUp adds notes fields to students
func addStudentExtendedFieldsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.6.10: Adding extended fields to students...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logrus.Warnf("Error rolling back transaction: %v", err)
		}
	}()

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

	// Commit the transaction
	return tx.Commit()
}

// addStudentExtendedFieldsDown removes the added fields
func addStudentExtendedFieldsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.6.10: Removing extended fields from students...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logrus.Warnf("Error rolling back transaction: %v", err)
		}
	}()

	// Remove fields from students table
	_, err = tx.ExecContext(ctx, `
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
