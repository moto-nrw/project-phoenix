package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	addPickupStatusToStudentsVersion     = "1.6.18"
	addPickupStatusToStudentsDescription = "Add pickup_status column to users.students"
)

func init() {
	MigrationRegistry[addPickupStatusToStudentsVersion] = &Migration{
		Version:     addPickupStatusToStudentsVersion,
		Description: addPickupStatusToStudentsDescription,
		DependsOn: []string{
			UsersStudentsVersion, // Depends on students table (1.3.5)
		},
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addPickupStatusToStudentsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return addPickupStatusToStudentsDown(ctx, db)
		},
	)
}

func addPickupStatusToStudentsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.6.18: Adding pickup_status column to users.students...")

	_, err := db.NewRaw(`
		ALTER TABLE users.students
		ADD COLUMN IF NOT EXISTS pickup_status TEXT;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding pickup_status column to users.students: %w", err)
	}

	// Create index for filtering/searching
	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_students_pickup_status ON users.students(pickup_status);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating index on pickup_status: %w", err)
	}

	return nil
}

func addPickupStatusToStudentsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.6.18: Removing pickup_status column from users.students...")

	// Drop index first
	_, err := db.NewRaw(`
		DROP INDEX IF EXISTS users.idx_students_pickup_status;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping index on pickup_status: %w", err)
	}

	// Drop column
	_, err = db.NewRaw(`
		ALTER TABLE users.students
		DROP COLUMN IF EXISTS pickup_status;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping pickup_status column from users.students: %w", err)
	}

	return nil
}
