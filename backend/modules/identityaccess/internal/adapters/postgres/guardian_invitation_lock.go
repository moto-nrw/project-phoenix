package postgres

import (
	"context"
	"fmt"
)

// LockGuardianInvitationAccounts takes both accounts before the caller's
// membership and child locks. Locking only the invitee first would still let
// two parents inviting each other deadlock on their own account guards.
func (s *Store) LockGuardianInvitationAccounts(ctx context.Context, inviterID int64, email string) error {
	db, err := s.database(ctx)
	if err != nil {
		return err
	}
	var ids []int64
	err = db.NewRaw(`SELECT id FROM auth.accounts
		WHERE (id = ? OR LOWER(email) = ?) AND personal_data_erased_at IS NULL
		ORDER BY id FOR UPDATE`, inviterID, email).Scan(ctx, &ids)
	if err != nil {
		return fmt.Errorf("identity access postgres: lock invitation accounts: %w", err)
	}
	return nil
}
