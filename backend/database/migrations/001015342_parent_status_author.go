package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const parentStatusAuthorVersion = "1.15.342"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentStatusAuthorVersion,
		Description: "Record the guardian author of parent status days (#2267)",
		DependsOn:   []string{familyProtectionVersion, studentStatusDaysNoteVersion, AuthAccountsVersion},
	})
	Migrations.MustRegister(parentStatusAuthorUp, parentStatusAuthorDown)
}

func parentStatusAuthorUp(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		ALTER TABLE active.student_status_days
			ADD COLUMN guardian_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL;
		CREATE INDEX idx_student_status_days_guardian_account
			ON active.student_status_days (tenant_id, guardian_account_id)
			WHERE guardian_account_id IS NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("add parent status author: %w", err)
	}
	return nil
}

func parentStatusAuthorDown(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		DROP INDEX IF EXISTS active.idx_student_status_days_guardian_account;
		ALTER TABLE active.student_status_days DROP COLUMN IF EXISTS guardian_account_id;
	`)
	if err != nil {
		return fmt.Errorf("drop parent status author: %w", err)
	}
	return nil
}
