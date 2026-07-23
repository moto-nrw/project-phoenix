package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	calendarAppointmentNotifyGuardiansVersion     = "1.15.215"
	calendarAppointmentNotifyGuardiansDescription = "Persist the guardian-notification opt-in (send_email) on calendar.appointments so cancellation honours it; existing rows default opted-out"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     calendarAppointmentNotifyGuardiansVersion,
		Description: calendarAppointmentNotifyGuardiansDescription,
		DependsOn:   []string{calendarAppointmentDeletedAtVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.215: Adding notify_guardians to calendar.appointments...")
			// DEFAULT FALSE so existing appointments are backfilled OPTED OUT: the
			// send_email preference was never persisted before this column, so there
			// is no evidence a legacy appointment was meant to mail guardians. Left
			// TRUE, cancelling any pre-existing guardian appointment after deploy
			// would send unsolicited "appointment_cancelled" e-mails. New rows are
			// written with their explicit value by bun (the model tag omits default:),
			// so this default only governs the backfill and any non-bun insert —
			// matching the UI, where the box is opt-in.
			if _, err := db.NewRaw(`
				ALTER TABLE calendar.appointments
					ADD COLUMN IF NOT EXISTS notify_guardians BOOLEAN NOT NULL DEFAULT FALSE;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding appointment notify_guardians: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.215: Dropping calendar.appointments.notify_guardians...")
			if _, err := db.NewRaw(`
				ALTER TABLE calendar.appointments DROP COLUMN IF EXISTS notify_guardians;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping appointment notify_guardians: %w", err)
			}
			return nil
		},
	)
}
