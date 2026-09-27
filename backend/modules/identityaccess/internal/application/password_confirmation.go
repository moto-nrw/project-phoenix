package application

import "context"

// ConfirmAccountPassword re-checks the password of an account that is
// already signed in, without touching its sessions. A guardian uses it to
// confirm a binding Erklärung (#3430) when the school asks for it. The
// failure is the login's: ErrInvalidCredentials, or ErrAccountInactive.
func (s *AccountAuthentication) ConfirmAccountPassword(ctx context.Context, accountID int64, password string) error {
	account, err := s.authenticatedAccount(ctx, "confirm password", accountID)
	if err != nil {
		return err
	}
	return s.verifyPassword(account, password)
}
