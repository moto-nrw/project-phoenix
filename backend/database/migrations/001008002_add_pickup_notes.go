package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	addPickupNotesVersion     = "1.8.2"
	addPickupNotesDescription = "Add student pickup notes table and make exception reason nullable"
)

var AddPickupNotesDependsOn = []string{
	createPickupSchedulesVersion, // Depends on pickup schedules/exceptions tables (1.8.1)
}

func init() {
	MigrationRegistry[addPickupNotesVersion] = &Migration{
		Version:     addPickupNotesVersion,
		Description: addPickupNotesDescription,
		DependsOn:   AddPickupNotesDependsOn,
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addPickupNotesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return addPickupNotesDown(ctx, db)
		},
	)
}

func addPickupNotesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.8.2: Adding student pickup notes table and making exception reason nullable...")

	// Step 1: Make reason nullable on exceptions table
	_, err := db.NewRaw(`
		ALTER TABLE schedule.student_pickup_exceptions ALTER COLUMN reason DROP NOT NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to make reason nullable: %w", err)
	}

	// Step 2: Set default empty string for reason
	_, err = db.NewRaw(`
		ALTER TABLE schedule.student_pickup_exceptions ALTER COLUMN reason SET DEFAULT '';
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to set reason default: %w", err)
	}

	// Step 3: Create student_pickup_notes table
	_, err = db.NewRaw(`
		CREATE TABLE IF NOT EXISTS schedule.student_pickup_notes (
			id BIGSERIAL PRIMARY KEY,
			student_id BIGINT NOT NULL REFERENCES users.students(id) ON DELETE CASCADE,
			note_date DATE NOT NULL,
			content VARCHAR(500) NOT NULL,
			created_by BIGINT NOT NULL REFERENCES users.staff(id),
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating student_pickup_notes table: %w", err)
	}

	// Step 4: Create composite index (student_id, note_date) — NOT unique, multiple notes per day allowed
	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_pickup_notes_student_date
		ON schedule.student_pickup_notes(student_id, note_date);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating composite index on student_pickup_notes: %w", err)
	}

	// Step 5: Create index on student_id for faster lookups
	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_pickup_notes_student_id
		ON schedule.student_pickup_notes(student_id);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating student_id index on student_pickup_notes: %w", err)
	}

	return nil
}

func addPickupNotesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.8.2: Dropping student pickup notes table and restoring reason NOT NULL...")

	// Drop notes table
	_, err := db.NewRaw(`
		DROP TABLE IF EXISTS schedule.student_pickup_notes CASCADE;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping student_pickup_notes table: %w", err)
	}

	// Fill empty reasons before restoring NOT NULL
	_, err = db.NewRaw(`
		UPDATE schedule.student_pickup_exceptions SET reason = 'Keine Angabe' WHERE reason = '' OR reason IS NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed filling empty reasons: %w", err)
	}

	// Restore NOT NULL on reason
	_, err = db.NewRaw(`
		ALTER TABLE schedule.student_pickup_exceptions ALTER COLUMN reason SET NOT NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed restoring reason NOT NULL: %w", err)
	}

	// Remove default
	_, err = db.NewRaw(`
		ALTER TABLE schedule.student_pickup_exceptions ALTER COLUMN reason DROP DEFAULT;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping reason default: %w", err)
	}

	return nil
}
