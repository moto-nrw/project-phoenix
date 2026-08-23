package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	careWithdrawalCompletionsVersion     = "1.15.324"
	careWithdrawalCompletionsDescription = "Persist and complete authoritative full-care withdrawals (#2546)."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     careWithdrawalCompletionsVersion,
		Description: careWithdrawalCompletionsDescription,
		DependsOn:   []string{studentCareExitRemovalsVersion},
	})
	Migrations.MustRegister(careWithdrawalCompletionsUp, careWithdrawalCompletionsDown)
}

func careWithdrawalCompletionsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.324: Creating care-withdrawal completions...")
	if _, err := db.NewRaw(`
		ALTER TABLE audit.enrollment_offering_adjustments
			ADD COLUMN IF NOT EXISTS complete_withdrawal_confirmed BOOLEAN NOT NULL DEFAULT FALSE;

		CREATE TABLE IF NOT EXISTS users.care_withdrawal_completions (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			student_id BIGINT,
			first_bookingless_day DATE NOT NULL,
			trigger TEXT NOT NULL CHECK (trigger IN ('direct_school')),
			source_adjustment_id BIGINT REFERENCES audit.enrollment_offering_adjustments(id) ON DELETE SET NULL,
			withdrawal_confirmed_by BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
			withdrawal_confirmed_role TEXT NOT NULL,
			withdrawal_confirmed_at TIMESTAMPTZ NOT NULL,
			source_offerings JSONB NOT NULL DEFAULT '[]'::jsonb,
			state TEXT NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'resolved', 'obsolete')),
			outcome TEXT CHECK (outcome IN ('care_ended')),
			obsolete_reason TEXT CHECK (obsolete_reason IN ('rebooked_without_gap')),
			resolved_by BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
			resolved_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_care_withdrawal_completion_tenant_id UNIQUE (tenant_id, id),
			CONSTRAINT fk_care_withdrawal_completion_student
				FOREIGN KEY (tenant_id, student_id)
				REFERENCES users.students(tenant_id, id) ON DELETE SET NULL (student_id),
			CONSTRAINT chk_care_withdrawal_completion_terminal CHECK (
				(state = 'pending' AND outcome IS NULL AND obsolete_reason IS NULL AND resolved_at IS NULL)
				OR (state = 'resolved' AND outcome = 'care_ended' AND obsolete_reason IS NULL AND resolved_at IS NOT NULL)
				OR (state = 'obsolete' AND outcome IS NULL AND obsolete_reason IS NOT NULL AND resolved_at IS NOT NULL)
			)
		);
		CREATE UNIQUE INDEX IF NOT EXISTS uq_care_withdrawal_completion_pending_student
			ON users.care_withdrawal_completions (tenant_id, student_id)
			WHERE state = 'pending' AND student_id IS NOT NULL;
		CREATE INDEX IF NOT EXISTS idx_care_withdrawal_completion_pending_order
			ON users.care_withdrawal_completions (tenant_id, first_bookingless_day, id)
			WHERE state = 'pending';

		ALTER TABLE users.student_care_exits
			ADD COLUMN IF NOT EXISTS withdrawal_completion_id BIGINT;
		ALTER TABLE users.student_care_exits
			ADD CONSTRAINT fk_student_care_exit_withdrawal_completion
			FOREIGN KEY (tenant_id, withdrawal_completion_id)
			REFERENCES users.care_withdrawal_completions(tenant_id, id)
			ON DELETE SET NULL (withdrawal_completion_id);

		CREATE TABLE IF NOT EXISTS users.student_care_exit_source_removals (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			student_id BIGINT NOT NULL,
			kind TEXT NOT NULL CHECK (kind IN (
				'source_booking', 'pickup_schedule', 'pickup_exception',
				'arrival_schedule', 'arrival_exception'
			)),
			source_row_id BIGINT NOT NULL,
			was_deleted BOOLEAN NOT NULL,
			snapshot JSONB NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_care_exit_source_removal UNIQUE (tenant_id, student_id, kind, source_row_id),
			CONSTRAINT fk_care_exit_source_removal_student
				FOREIGN KEY (tenant_id, student_id)
				REFERENCES users.students(tenant_id, id) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS idx_care_exit_source_removals_student
			ON users.student_care_exit_source_removals (tenant_id, student_id);

		GRANT SELECT, INSERT, UPDATE, DELETE ON users.care_withdrawal_completions TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE users.care_withdrawal_completions_id_seq TO phoenix_tenant;
		GRANT SELECT, INSERT, UPDATE, DELETE ON users.student_care_exit_source_removals TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE users.student_care_exit_source_removals_id_seq TO phoenix_tenant;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating care-withdrawal completion storage: %w", err)
	}
	if err := provisionTenantRLS(ctx, db, "users.care_withdrawal_completions"); err != nil {
		return err
	}
	return provisionTenantRLS(ctx, db, "users.student_care_exit_source_removals")
}

func careWithdrawalCompletionsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back 1.15.324: Dropping care-withdrawal completions...")
	if _, err := db.NewRaw(`
		ALTER TABLE users.student_care_exits
			DROP CONSTRAINT IF EXISTS fk_student_care_exit_withdrawal_completion,
			DROP COLUMN IF EXISTS withdrawal_completion_id;
		DROP TABLE IF EXISTS users.student_care_exit_source_removals;
		DROP TABLE IF EXISTS users.care_withdrawal_completions;
		ALTER TABLE audit.enrollment_offering_adjustments
			DROP COLUMN IF EXISTS complete_withdrawal_confirmed;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping care-withdrawal completion storage: %w", err)
	}
	return nil
}
