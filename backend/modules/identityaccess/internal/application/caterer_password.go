package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// catererPasswordChangeReason names the revocation a caterer's own password
// change records, as the operator's does.
const catererPasswordChangeReason = "password_change"

// ChangeCatererPassword replaces a signed-in caterer's password (#4496).
// The current password is checked like a login: a wrong one counts towards
// the account's lock, and a locked account is refused. The new password,
// the spent reset links and the end of every session of the account commit
// together, as after a password reset.
func (c *CatererAccess) ChangeCatererPassword(ctx context.Context, accountID int64, currentPassword, newPassword, ipAddress, userAgent string) error {
	return catererError(c.changePassword(c.attach(ctx), accountID, currentPassword, newPassword, ipAddress, userAgent))
}

func (c *CatererAccess) changePassword(ctx context.Context, accountID int64, currentPassword, newPassword, ipAddress, userAgent string) error {
	if err := c.passwords.ValidatePasswordStrength(newPassword); err != nil {
		return failed("change caterer password", err)
	}
	hash, err := c.passwords.HashPassword(newPassword)
	if err != nil {
		return failed("hash password", err)
	}
	var refusal *catererPasswordRefusal
	// The global identity read the login checks its password in, joined
	// when the caller holds a transaction. The refused attempt must commit
	// its count, so a wrong password ends the transaction without an error.
	err = globalIdentityRead(ctx, c.runtime, func(txCtx context.Context) error {
		var txErr error
		refusal, txErr = c.changePasswordInTx(txCtx, accountID, currentPassword, hash, ipAddress, userAgent)
		return txErr
	})
	if err != nil {
		return failed("change caterer password", err)
	}
	if refusal != nil {
		if refusal.schoolID > 0 {
			c.auth.logFailedLogin(c.runtime.WithTenantID(ctx, refusal.schoolID), accountID, ipAddress, userAgent, "Caterer password change refused")
		}
		return failed("change caterer password", identityaccess.ErrInvalidCredentials)
	}
	return nil
}

// catererPasswordRefusal is a refused current password and the school the
// caterer's audit event is filed at; a caterer without a school has none.
type catererPasswordRefusal struct{ schoolID int64 }

// changePasswordInTx checks the current password under the account's
// login lock and, when it is accepted, writes the new hash and revokes the
// account's reset links and sessions.
func (c *CatererAccess) changePasswordInTx(ctx context.Context, accountID int64, currentPassword, hash, ipAddress, userAgent string) (*catererPasswordRefusal, error) {
	account, found, _, err := c.auth.store.FindLoginAccount(ctx, accountID, true)
	if err != nil {
		return nil, err
	}
	if !found || !account.Active {
		return nil, domain.ErrAccountNotFound
	}
	caterer, isCaterer, _, err := c.store.FindCatererOfAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if !isCaterer {
		return nil, domain.ErrAccountNotCaterer
	}
	accepted, err := checkPasswordLogin(ctx, c.auth.store, c.verifier, c.auth.passwordPolicy, account.ID, account.PasswordHash, currentPassword, false)
	if err != nil {
		return nil, err
	}
	if !accepted {
		schools, _, err := c.store.ListCatererSchools(ctx, caterer.ID)
		if err != nil {
			return nil, err
		}
		refusal := &catererPasswordRefusal{}
		if len(schools) > 0 {
			refusal.schoolID = schools[0]
		}
		return refusal, nil
	}
	updated, _, err := c.store.SetAccountPassword(ctx, accountID, hash)
	if err != nil {
		return nil, err
	}
	if !updated {
		return nil, domain.ErrAccountNotFound
	}
	if _, err := c.store.RevokePasswordResetTokens(ctx, accountID); err != nil {
		return nil, err
	}
	return nil, c.auth.revokeAccountWideInContext(ctx, accountID, catererPasswordChangeReason, ipAddress, userAgent)
}
