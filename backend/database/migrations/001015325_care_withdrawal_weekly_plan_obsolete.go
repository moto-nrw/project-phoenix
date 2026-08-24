package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	careWithdrawalWeeklyPlanObsoleteVersion     = "1.15.325"
	careWithdrawalWeeklyPlanObsoleteDescription = "Allow obsolete care-withdrawal completions after switching to weekly plans."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     careWithdrawalWeeklyPlanObsoleteVersion,
		Description: careWithdrawalWeeklyPlanObsoleteDescription,
		DependsOn:   []string{careWithdrawalCompletionsVersion},
	})
	Migrations.MustRegister(careWithdrawalWeeklyPlanObsoleteUp, careWithdrawalWeeklyPlanObsoleteDown)
}

func careWithdrawalWeeklyPlanObsoleteUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.NewRaw(`
		ALTER TABLE users.care_withdrawal_completions
			DROP CONSTRAINT IF EXISTS care_withdrawal_completions_obsolete_reason_check;
		ALTER TABLE users.care_withdrawal_completions
			ADD CONSTRAINT care_withdrawal_completions_obsolete_reason_check
			CHECK (obsolete_reason IN ('rebooked_without_gap', 'weekly_plan_mode'));
	`).Exec(ctx); err != nil {
		return fmt.Errorf("allow weekly-plan obsolete care withdrawals: %w", err)
	}
	return nil
}

func careWithdrawalWeeklyPlanObsoleteDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.NewRaw(`
		UPDATE users.care_withdrawal_completions
		SET obsolete_reason = 'rebooked_without_gap'
		WHERE obsolete_reason = 'weekly_plan_mode';
		ALTER TABLE users.care_withdrawal_completions
			DROP CONSTRAINT IF EXISTS care_withdrawal_completions_obsolete_reason_check;
		ALTER TABLE users.care_withdrawal_completions
			ADD CONSTRAINT care_withdrawal_completions_obsolete_reason_check
			CHECK (obsolete_reason IN ('rebooked_without_gap'));
	`).Exec(ctx); err != nil {
		return fmt.Errorf("restore care withdrawal obsolete reasons: %w", err)
	}
	return nil
}
