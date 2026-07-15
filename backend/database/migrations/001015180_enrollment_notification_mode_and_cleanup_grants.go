package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	enrollmentNotificationModeAndCleanupGrantsVersion     = "1.15.180"
	enrollmentNotificationModeAndCleanupGrantsDescription = "Pin enrollment decision notification modes and grant tenant cleanup deletes"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     enrollmentNotificationModeAndCleanupGrantsVersion,
		Description: enrollmentNotificationModeAndCleanupGrantsDescription,
		DependsOn:   []string{changeRequestCareOfferingCapabilityVersion},
	})
	Migrations.MustRegister(enrollmentNotificationModeAndCleanupGrantsUp, enrollmentNotificationModeAndCleanupGrantsDown)
}

func enrollmentNotificationModeAndCleanupGrantsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.180: Pinning enrollment notification modes and granting tenant cleanup deletes...")
	if _, err := db.NewRaw(`
		ALTER TABLE enrollment.requests
			ADD COLUMN IF NOT EXISTS decision_notification_mode TEXT;

		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1
				FROM pg_constraint
				WHERE conname = 'chk_enrollment_requests_decision_notification_mode'
				  AND conrelid = 'enrollment.requests'::regclass
			) THEN
				ALTER TABLE enrollment.requests
					ADD CONSTRAINT chk_enrollment_requests_decision_notification_mode
					CHECK (decision_notification_mode IN ('digest', 'immediate'));
			END IF;
		END $$;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed adding enrollment decision notification mode: %w", err)
	}

	if err := backfillEnrollmentDecisionNotificationModes(ctx, db, "TRUE"); err != nil {
		return err
	}

	if _, err := db.NewRaw(`
		GRANT DELETE ON platform.email_outbox TO phoenix_tenant;
		GRANT DELETE ON enrollment.late_invites TO phoenix_tenant;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed granting enrollment cleanup permissions: %w", err)
	}
	return nil
}

// backfillEnrollmentDecisionNotificationModes accepts only a trusted predicate
// supplied by this package. Production passes TRUE; tests scope it to their
// hermetic tenant fixtures so they never mutate unrelated requests.
func backfillEnrollmentDecisionNotificationModes(ctx context.Context, db *bun.DB, tenantPredicate string, args ...any) error {
	query := fmt.Sprintf(`
		WITH requests_with_decisions AS (
			SELECT r.id, r.tenant_id
			FROM enrollment.requests AS r
			WHERE r.decision_notification_mode IS NULL
			  AND %s
			  AND (
				EXISTS (
					SELECT 1
					FROM enrollment.request_children AS rc
					WHERE rc.request_id = r.id
					  AND rc.tenant_id = r.tenant_id
					  AND rc.status IN ('approved', 'rejected', 'waitlisted')
				)
				OR EXISTS (
					SELECT 1
					FROM platform.email_outbox AS prior_outbox
					WHERE prior_outbox.tenant_id = r.tenant_id
					  AND prior_outbox.related_entity_type = 'enrollment_request'
					  AND prior_outbox.related_entity_id = r.id
					  AND prior_outbox.kind IN (
						'enrollment_decision_digest',
						'enrollment_approved',
						'enrollment_rejected',
						'enrollment_waitlisted'
					  )
				)
			  )
		), inferred_modes AS (
			SELECT decided.id,
				COALESCE(
					(
						SELECT CASE
							WHEN outbox.kind = 'enrollment_decision_digest' THEN 'digest'
							ELSE 'immediate'
						END
						FROM platform.email_outbox AS outbox
						WHERE outbox.tenant_id = decided.tenant_id
						  AND outbox.related_entity_type = 'enrollment_request'
						  AND outbox.related_entity_id = decided.id
						  AND outbox.kind IN (
							'enrollment_decision_digest',
							'enrollment_approved',
							'enrollment_rejected',
							'enrollment_waitlisted'
						  )
						ORDER BY outbox.created_at, outbox.id
						LIMIT 1
					),
					-- An unfinished digest intentionally has no outbox row yet. Using
					-- the live tenant setting here would reinterpret its earlier
					-- sibling decisions after a mode change and lose notification
					-- history. Immediate decisions, by contrast, leave a qualifying
					-- outbox row transactionally, so no history is conservatively
					-- treated as digest.
					'digest'
				) AS mode
			FROM requests_with_decisions AS decided
		)
		UPDATE enrollment.requests AS request
		SET decision_notification_mode = inferred.mode
		FROM inferred_modes AS inferred
		WHERE request.id = inferred.id;
	`, tenantPredicate)
	if _, err := db.NewRaw(query, args...).Exec(ctx); err != nil {
		return fmt.Errorf("failed backfilling enrollment decision notification modes: %w", err)
	}
	return nil
}

func enrollmentNotificationModeAndCleanupGrantsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.180: Removing enrollment notification mode pins and tenant cleanup deletes...")
	// enrollment.late_invites inherited DELETE from the enrollment schema's
	// default privileges before this migration. Its explicit Up grant is a
	// repair/no-op and must not be revoked or rollback would narrow prior access.
	if _, err := db.NewRaw(`
		REVOKE DELETE ON platform.email_outbox FROM phoenix_tenant;

		ALTER TABLE enrollment.requests
			DROP CONSTRAINT IF EXISTS chk_enrollment_requests_decision_notification_mode,
			DROP COLUMN IF EXISTS decision_notification_mode;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed removing enrollment notification modes and cleanup permissions: %w", err)
	}
	return nil
}
