package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// Setup link of a parent account at a Träger (ADR 0216 points 5 and 6). The
// token is signed, stored nowhere and names the address and the Träger of
// the holiday registration it was mailed for. Whoever holds the link holds
// the mailbox, so the setup trusts its address like a guardian invitation's.

const opHolidayParentSetup = "holiday parent setup"

// IssueHolidayParentSetupToken signs the setup link; it reads nothing. The
// link names the normalized address, so the setup finds the account the
// sign-in finds.
func (l *AccountLifecycle) IssueHolidayParentSetupToken(email string, organizationID int64) (string, error) {
	if l == nil || l.auth == nil {
		return "", errAccountLifecycleUnavailable
	}
	normalized, err := identityaccess.NormalizeInvitationEmail(email)
	if err != nil {
		return "", failed(opHolidayParentSetup, err)
	}
	if organizationID <= 0 {
		return "", failed(opHolidayParentSetup, identityaccess.ErrHolidayParentSetupInvalid)
	}
	return l.signHolidayParentSetup(identityaccess.HolidayParentSetupClaims{Email: normalized, OrganizationID: organizationID})
}

// HasTraegerParentAccess reports whether the account holds parents-portal
// access at an active Träger.
func (l *AccountLifecycle) HasTraegerParentAccess(ctx context.Context, accountID int64) (bool, error) {
	if l == nil || l.auth == nil {
		return false, errAccountLifecycleUnavailable
	}
	if accountID <= 0 {
		return false, nil
	}
	organizations, err := liveTraegerParentOrganizations(bindRuntime(l.auth.attach, ctx), l.auth, accountID)
	if err != nil {
		return false, failed(opHolidayParentSetup, err)
	}
	return len(organizations) > 0, nil
}

// signHolidayParentSetup signs the link for its fixed lifetime.
func (l *AccountLifecycle) signHolidayParentSetup(claims identityaccess.HolidayParentSetupClaims) (string, error) {
	return l.auth.codec.IssueHolidayParentSetupToken(claims, identityaccess.HolidayParentSetupTTL)
}

// PreviewHolidayParentSetup names the address, the Träger and whether an
// account for the address exists.
func (l *AccountLifecycle) PreviewHolidayParentSetup(ctx context.Context, token string) (_ identityaccess.HolidayParentSetupPreview, err error) {
	if l == nil || l.auth == nil {
		return identityaccess.HolidayParentSetupPreview{}, errAccountLifecycleUnavailable
	}
	defer func() { err = invitationError(err) }()
	ctx = bindRuntime(l.auth.attach, ctx)
	claims, err := l.holidayParentSetupClaims(token)
	if err != nil {
		return identityaccess.HolidayParentSetupPreview{}, err
	}
	preview := identityaccess.HolidayParentSetupPreview{Email: claims.Email}
	err = l.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		traeger, live, err := l.setupTraeger(txCtx, claims.OrganizationID, false)
		if err != nil || !live {
			return err
		}
		preview.TraegerName = traeger.Name
		_, preview.AccountExists, _, err = l.logins.FindLoginAccountByEmail(txCtx, claims.Email)
		if err != nil {
			return failed(opHolidayParentSetup, err)
		}
		return nil
	})
	return preview, err
}

