package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	deleteWithdrawalCompletionWithStudentVersion     = "1.15.326"
	deleteWithdrawalCompletionWithStudentDescription = "Delete pending care-withdrawal tasks with their student."
)

func init() {
	MigrationRegistry.Register(&Migration{Version: deleteWithdrawalCompletionWithStudentVersion, Description: deleteWithdrawalCompletionWithStudentDescription, DependsOn: []string{careWithdrawalWeeklyPlanObsoleteVersion}})
	Migrations.MustRegister(deleteWithdrawalCompletionWithStudentUp, deleteWithdrawalCompletionWithStudentDown)
}

func deleteWithdrawalCompletionWithStudentUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.NewRaw(`ALTER TABLE users.care_withdrawal_completions DROP CONSTRAINT IF EXISTS fk_care_withdrawal_completion_student;
		ALTER TABLE users.care_withdrawal_completions ADD CONSTRAINT fk_care_withdrawal_completion_student FOREIGN KEY (tenant_id, student_id) REFERENCES users.students(tenant_id, id) ON DELETE CASCADE;`).Exec(ctx); err != nil {
		return fmt.Errorf("cascade care-withdrawal completion deletion: %w", err)
	}
	return nil
}

func deleteWithdrawalCompletionWithStudentDown(ctx context.Context, db *bun.DB) error {
	return deleteWithdrawalCompletionWithStudentUp(ctx, db)
}
