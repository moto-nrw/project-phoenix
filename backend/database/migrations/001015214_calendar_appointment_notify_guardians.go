package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	calendarAppointmentNotifyGuardiansVersion     = "1.15.214"
	calendarAppointmentNotifyGuardiansDescription = "Persist the guardian-notification opt-in (send_email) on calendar.appointments so cancellation honours it"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     calendarAppointmentNotifyGuardiansVersion,
		Description: calendarAppointmentNotifyGuardiansDescription,
		DependsOn:   []string{calendarAppointmentDeletedAtVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.214: Adding notify_guardians to calendar.appointments...")
			if _, err := db.NewRaw(`
				ALTER TABLE calendar.appointments
					ADD COLUMN IF NOT EXISTS notify_guardians BOOLEAN NOT NULL DEFAULT TRUE;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding appointment notify_guardians: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.214: Dropping calendar.appointments.notify_guardians...")
			if _, err := db.NewRaw(`
				ALTER TABLE calendar.appointments DROP COLUMN IF EXISTS notify_guardians;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping appointment notify_guardians: %w", err)
			}
			return nil
		},
	)
}
