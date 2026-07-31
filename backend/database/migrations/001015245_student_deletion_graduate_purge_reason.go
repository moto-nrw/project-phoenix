package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	studentDeletionGraduatePurgeReasonVersion     = "1.15.245"
	studentDeletionGraduatePurgeReasonDescription = "Audit permanent graduate purges with a dedicated deletion reason (#2068)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentDeletionGraduatePurgeReasonVersion,
		Description: studentDeletionGraduatePurgeReasonDescription,
		DependsOn:   []string{preserveStudentDeletionAuditVersion},
	})

	Migrations.MustRegister(studentDeletionGraduatePurgeReasonUp, studentDeletionGraduatePurgeReasonDown)
}

func studentDeletionGraduatePurgeReasonUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `
		ALTER TABLE audit.student_deletions
			DROP CONSTRAINT IF EXISTS student_deletions_reason_check,
			ADD CONSTRAINT student_deletions_reason_check
			CHECK (reason IN ('test_data', 'incorrect_entry', 'duplicate', 'privacy_request', 'graduate_purge'));
	`); err != nil {
		return fmt.Errorf("add graduate purge deletion reason: %w", err)
	}
	return nil
}

func studentDeletionGraduatePurgeReasonDown(ctx context.Context, db *bun.DB) error {
	var graduatePurgeRows int
	if err := db.NewRaw(`
		SELECT COUNT(*) FROM audit.student_deletions WHERE reason = 'graduate_purge'
	`).Scan(ctx, &graduatePurgeRows); err != nil {
		return fmt.Errorf("check graduate purge deletion audits: %w", err)
	}
	if graduatePurgeRows > 0 {
		return fmt.Errorf("cannot remove graduate_purge deletion reason while %d audit records exist", graduatePurgeRows)
	}
	if _, err := db.ExecContext(ctx, `
		ALTER TABLE audit.student_deletions
			DROP CONSTRAINT IF EXISTS student_deletions_reason_check,
			ADD CONSTRAINT student_deletions_reason_check
			CHECK (reason IN ('test_data', 'incorrect_entry', 'duplicate', 'privacy_request'));
	`); err != nil {
		return fmt.Errorf("remove graduate purge deletion reason: %w", err)
	}
	return nil
}
