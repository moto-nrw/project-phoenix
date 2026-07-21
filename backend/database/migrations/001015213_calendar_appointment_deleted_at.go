package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	calendarAppointmentDeletedAtVersion     = "1.15.213"
	calendarAppointmentDeletedAtDescription = "Add deleted_at soft-delete to calendar.appointments for feed-only cancellation tombstones"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     calendarAppointmentDeletedAtVersion,
		Description: calendarAppointmentDeletedAtDescription,
		DependsOn:   []string{calendarAppointmentRevisionVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.213: Adding deleted_at to calendar.appointments...")
			if _, err := db.NewRaw(`
				ALTER TABLE calendar.appointments
					ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding appointment deleted_at: %w", err)
			}
			// Partial index so the feed's durable-tombstone scan (deleted_at IS NOT
			// NULL AND deleted_at >= cutoff) stays cheap without weighing on the
			// hot path of live (deleted_at IS NULL) appointments.
			if _, err := db.NewRaw(`
				CREATE INDEX IF NOT EXISTS idx_calendar_appointments_deleted_at
					ON calendar.appointments (deleted_at)
					WHERE deleted_at IS NOT NULL;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed indexing appointment deleted_at: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.213: Dropping calendar.appointments.deleted_at...")
			if _, err := db.NewRaw(`
				DROP INDEX IF EXISTS calendar.idx_calendar_appointments_deleted_at;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping appointment deleted_at index: %w", err)
			}
			if _, err := db.NewRaw(`
				ALTER TABLE calendar.appointments DROP COLUMN IF EXISTS deleted_at;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping appointment deleted_at: %w", err)
			}
			return nil
		},
	)
}
