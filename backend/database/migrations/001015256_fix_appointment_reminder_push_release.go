package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	fixAppointmentReminderPushReleaseVersion     = "1.15.256"
	fixAppointmentReminderPushReleaseDescription = "Fix appointment reminder push release function"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     fixAppointmentReminderPushReleaseVersion,
		Description: fixAppointmentReminderPushReleaseDescription,
		DependsOn:   []string{appointmentReminderPushDeliveryAccessVersion},
	})
	Migrations.MustRegister(fixAppointmentReminderPushReleaseUp, fixAppointmentReminderPushReleaseDown)
}

func fixAppointmentReminderPushReleaseUp(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE OR REPLACE FUNCTION calendar.release_appointment_reminder_push_delivery(
			p_appointment_id BIGINT,
			p_appointment_revision INTEGER,
			p_occurrence_date DATE,
			p_guardian_profile_id BIGINT
		) RETURNS BOOLEAN
		LANGUAGE plpgsql
		SECURITY DEFINER
		SET search_path = pg_catalog, calendar
		AS $$
		DECLARE
			current_tenant_id BIGINT := NULLIF(current_setting('app.current_tenant_id', true), '')::bigint;
		BEGIN
			IF current_tenant_id IS NULL THEN
				RAISE EXCEPTION 'app.current_tenant_id is required';
			END IF;
			DELETE FROM calendar.appointment_reminder_push_deliveries AS delivery
			WHERE delivery.tenant_id = current_tenant_id
				AND delivery.appointment_id = p_appointment_id
				AND delivery.appointment_revision = p_appointment_revision
				AND delivery.occurrence_date = p_occurrence_date
				AND delivery.guardian_profile_id = p_guardian_profile_id;
			RETURN FOUND;
		END;
		$$;
	`)
	if err != nil {
		return fmt.Errorf("fix appointment reminder push release function: %w", err)
	}
	return nil
}

func fixAppointmentReminderPushReleaseDown(context.Context, *bun.DB) error {
	return nil
}
