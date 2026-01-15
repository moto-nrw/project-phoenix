package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	addSicknessToStudentsVersion     = "1.7.2"
	addSicknessToStudentsDescription = "Add sick and sick_since columns to users.students"
)

func init() {
	MigrationRegistry[addSicknessToStudentsVersion] = &Migration{
		Version:     addSicknessToStudentsVersion,
		Description: addSicknessToStudentsDescription,
		DependsOn: []string{
			UsersStudentsVersion, // Depends on students table (1.3.5)
		},
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addSicknessToStudentsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return addSicknessToStudentsDown(ctx, db)
		},
	)
}

func addSicknessToStudentsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.7.2: Adding sick and sick_since columns to users.students...")

	// Add sick boolean column (defaults to false)
	_, err := db.NewRaw(`
		ALTER TABLE users.students
		ADD COLUMN IF NOT EXISTS sick BOOLEAN DEFAULT FALSE;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding sick column to users.students: %w", err)
	}

	// Add sick_since timestamp column (nullable - only set when sick)
	_, err = db.NewRaw(`
		ALTER TABLE users.students
		ADD COLUMN IF NOT EXISTS sick_since TIMESTAMP;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding sick_since column to users.students: %w", err)
	}

	// Create index for filtering sick students (useful for "show all sick students")
	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_students_sick ON users.students(sick) WHERE sick = true;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating index on sick: %w", err)
	}

	return nil
}

func addSicknessToStudentsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.7.2: Removing sick and sick_since columns from users.students...")

	// Drop index first
	_, err := db.NewRaw(`
		DROP INDEX IF EXISTS users.idx_students_sick;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping index on sick: %w", err)
	}

	// Drop sick_since column
	_, err = db.NewRaw(`
		ALTER TABLE users.students
		DROP COLUMN IF EXISTS sick_since;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping sick_since column from users.students: %w", err)
	}

	// Drop sick column
	_, err = db.NewRaw(`
		ALTER TABLE users.students
		DROP COLUMN IF EXISTS sick;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping sick column from users.students: %w", err)
	}

	return nil
}
