package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	appointmentReminderPushDeliveryAccessVersion     = "1.15.248"
	appointmentReminderPushDeliveryAccessDescription = "Restrict appointment reminder push delivery access"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     appointmentReminderPushDeliveryAccessVersion,
		Description: appointmentReminderPushDeliveryAccessDescription,
		DependsOn:   []string{appointmentReminderPushRevisionsVersion},
	})
	Migrations.MustRegister(appointmentReminderPushDeliveryAccessUp, appointmentReminderPushDeliveryAccessDown)
}

func appointmentReminderPushDeliveryAccessUp(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		REVOKE ALL ON TABLE calendar.appointment_reminder_push_deliveries FROM phoenix_tenant;
		REVOKE USAGE ON SEQUENCE calendar.appointment_reminder_push_deliveries_id_seq FROM phoenix_tenant;
		DROP POLICY IF EXISTS tenant_isolation_calendar_appointment_reminder_push_deliveries
			ON calendar.appointment_reminder_push_deliveries;

		CREATE FUNCTION calendar.claim_appointment_reminder_push_delivery(
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
			INSERT INTO calendar.appointment_reminder_push_deliveries
				(tenant_id, appointment_id, appointment_revision, occurrence_date, guardian_profile_id)
			VALUES
				(current_tenant_id, p_appointment_id, p_appointment_revision, p_occurrence_date, p_guardian_profile_id)
			ON CONFLICT (tenant_id, appointment_id, appointment_revision, occurrence_date, guardian_profile_id) DO NOTHING;
			RETURN FOUND;
		END;
		$$;

		CREATE FUNCTION calendar.release_appointment_reminder_push_delivery(
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

		REVOKE ALL ON FUNCTION calendar.claim_appointment_reminder_push_delivery(BIGINT, INTEGER, DATE, BIGINT) FROM PUBLIC;
		REVOKE ALL ON FUNCTION calendar.release_appointment_reminder_push_delivery(BIGINT, INTEGER, DATE, BIGINT) FROM PUBLIC;
		GRANT EXECUTE ON FUNCTION calendar.claim_appointment_reminder_push_delivery(BIGINT, INTEGER, DATE, BIGINT) TO phoenix_tenant;
		GRANT EXECUTE ON FUNCTION calendar.release_appointment_reminder_push_delivery(BIGINT, INTEGER, DATE, BIGINT) TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("restrict appointment reminder push delivery access: %w", err)
	}
	return nil
}

func appointmentReminderPushDeliveryAccessDown(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		DROP FUNCTION IF EXISTS calendar.release_appointment_reminder_push_delivery(BIGINT, INTEGER, DATE, BIGINT);
		DROP FUNCTION IF EXISTS calendar.claim_appointment_reminder_push_delivery(BIGINT, INTEGER, DATE, BIGINT);

		CREATE POLICY tenant_isolation_calendar_appointment_reminder_push_deliveries
			ON calendar.appointment_reminder_push_deliveries FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
		GRANT SELECT, INSERT, UPDATE, DELETE ON calendar.appointment_reminder_push_deliveries TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE calendar.appointment_reminder_push_deliveries_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("restore appointment reminder push delivery access: %w", err)
	}
	return nil
}
