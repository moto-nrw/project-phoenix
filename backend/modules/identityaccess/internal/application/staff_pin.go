package application

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// Staff PIN verification, the brute-force lockout (issue #586) and the PIN
// self-service behind /api/staff/pin. The IoT error texts are the contract
// the kiosk maps; they leave this file byte-identical.

// AuthenticateStaffPIN binds a kiosk action to one staff member by verifying
// the PIN stored on that staff member's account. The whole lookup and lockout
// update runs under tenant RLS because device middleware executes before the
// request's tenant transaction.
func (l *AccountLifecycle) AuthenticateStaffPIN(ctx context.Context, tenantID, staffID int64, pin string) (domain.AuthenticatedStaff, error) {
	const op = "authenticate staff PIN"
	if tenantID <= 0 || staffID <= 0 || pin == "" {
		return domain.AuthenticatedStaff{}, domain.ErrInvalidStaffPINCredentials
	}
	var (
		result        domain.AuthenticatedStaff
		credentialErr error
	)
	err := l.runtime.WithTenantTx(ctx, tenantID, func(txCtx context.Context) error {
		staff, account, err := l.loadStaffPINAccount(txCtx, tenantID, staffID)
		if err != nil {
			return captureCredentialError(err, &credentialErr)
		}
		if err := l.verifyStaffPIN(txCtx, account, pin); err != nil {
			return captureCredentialError(err, &credentialErr)
		}
		result = domain.AuthenticatedStaff{ID: staff.ID, PersonID: staff.PersonID, TenantID: staff.TenantID}
		return nil
	})
	if err != nil {
		return domain.AuthenticatedStaff{}, failed(op, err)
	}
	if credentialErr != nil {
		return domain.AuthenticatedStaff{}, credentialErr
	}
	if result.ID == 0 {
		return domain.AuthenticatedStaff{}, errors.New("staff PIN authentication returned no staff")
	}
	return result, nil
}

// captureCredentialError keeps a credential refusal out of the transaction
// error so the lockout counter the refusal recorded still commits.
func captureCredentialError(err error, credentialErr *error) error {
	if errors.Is(err, domain.ErrInvalidStaffPINCredentials) || errors.Is(err, domain.ErrStaffPINLocked) {
		*credentialErr = err
		return nil
	}
	return err
}

func (l *AccountLifecycle) loadStaffPINAccount(ctx context.Context, tenantID, staffID int64) (domain.StaffMember, domain.PINAccount, error) {
	staff, found, err := l.staff.FindStaff(ctx, staffID)
	if err != nil {
		return domain.StaffMember{}, domain.PINAccount{}, err
	}
	if !found || staff.TenantID != tenantID {
		return domain.StaffMember{}, domain.PINAccount{}, domain.ErrInvalidStaffPINCredentials
	}
	person, found, err := l.staff.FindPerson(ctx, staff.PersonID)
	if err != nil {
		return domain.StaffMember{}, domain.PINAccount{}, err
	}
	if !found || person.TenantID != tenantID || person.AccountID == nil {
		return domain.StaffMember{}, domain.PINAccount{}, domain.ErrInvalidStaffPINCredentials
	}
	account, found, _, err := l.store.FindPINAccount(ctx, *person.AccountID, true)
	if err != nil {
		return domain.StaffMember{}, domain.PINAccount{}, err
	}
	if !found || !account.Active || !account.HasPIN() {
		return domain.StaffMember{}, domain.PINAccount{}, domain.ErrInvalidStaffPINCredentials
	}
	hasTenantAccess, _, err := l.logins.HasActiveAccountTenant(ctx, account.ID, tenantID)
	if err != nil {
		return domain.StaffMember{}, domain.PINAccount{}, err
	}
	if !hasTenantAccess {
		return domain.StaffMember{}, domain.PINAccount{}, domain.ErrInvalidStaffPINCredentials
	}
	return staff, account, nil
}

