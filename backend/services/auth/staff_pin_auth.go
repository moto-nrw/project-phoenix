package auth

import (
	"context"
	"errors"
	"time"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type staffPINAuthenticationResult struct {
	staff         *userModels.Staff
	credentialErr error
}

// AuthenticateStaffPIN binds a kiosk action to one staff member by verifying
// the PIN stored on that staff member's account. The whole lookup and lockout
// update runs under tenant RLS because device middleware executes before the
// request's TenantTxMiddleware.
func (s *Service) AuthenticateStaffPIN(
	ctx context.Context,
	tenantID, staffID int64,
	pin string,
) (*userModels.Staff, error) {
	ctx = s.withTenantRuntime(ctx)
	if tenantID <= 0 || staffID <= 0 || pin == "" {
		return nil, ErrInvalidStaffPINCredentials
	}

	var result staffPINAuthenticationResult
	err := tenant.WithTenantTx(s.withTenantRuntime(ctx), s.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		staff, account, err := s.loadStaffPINAccount(txCtx, tenantID, staffID)
		if err != nil {
			return result.captureError(err)
		}
		if err := s.verifyStaffPIN(txCtx, account, pin); err != nil {
			return result.captureError(err)
		}
		result.staff = staff
		return nil
	})
	if err != nil {
		return nil, &AuthError{Op: "authenticate staff PIN", Err: err}
	}
	if result.credentialErr != nil {
		return nil, result.credentialErr
	}
	if result.staff == nil {
		return nil, errors.New("staff PIN authentication returned no staff")
	}
	return result.staff, nil
}

func (r *staffPINAuthenticationResult) captureError(err error) error {
	if errors.Is(err, ErrInvalidStaffPINCredentials) || errors.Is(err, ErrStaffPINLocked) {
		r.credentialErr = err
		return nil
	}
	return err
}

func (s *Service) loadStaffPINAccount(
	ctx context.Context,
	tenantID, staffID int64,
) (*userModels.Staff, *authModels.Account, error) {
	staff, err := s.repos.Staff.FindByID(ctx, staffID)
	if err != nil {
		return nil, nil, staffPINLookupError(err)
	}
	if staff == nil || staff.GetTenantID() != tenantID {
		return nil, nil, ErrInvalidStaffPINCredentials
	}

	person, err := s.repos.Person.FindByID(ctx, staff.PersonID)
	if err != nil {
		return nil, nil, staffPINLookupError(err)
	}
	if person == nil || person.GetTenantID() != tenantID || person.AccountID == nil {
		return nil, nil, ErrInvalidStaffPINCredentials
	}

	account, err := s.repos.Account.FindByIDForUpdate(ctx, *person.AccountID)
	if err != nil {
		return nil, nil, staffPINLookupError(err)
	}
	if account == nil || !account.Active || !account.HasPIN() {
		return nil, nil, ErrInvalidStaffPINCredentials
	}
	hasTenantAccess, err := s.repos.AccountTenant.ExistsByAccountAndTenant(ctx, account.ID, tenantID)
	if err != nil {
		return nil, nil, err
	}
	if !hasTenantAccess {
		return nil, nil, ErrInvalidStaffPINCredentials
	}
	return staff, account, nil
}

func staffPINLookupError(err error) error {
	if base.IsNoRows(err) {
		return ErrInvalidStaffPINCredentials
	}
	return err
}

func (s *Service) verifyStaffPIN(
	ctx context.Context,
	account *authModels.Account,
	pin string,
) error {
	if s.IsPINLocked(account, time.Now()) {
		return ErrStaffPINLocked
	}
	if !account.VerifyPIN(pin) {
		if err := s.RecordFailedPINAttempt(ctx, account.ID); err != nil {
			return err
		}
		return ErrInvalidStaffPINCredentials
	}
	return s.ResetPINLockout(ctx, account.ID)
}
