package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	calendarAppointmentRevisionVersion     = "1.15.213"
	calendarAppointmentRevisionDescription = "Add revision counter to calendar.appointments for iCalendar SEQUENCE"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     calendarAppointmentRevisionVersion,
		Description: calendarAppointmentRevisionDescription,
		DependsOn:   []string{calendarTargetAllSchoolParentsVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.213: Adding revision to calendar.appointments...")
			if _, err := db.NewRaw(`
				ALTER TABLE calendar.appointments
					ADD COLUMN IF NOT EXISTS revision INTEGER NOT NULL DEFAULT 0;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding appointment revision: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.213: Dropping calendar.appointments.revision...")
			if _, err := db.NewRaw(`
				ALTER TABLE calendar.appointments DROP COLUMN IF EXISTS revision;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping appointment revision: %w", err)
			}
			return nil
		},
	)
}