func (l *AccountLifecycle) verifyStaffPIN(ctx context.Context, account domain.PINAccount, pin string) error {
	if domain.PINLocked(account.PINLockedUntil, time.Now()) {
		return domain.ErrStaffPINLocked
	}
	if !l.pins.VerifyPIN(pin, account.PINHash) {
		if err := l.RecordFailedPINAttempt(ctx, account.ID); err != nil {
			return err
		}
		return domain.ErrInvalidStaffPINCredentials
	}
	return l.ResetPINLockout(ctx, account.ID)
}

// RecordFailedPINAttempt atomically bumps the account's PIN-failure counter
// and applies the lockout once the tenant's threshold is crossed.
func (l *AccountLifecycle) RecordFailedPINAttempt(ctx context.Context, accountID int64) error {
	threshold, duration := l.lockout.PINLockout(ctx)
	if threshold <= 0 {
		threshold = domain.PINLockoutThreshold
	}
	if duration <= 0 {
		duration = domain.PINLockoutDuration
	}
	_, err := l.store.IncrementPINAttempts(ctx, accountID, threshold, time.Now().Add(duration))
	return err
}

// ResetPINLockout atomically clears the account's PIN-failure counter and
// lockout window after a successful verify or PIN change.
func (l *AccountLifecycle) ResetPINLockout(ctx context.Context, accountID int64) error {
	_, err := l.store.ResetPINAttempts(ctx, accountID)
	return err
}

// StaffPINStatus reports whether the account has a PIN and when it last
// changed (the account's UpdatedAt stands in for a dedicated timestamp).
func (l *AccountLifecycle) StaffPINStatus(ctx context.Context, accountID int64) (bool, *time.Time, error) {
	account, found, _, err := l.store.FindPINAccount(ctx, accountID, false)
	if err != nil || !found {
		return false, nil, domain.ErrStaffPINAccountNotFound
	}
	if !account.HasPIN() {
		return false, nil, nil
	}
	lastChanged := account.UpdatedAt
	return true, &lastChanged, nil
}

// StaffPINPreflight runs the account checks that answer before anything
// else on a PIN change: the account must exist and must not be locked.
func (l *AccountLifecycle) StaffPINPreflight(ctx context.Context, accountID int64) error {
	account, found, _, err := l.store.FindPINAccount(ctx, accountID, false)
	if err != nil || !found {
		return domain.ErrStaffPINAccountNotFound
	}
	if domain.PINLocked(account.PINLockedUntil, time.Now()) {
		return domain.ErrStaffPINSelfServiceLocked
	}
	return nil
}

// ChangeStaffPIN verifies the current PIN (a wrong one counts towards the
// lockout), hashes the new one and persists it together with a lockout reset
// in one tenant transaction.
func (l *AccountLifecycle) ChangeStaffPIN(ctx context.Context, accountID int64, currentPIN *string, newPIN string) error {
	account, found, _, err := l.store.FindPINAccount(ctx, accountID, false)
	if err != nil || !found {
		return domain.ErrStaffPINAccountNotFound
	}
	if domain.PINLocked(account.PINLockedUntil, time.Now()) {
		return domain.ErrStaffPINSelfServiceLocked
	}
	if account.HasPIN() {
		if currentPIN == nil || *currentPIN == "" {
			return domain.ErrStaffPINCurrentRequired
		}
		if !l.pins.VerifyPIN(*currentPIN, account.PINHash) {
			if updateErr := l.RecordFailedPINAttempt(ctx, account.ID); updateErr != nil {
				l.logger.Error("failed to update account PIN attempts", slog.String("error", updateErr.Error()))
			}
			return domain.ErrStaffPINCurrentWrong
		}
	}
	hash, err := l.pins.HashPIN(newPIN)
	if err != nil {
		return domain.ErrStaffPINHash
	}
	tenantID := l.runtime.TenantID(ctx)
	return l.runtime.WithTenantTx(ctx, tenantID, func(txCtx context.Context) error {
		if _, err := l.store.UpdatePINHash(txCtx, account.ID, hash); err != nil {
			return err
		}
		return l.ResetPINLockout(txCtx, account.ID)
	})
}
