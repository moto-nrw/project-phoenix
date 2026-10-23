package application

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

func scopeNeedsMFA(scope string) bool { return scope != domain.PortalScopeParent }

// requireSessionMFA runs under the policy/account lock at every persistence
// boundary, including refresh hand-off recovery. Legacy sessions have no proof.
// Parent sessions carry no staff authority and retain their separate policy.
func (s *AccountAuthentication) requireSessionMFA(ctx context.Context, accountID, tenantID int64, scope string, verified bool) error {
	if !s.mfa.Configured() || !scopeNeedsMFA(scope) {
		return nil
	}
	if _, err := s.store.LockAccountPermissionSources(ctx, accountID, tenantID); err != nil {
		return identityaccess.ErrMFAStatusUnavailable
	}
	permissions, err := s.loadAccountPermissionsForTenant(ctx, accountID, tenantID)
	if err != nil {
		return identityaccess.ErrMFAStatusUnavailable
	}
	policy, err := s.mfa.ResolvePolicyInTx(ctx, accountID, tenantID)
	if err != nil {
		return identityaccess.ErrMFAStatusUnavailable
	}
	if policy.RequiredFor(permissions) && !verified {
		return identityaccess.ErrInvalidToken
	}
	return nil
}

// prepareSessionMint orders policy -> account -> permission-source locks and
// rejects the transaction before it persists an unverified privileged session.
func (s *AccountAuthentication) prepareSessionMint(ctx context.Context, account domain.LoginAccount, session domain.AccountSession, tenantID int64, guard mintGuard) (time.Time, error) {
	if s.mfa.Configured() && scopeNeedsMFA(session.PortalScope) {
		if err := s.lockMFAPolicyForMint(ctx, account.ID, tenantID); err != nil {
			return time.Time{}, err
		}
	}
	if guard != nil {
		if err := guard(ctx, account); err != nil {
			return time.Time{}, &mintGuardError{err: err}
		}
	}
	loginTime := time.Now()
	if _, err := s.store.RecordAccountLogin(ctx, account.ID, loginTime); err != nil {
		return time.Time{}, fmt.Errorf("update last login before token issuance: %w", err)
	}
	if err := s.requireSessionMFA(ctx, account.ID, tenantID, session.PortalScope, session.MFAVerified); err != nil {
		return time.Time{}, &mintGuardError{err: err}
	}
	return loginTime, nil
}

// A reset/revocation may commit between the initial cookie check and mint.
// Recheck enrollment and the school-bound device while holding the same
// account lock as reset, so that stale trust cannot mint new MFA proof.
func (s *AccountAuthentication) trustedDeviceMintGuard(accountID, tenantID int64, cookie string, next mintGuard) mintGuard {
	return func(ctx context.Context, account domain.LoginAccount) error {
		if next != nil {
			if err := next(ctx, account); err != nil {
				return err
			}
		}
		if _, err := s.fetchAndValidateAccountForUpdate(ctx, accountID); err != nil {
			return err
		}
		enrolled, err := s.mfa.HasEnrollment(ctx, accountID)
		if err != nil {
			return identityaccess.ErrMFAStatusUnavailable
		}
		if !enrolled || !s.trustedDeviceVerified(ctx, accountID, tenantID, cookie) {
			return identityaccess.ErrInvalidToken
		}
		return nil
	}
}
