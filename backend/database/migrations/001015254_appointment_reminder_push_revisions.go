package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	appointmentReminderPushRevisionsVersion     = "1.15.254"
	appointmentReminderPushRevisionsDescription = "Deduplicate appointment reminder pushes by revision"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     appointmentReminderPushRevisionsVersion,
		Description: appointmentReminderPushRevisionsDescription,
		DependsOn:   []string{appointmentReminderPushDeliveriesVersion},
	})
	Migrations.MustRegister(appointmentReminderPushRevisionsUp, appointmentReminderPushRevisionsDown)
}

func appointmentReminderPushRevisionsUp(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		ALTER TABLE calendar.appointment_reminder_push_deliveries
			ADD COLUMN appointment_revision INTEGER NOT NULL DEFAULT 0;
		DO $$
		DECLARE
			constraint_name TEXT;
		BEGIN
			SELECT conname INTO constraint_name
			FROM pg_constraint
			WHERE conrelid = 'calendar.appointment_reminder_push_deliveries'::regclass
			  AND contype = 'u'
			  AND pg_get_constraintdef(oid) = 'UNIQUE (tenant_id, appointment_id, occurrence_date, guardian_profile_id)';
			IF constraint_name IS NULL THEN
				RAISE EXCEPTION 'appointment reminder push delivery uniqueness constraint not found';
			END IF;
			EXECUTE format('ALTER TABLE calendar.appointment_reminder_push_deliveries DROP CONSTRAINT %I', constraint_name);
		END $$;
		ALTER TABLE calendar.appointment_reminder_push_deliveries
			ADD CONSTRAINT appointment_reminder_push_deliveries_revision_key
			UNIQUE (tenant_id, appointment_id, appointment_revision, occurrence_date, guardian_profile_id);
	`)
	if err != nil {
		return fmt.Errorf("add revision to appointment reminder push deliveries: %w", err)
	}
	return nil
}

// appointmentReminderPushRevisionsDown restores the revision-blind uniqueness.
// Rows recorded for several revisions of the same occurrence are legal under
// the current constraint and collide under the old one, so the rollback would
// fail on any database that has queued a reminder for an edited appointment.
// Fold each group down to its newest claim first: an earlier revision's claim
// is spent and carries nothing the reverted schema could express.
func appointmentReminderPushRevisionsDown(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		DELETE FROM calendar.appointment_reminder_push_deliveries AS stale
		USING calendar.appointment_reminder_push_deliveries AS keep
		WHERE stale.tenant_id = keep.tenant_id
			AND stale.appointment_id = keep.appointment_id
			AND stale.occurrence_date = keep.occurrence_date
			AND stale.guardian_profile_id = keep.guardian_profile_id
			AND (stale.appointment_revision, stale.id) < (keep.appointment_revision, keep.id);
		ALTER TABLE calendar.appointment_reminder_push_deliveries
			DROP CONSTRAINT appointment_reminder_push_deliveries_revision_key;
		ALTER TABLE calendar.appointment_reminder_push_deliveries
			DROP COLUMN appointment_revision;
		ALTER TABLE calendar.appointment_reminder_push_deliveries
			ADD CONSTRAINT appointment_reminder_push_deliveries_tenant_id_appointment_id_occurrence_date_guardian_profile_id_key
			UNIQUE (tenant_id, appointment_id, occurrence_date, guardian_profile_id);
	`)
	if err != nil {
		return fmt.Errorf("remove revision from appointment reminder push deliveries: %w", err)
	}
	return nil
}