// CompleteHolidayParentSetup creates the account when the address has none
// and records its access at the Träger, in one administrative transaction.
// The address lock serializes it with guardian provisioning; the account row
// lock is taken before the Träger's, the order of the parent sign-in.
func (l *AccountLifecycle) CompleteHolidayParentSetup(ctx context.Context, token string, registration identityaccess.GuardianRegistration) (accountExisted bool, err error) {
	if l == nil || l.auth == nil {
		return false, errAccountLifecycleUnavailable
	}
	defer func() { err = invitationError(err) }()
	ctx = bindRuntime(l.auth.attach, ctx)
	claims, err := l.holidayParentSetupClaims(token)
	if err != nil {
		return false, err
	}
	var accountID int64
	err = l.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		account, existed, err := l.provisionHolidayParentAccount(txCtx, claims.Email, registration)
		if err != nil {
			return err
		}
		if _, live, err := l.setupTraeger(txCtx, claims.OrganizationID, true); err != nil || !live {
			return err
		}
		if _, err := l.logins.GrantTraegerParentAccess(txCtx, account.ID, claims.OrganizationID); err != nil {
			return failed(opHolidayParentSetup, err)
		}
		accountExisted, accountID = existed, account.ID
		return nil
	})
	if err != nil {
		return false, err
	}
	l.logger.Info("holiday parent account set up",
		slog.Int64("account_id", accountID),
		slog.Int64("organization_id", claims.OrganizationID),
		slog.Bool("account_existed", accountExisted))
	return accountExisted, nil
}

// provisionHolidayParentAccount returns the address's account with its
// credential untouched, or creates one with the registration's password: no
// school, no profile, no role (ADR 0216 point 6).
func (l *AccountLifecycle) provisionHolidayParentAccount(ctx context.Context, email string, registration identityaccess.GuardianRegistration) (domain.LoginAccount, bool, error) {
	if err := l.invitations.LockGuardianAccountEmail(ctx, email); err != nil {
		return domain.LoginAccount{}, false, failed(opHolidayParentSetup, err)
	}
	account, found, _, err := l.logins.LockLoginAccountByEmail(ctx, email)
	if err != nil {
		return domain.LoginAccount{}, false, failed(opHolidayParentSetup, err)
	}
	if found {
		if !account.Active {
			return domain.LoginAccount{}, false, failed(opHolidayParentSetup, identityaccess.ErrAccountInactive)
		}
		return account, true, refuseCatererAccount(ctx, l.logins, account.ID, opHolidayParentSetup)
	}
	if registration.Password != registration.ConfirmPassword {
		return domain.LoginAccount{}, false, failed(opHolidayParentSetup, domain.ErrPasswordMismatch)
	}
	if err := l.passwords.ValidatePasswordStrength(registration.Password); err != nil {
		return domain.LoginAccount{}, false, failed(opHolidayParentSetup, err)
	}
	hash, err := l.passwords.HashPassword(registration.Password)
	if err != nil {
		return domain.LoginAccount{}, false, failed(opHolidayParentSetup, err)
	}
	created, _, err := l.invitations.InsertAccount(ctx, email, hash)
	if err != nil {
		return domain.LoginAccount{}, false, failed(opHolidayParentSetup, err)
	}
	return created, false, nil
}

// holidayParentSetupClaims reads the link; every unreadable, expired or
// foreign token is the same invalid link.
func (l *AccountLifecycle) holidayParentSetupClaims(token string) (identityaccess.HolidayParentSetupClaims, error) {
	claims, err := l.auth.codec.ParseHolidayParentSetupToken(token)
	if err != nil {
		return identityaccess.HolidayParentSetupClaims{}, failed(opHolidayParentSetup, fmt.Errorf("%w: %w", identityaccess.ErrHolidayParentSetupInvalid, err))
	}
	return claims, nil
}

// setupTraeger reads the Träger of the link; a Träger that is gone or
// inactive makes the link invalid. lock pins it until the setup commits.
func (l *AccountLifecycle) setupTraeger(ctx context.Context, organizationID int64, lock bool) (identityaccess.TraegerOrganization, bool, error) {
	if l.auth.traeger == nil {
		return identityaccess.TraegerOrganization{}, false, failed(opHolidayParentSetup, errTraegerMembershipsUnavailable)
	}
	organizations, err := l.auth.traeger.LiveTraegers(ctx, []int64{organizationID}, lock)
	if err != nil {
		return identityaccess.TraegerOrganization{}, false, failed(opHolidayParentSetup, err)
	}
	if len(organizations) == 0 {
		return identityaccess.TraegerOrganization{}, false, failed(opHolidayParentSetup, identityaccess.ErrHolidayParentSetupInvalid)
	}
	return organizations[0], true, nil
}
