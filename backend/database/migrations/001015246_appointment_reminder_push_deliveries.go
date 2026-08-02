package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	appointmentReminderPushDeliveriesVersion     = "1.15.246"
	appointmentReminderPushDeliveriesDescription = "Deduplicate appointment reminder push deliveries (#1671)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     appointmentReminderPushDeliveriesVersion,
		Description: appointmentReminderPushDeliveriesDescription,
		DependsOn:   []string{studentDeletionGraduatePurgeReasonVersion},
	})
	Migrations.MustRegister(appointmentReminderPushDeliveriesUp, appointmentReminderPushDeliveriesDown)
}

func appointmentReminderPushDeliveriesUp(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE calendar.appointment_reminder_push_deliveries (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			appointment_id BIGINT NOT NULL,
			occurrence_date DATE NOT NULL,
			guardian_profile_id BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (tenant_id, appointment_id, occurrence_date, guardian_profile_id),
			FOREIGN KEY (tenant_id, appointment_id)
				REFERENCES calendar.appointments(tenant_id, id) ON DELETE CASCADE,
			FOREIGN KEY (tenant_id, guardian_profile_id)
				REFERENCES users.guardian_profiles(tenant_id, id) ON DELETE CASCADE
		);
	`)
	if err != nil {
		return fmt.Errorf("create appointment reminder push deliveries: %w", err)
	}
	return nil
}

func appointmentReminderPushDeliveriesDown(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS calendar.appointment_reminder_push_deliveries`)
	if err != nil {
		return fmt.Errorf("drop appointment reminder push deliveries: %w", err)
	}
	return nil
}
