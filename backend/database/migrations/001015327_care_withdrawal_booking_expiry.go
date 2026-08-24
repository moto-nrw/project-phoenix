package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	careWithdrawalBookingExpiryVersion     = "1.15.327"
	careWithdrawalBookingExpiryDescription = "Record completion tasks when final care bookings expire."
)

func init() {
	MigrationRegistry.Register(&Migration{Version: careWithdrawalBookingExpiryVersion, Description: careWithdrawalBookingExpiryDescription, DependsOn: []string{deleteWithdrawalCompletionWithStudentVersion}})
	Migrations.MustRegister(careWithdrawalBookingExpiryUp, careWithdrawalBookingExpiryDown)
}

func careWithdrawalBookingExpiryUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.NewRaw(`ALTER TABLE users.care_withdrawal_completions
		DROP CONSTRAINT IF EXISTS care_withdrawal_completions_trigger_check;
		ALTER TABLE users.care_withdrawal_completions
		ADD CONSTRAINT care_withdrawal_completions_trigger_check
		CHECK (trigger IN ('direct_school', 'booking_expired'));`).Exec(ctx); err != nil {
		return fmt.Errorf("allow booking-expiry withdrawal completions: %w", err)
	}
	return nil
}

func careWithdrawalBookingExpiryDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.NewRaw(`DELETE FROM users.care_withdrawal_completions WHERE trigger = 'booking_expired';
		ALTER TABLE users.care_withdrawal_completions DROP CONSTRAINT IF EXISTS care_withdrawal_completions_trigger_check;
		ALTER TABLE users.care_withdrawal_completions ADD CONSTRAINT care_withdrawal_completions_trigger_check CHECK (trigger IN ('direct_school'));`).Exec(ctx); err != nil {
		return fmt.Errorf("remove booking-expiry withdrawal completions: %w", err)
	}
	return nil
}
