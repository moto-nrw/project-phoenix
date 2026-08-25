package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	redactDeletedWithdrawalCompletionVersion     = "1.15.329"
	redactDeletedWithdrawalCompletionDescription = "Retain redacted completion history when a withdrawn child is deleted (#2547)."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version: redactDeletedWithdrawalCompletionVersion, Description: redactDeletedWithdrawalCompletionDescription,
		DependsOn: []string{parentWithdrawalConfirmationVersion},
	})
	Migrations.MustRegister(redactDeletedWithdrawalCompletionUp, redactDeletedWithdrawalCompletionDown)
}

func redactDeletedWithdrawalCompletionUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.NewRaw(`
		ALTER TABLE users.care_withdrawal_completions
			DROP CONSTRAINT IF EXISTS chk_care_withdrawal_completion_terminal,
			DROP CONSTRAINT IF EXISTS care_withdrawal_completions_outcome_check,
			DROP CONSTRAINT IF EXISTS fk_care_withdrawal_completion_student;
		ALTER TABLE users.care_withdrawal_completions
			ADD CONSTRAINT care_withdrawal_completions_outcome_check CHECK (outcome IN ('care_ended', 'deleted')),
			ADD CONSTRAINT fk_care_withdrawal_completion_student FOREIGN KEY (tenant_id, student_id)
				REFERENCES users.students(tenant_id, id) ON DELETE SET NULL (student_id),
			ADD CONSTRAINT chk_care_withdrawal_completion_terminal CHECK (
				(state = 'pending' AND outcome IS NULL AND obsolete_reason IS NULL AND resolved_at IS NULL)
				OR (state = 'resolved' AND outcome IN ('care_ended', 'deleted') AND obsolete_reason IS NULL AND resolved_at IS NOT NULL)
				OR (state = 'obsolete' AND outcome IS NULL AND obsolete_reason IS NOT NULL AND resolved_at IS NOT NULL)
			);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("allow redacted deleted care-withdrawal completions: %w", err)
	}
	return nil
}

func redactDeletedWithdrawalCompletionDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.NewRaw(`
		DELETE FROM users.care_withdrawal_completions WHERE outcome = 'deleted';
		ALTER TABLE users.care_withdrawal_completions
			DROP CONSTRAINT IF EXISTS chk_care_withdrawal_completion_terminal,
			DROP CONSTRAINT IF EXISTS care_withdrawal_completions_outcome_check,
			DROP CONSTRAINT IF EXISTS fk_care_withdrawal_completion_student;
		ALTER TABLE users.care_withdrawal_completions
			ADD CONSTRAINT care_withdrawal_completions_outcome_check CHECK (outcome IN ('care_ended')),
			ADD CONSTRAINT fk_care_withdrawal_completion_student FOREIGN KEY (tenant_id, student_id)
				REFERENCES users.students(tenant_id, id) ON DELETE CASCADE,
			ADD CONSTRAINT chk_care_withdrawal_completion_terminal CHECK (
				(state = 'pending' AND outcome IS NULL AND obsolete_reason IS NULL AND resolved_at IS NULL)
				OR (state = 'resolved' AND outcome = 'care_ended' AND obsolete_reason IS NULL AND resolved_at IS NOT NULL)
				OR (state = 'obsolete' AND outcome IS NULL AND obsolete_reason IS NOT NULL AND resolved_at IS NOT NULL)
			);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("restore care-withdrawal completion constraints: %w", err)
	}
	return nil
}
