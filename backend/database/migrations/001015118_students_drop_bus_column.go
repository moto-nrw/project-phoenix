package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	studentsDropBusColumnVersion     = "1.15.118"
	studentsDropBusColumnDescription = "Drop legacy users.students.bus column; bus_days is the single source of truth"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentsDropBusColumnVersion,
		Description: studentsDropBusColumnDescription,
		DependsOn: []string{
			studentsBusDaysVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return studentsDropBusColumnUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return studentsDropBusColumnDown(ctx, db)
		},
	)
}

func studentsDropBusColumnUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.118: Dropping legacy users.students.bus column...")

	// Defensive backfill before the drop: guarantee no Buskind information is
	// lost if any row still has the legacy bus flag set but never received the
	// weekday backfill (bus_days became the SSOT in #1582; this should be a
	// no-op for all rows touched since 1.15.112).
	if _, err := db.NewRaw(`
		UPDATE users.students
		SET bus_days = '{"mon":true,"tue":true,"wed":true,"thu":true,"fri":true}'::jsonb
		WHERE COALESCE(bus, FALSE) = TRUE
			AND NOT (bus_days ?| array['mon', 'tue', 'wed', 'thu', 'fri']);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed backfilling users.students.bus_days before drop: %w", err)
	}

	// Dropping the column also drops the dependent idx_students_bus index; the
	// explicit DROP INDEX keeps the intent clear and is a no-op if it is gone.
	if _, err := db.NewRaw(`DROP INDEX IF EXISTS users.idx_students_bus;`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping users.idx_students_bus: %w", err)
	}

	if _, err := db.NewRaw(`
		ALTER TABLE users.students
			DROP COLUMN IF EXISTS bus;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping users.students.bus column: %w", err)
	}

	return nil
}

func studentsDropBusColumnDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.118: Re-adding users.students.bus column...")

	if _, err := db.NewRaw(`
		ALTER TABLE users.students
			ADD COLUMN IF NOT EXISTS bus BOOLEAN DEFAULT FALSE;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed re-adding users.students.bus column: %w", err)
	}

	// Rebuild the legacy flag from the bus_days source of truth.
	if _, err := db.NewRaw(`
		UPDATE users.students
		SET bus = (bus_days ?| array['mon', 'tue', 'wed', 'thu', 'fri']);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed restoring users.students.bus from bus_days: %w", err)
	}

	if _, err := db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_students_bus ON users.students(bus);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed recreating idx_students_bus: %w", err)
	}

	return nil
}
