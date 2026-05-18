package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	requestChildrenRenewalColumnsVersion     = "1.15.73"
	requestChildrenRenewalColumnsDescription = "Add rollover_source_child_id, review_reason to enrollment.request_children, and extend the status CHECK with pending_renewal / auto_renewed / pending_admin_review to support the annual phase rollover flow."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     requestChildrenRenewalColumnsVersion,
		Description: requestChildrenRenewalColumnsDescription,
		DependsOn: []string{
			createEnrollmentRequestChildrenVersion,
			phaseRolloverColumnsVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.73: Adding renewal columns to enrollment.request_children...")

			// New columns first so the constraint rewrite below can see them.
			if _, err := db.NewRaw(`
				ALTER TABLE enrollment.request_children
				ADD COLUMN IF NOT EXISTS rollover_source_child_id BIGINT
					REFERENCES enrollment.request_children(id) ON DELETE SET NULL,
				ADD COLUMN IF NOT EXISTS review_reason TEXT;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding renewal columns: %w", err)
			}

			// Each source child can be rolled forward only once. Partial
			// unique index ignores the many NULL rows from non-rollover
			// submissions.
			if _, err := db.NewRaw(`
				CREATE UNIQUE INDEX IF NOT EXISTS uq_enrollment_request_children_rollover_source
				ON enrollment.request_children (rollover_source_child_id)
				WHERE rollover_source_child_id IS NOT NULL;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed creating rollover_source unique index: %w", err)
			}

			// Postgres lets us swap a CHECK by dropping + adding under the
			// same name. The pre-existing constraint name is the table's
			// auto-generated one from the original migration — find via
			// pg_catalog to be robust.
			if _, err := db.NewRaw(`
				DO $$
				DECLARE
					constraint_name TEXT;
				BEGIN
					SELECT conname INTO constraint_name
					FROM pg_constraint
					WHERE conrelid = 'enrollment.request_children'::regclass
					  AND contype = 'c'
					  AND pg_get_constraintdef(oid) ILIKE '%status%submitted%'
					LIMIT 1;
					IF constraint_name IS NOT NULL THEN
						EXECUTE format('ALTER TABLE enrollment.request_children DROP CONSTRAINT %I', constraint_name);
					END IF;
				END $$;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping legacy status CHECK: %w", err)
			}

			if _, err := db.NewRaw(`
				ALTER TABLE enrollment.request_children
				ADD CONSTRAINT enrollment_request_children_status_chk
				CHECK (status IN (
					'submitted','under_review','approved','waitlisted',
					'rejected','withdrawn',
					'pending_renewal','auto_renewed','pending_admin_review'
				));
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed installing extended status CHECK: %w", err)
			}

			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			// Restoring the narrow CHECK is only safe if no rows use the
			// new statuses; rollback in that situation requires manual
			// data cleanup, which is fine — rollbacks here are dev-only.
			if _, err := db.NewRaw(`
				ALTER TABLE enrollment.request_children
				DROP CONSTRAINT IF EXISTS enrollment_request_children_status_chk;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping extended status CHECK: %w", err)
			}
			if _, err := db.NewRaw(`
				ALTER TABLE enrollment.request_children
				ADD CONSTRAINT enrollment_request_children_status_chk
				CHECK (status IN (
					'submitted','under_review','approved','waitlisted',
					'rejected','withdrawn'
				));
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed reinstating narrow status CHECK: %w", err)
			}
			if _, err := db.NewRaw(`DROP INDEX IF EXISTS enrollment.uq_enrollment_request_children_rollover_source;`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping rollover_source unique index: %w", err)
			}
			if _, err := db.NewRaw(`
				ALTER TABLE enrollment.request_children
				DROP COLUMN IF EXISTS rollover_source_child_id,
				DROP COLUMN IF EXISTS review_reason;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping renewal columns: %w", err)
			}
			return nil
		},
	)
}
