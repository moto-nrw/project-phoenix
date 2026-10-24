package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// LockPasswordLoginState requires the caller's administrative transaction.
func (s *Store) LockPasswordLoginState(ctx context.Context, id int64, operator bool) (domain.PasswordLoginState, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.PasswordLoginState{}, err
	}
	var state domain.PasswordLoginState
	if operator {
		err = db.NewRaw(`SELECT password_attempts, password_locked_until FROM platform.operators WHERE id = ? FOR UPDATE`, id).Scan(ctx, &state)
	} else {
		err = db.NewRaw(`SELECT password_attempts, password_locked_until FROM auth.accounts WHERE id = ? AND personal_data_erased_at IS NULL FOR UPDATE`, id).Scan(ctx, &state)
	}
	return state, err
}

// The caller holds this identity's row lock for the entire password check.
func (s *Store) SavePasswordLoginState(ctx context.Context, id int64, operator bool, state domain.PasswordLoginState) error {
	db, err := s.database(ctx)
	if err != nil {
		return err
	}
	if operator {
		_, err = db.NewRaw(`UPDATE platform.operators SET password_attempts = ?, password_locked_until = ? WHERE id = ?`, state.PasswordAttempts, state.PasswordLockedUntil, id).Exec(ctx)
	} else {
		_, err = db.NewRaw(`UPDATE auth.accounts SET password_attempts = ?, password_locked_until = ? WHERE id = ? AND personal_data_erased_at IS NULL`, state.PasswordAttempts, state.PasswordLockedUntil, id).Exec(ctx)
	}
	return err
}
