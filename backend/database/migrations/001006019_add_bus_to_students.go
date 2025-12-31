package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	addBusToStudentsVersion     = "1.6.19"
	addBusToStudentsDescription = "Add bus column to users.students"
)

func init() {
	MigrationRegistry[addBusToStudentsVersion] = &Migration{
		Version:     addBusToStudentsVersion,
		Description: addBusToStudentsDescription,
		DependsOn: []string{
			UsersStudentsVersion, // Depends on students table (1.3.5)
		},
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addBusToStudentsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return addBusToStudentsDown(ctx, db)
		},
	)
}

func addBusToStudentsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.6.19: Adding bus column to users.students...")

	_, err := db.NewRaw(`
		ALTER TABLE users.students
		ADD COLUMN IF NOT EXISTS bus BOOLEAN DEFAULT FALSE;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding bus column to users.students: %w", err)
	}

	// Create index for filtering (useful for queries like "show all Buskinder")
	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_students_bus ON users.students(bus);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating index on bus: %w", err)
	}

	return nil
}

func addBusToStudentsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.6.19: Removing bus column from users.students...")

	// Drop index first
	_, err := db.NewRaw(`
		DROP INDEX IF EXISTS users.idx_students_bus;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping index on bus: %w", err)
	}

	// Drop column
	_, err = db.NewRaw(`
		ALTER TABLE users.students
		DROP COLUMN IF EXISTS bus;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping bus column from users.students: %w", err)
	}

	return nil
}
