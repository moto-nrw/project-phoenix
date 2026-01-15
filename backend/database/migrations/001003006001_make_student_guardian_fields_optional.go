package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/uptrace/bun"
)

const (
	MakeStudentGuardianFieldsOptionalVersion     = "1.3.6.1"
	MakeStudentGuardianFieldsOptionalDescription = "Make guardian_name and guardian_contact optional in students table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[MakeStudentGuardianFieldsOptionalVersion] = &Migration{
		Version:     MakeStudentGuardianFieldsOptionalVersion,
		Description: MakeStudentGuardianFieldsOptionalDescription,
		DependsOn:   []string{"1.3.6"}, // Depends on students_guardians junction table
	}

	// Migration 1.3.6.1: Make guardian fields optional
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return makeStudentGuardianFieldsOptionalUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return makeStudentGuardianFieldsOptionalDown(ctx, db)
		},
	)
}

// makeStudentGuardianFieldsOptionalUp makes guardian_name and guardian_contact nullable
func makeStudentGuardianFieldsOptionalUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.3.6.1: Making guardian_name and guardian_contact optional in students table...")

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

	// Make guardian_name nullable
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.students
		ALTER COLUMN guardian_name DROP NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("error making guardian_name nullable: %w", err)
	}

	// Make guardian_contact nullable
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.students
		ALTER COLUMN guardian_contact DROP NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("error making guardian_contact nullable: %w", err)
	}

	fmt.Println("Successfully made guardian_name and guardian_contact optional")

	// Commit the transaction
	return tx.Commit()
}

// makeStudentGuardianFieldsOptionalDown makes guardian_name and guardian_contact required again
func makeStudentGuardianFieldsOptionalDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.3.6.1: Making guardian_name and guardian_contact required again...")

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

	// First, set empty values to a default to avoid constraint violations
	_, err = tx.ExecContext(ctx, `
		UPDATE users.students
		SET guardian_name = 'Not Specified'
		WHERE guardian_name IS NULL;
	`)
	if err != nil {
		return fmt.Errorf("error setting default guardian_name: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE users.students
		SET guardian_contact = 'Not Specified'
		WHERE guardian_contact IS NULL;
	`)
	if err != nil {
		return fmt.Errorf("error setting default guardian_contact: %w", err)
	}

	// Make guardian_name required
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.students
		ALTER COLUMN guardian_name SET NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("error making guardian_name required: %w", err)
	}

	// Make guardian_contact required
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.students
		ALTER COLUMN guardian_contact SET NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("error making guardian_contact required: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
