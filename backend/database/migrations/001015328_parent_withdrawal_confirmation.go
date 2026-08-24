package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	parentWithdrawalConfirmationVersion     = "1.15.328"
	parentWithdrawalConfirmationDescription = "Persist guardian confirmation for complete-withdrawal requests (#2547)."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentWithdrawalConfirmationVersion,
		Description: parentWithdrawalConfirmationDescription,
		DependsOn:   []string{careWithdrawalBookingExpiryVersion},
	})
	Migrations.MustRegister(parentWithdrawalConfirmationUp, parentWithdrawalConfirmationDown)
}

func parentWithdrawalConfirmationUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.NewRaw(`
		ALTER TABLE enrollment.offering_change_requests
			ADD COLUMN complete_withdrawal_confirmed BOOLEAN NOT NULL DEFAULT FALSE,
			ADD COLUMN approved_complete_withdrawal BOOLEAN NOT NULL DEFAULT FALSE,
			ADD COLUMN withdrawal_confirmed_by BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
			ADD COLUMN withdrawal_confirmed_at TIMESTAMPTZ,
			ADD CONSTRAINT chk_offering_change_withdrawal_confirmation CHECK (
				(complete_withdrawal_confirmed AND withdrawal_confirmed_at IS NOT NULL)
				OR (NOT complete_withdrawal_confirmed AND withdrawal_confirmed_by IS NULL AND withdrawal_confirmed_at IS NULL)
			);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("add parent complete-withdrawal confirmation: %w", err)
	}
	return nil
}

func parentWithdrawalConfirmationDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.NewRaw(`
		ALTER TABLE enrollment.offering_change_requests
			DROP CONSTRAINT chk_offering_change_withdrawal_confirmation,
			DROP COLUMN withdrawal_confirmed_at,
			DROP COLUMN withdrawal_confirmed_by,
			DROP COLUMN approved_complete_withdrawal,
			DROP COLUMN complete_withdrawal_confirmed;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("drop parent complete-withdrawal confirmation: %w", err)
	}
	return nil
}
