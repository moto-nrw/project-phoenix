package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// scopeNeedsMFA selects the sessions the school MFA policy governs. The
// Träger-Portal is not bound to the MFA mode of any school (ADR 0084): every
// Träger session needs its second factor (#4651), see requireSessionMFA.
func scopeNeedsMFA(scope string) bool {
	return scope != domain.PortalScopeParent && scope != domain.PortalScopeOrg
}

// requireSessionMFA runs under the policy/account lock at every persistence
// boundary, including refresh hand-off recovery. Legacy sessions have no proof.
// Parent sessions carry no staff authority and retain their separate policy.
// A Träger session needs its proof for every role, so a session minted before
// the duty or one whose proof an MFA reset revoked is refused at mint and at
// refresh.
func (s *AccountAuthentication) requireSessionMFA(ctx context.Context, accountID, tenantID int64, scope string, verified bool) error {
	if !s.mfa.Configured() {
		return nil
	}
	if scope == domain.PortalScopeOrg {
		if !verified {
			return identityaccess.ErrInvalidToken
		}
		return nil
	}
	if !scopeNeedsMFA(scope) {
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
func (s *AccountAuthentication) prepareSessionMint(ctx context.Context, account domain.LoginAccount, session *domain.AccountSession, tenantID int64, guard mintGuard, retireFamilyID string) (time.Time, error) {
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
	if err := s.adoptSourceFamily(ctx, account.ID, retireFamilyID, session, loginTime); err != nil {
		return time.Time{}, &mintGuardError{err: err}
	}
	if err := s.requireSessionMFA(ctx, account.ID, tenantID, session.PortalScope, session.MFAVerified); err != nil {
		return time.Time{}, &mintGuardError{err: err}
	}
	if err := s.requireTraegerSchoolMFA(*session); err != nil {
		return time.Time{}, &mintGuardError{err: err}
	}
	return loginTime, nil
}

// The signed access token supplies the family, not a client-provided MFA flag.
// Read its current generation after taking the account lock shared with MFA
// reset. A revoked, expired or foreign-portal family cannot restore proof,
// even while its old access JWT remains valid.
func (s *AccountAuthentication) switchMFAProof(ctx context.Context, accountID int64, familyID, scope string) (bool, error) {
	source, err := s.sessions.LatestAccountSessionInFamily(ctx, familyID)
	if err != nil {
		if errors.Is(err, identityaccess.ErrAccountSessionNotFound) {
			return false, nil
		}
		return false, identityaccess.ErrMFAStatusUnavailable
	}
	now := time.Now()
	return source.AccountID == accountID && source.PortalScope == scope &&
		source.MFAVerified && source.RotatedAt == nil && source.Expiry.After(now) &&
		(source.FamilyExpiryCap == nil || source.FamilyExpiryCap.After(now)), nil
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

// Reset and mint hold the same account row lock. A code proved before reset
// must never restore the session proof reset has just revoked, even where
// the current school does not itself require MFA.
func (s *AccountAuthentication) verifiedMFAMintGuard(proof *identityaccess.VerifiedMFAChallenge, scope string, next mintGuard) mintGuard {
	if proof == nil {
		return next
	}
	return func(ctx context.Context, account domain.LoginAccount) error {
		current, err := s.fetchAndValidateAccountForUpdate(ctx, account.ID)
		if err != nil {
			return err
		}
		if proof.AccountID != account.ID || proof.Scope != scope || proof.MFAResetGeneration != current.MFAResetGeneration {
			return identityaccess.ErrInvalidToken
		}
		if next != nil {
			return next(ctx, current)
		}
		return nil
	}
}

// adoptSourceFamily checks the family a switch presents: it must still live,
// and a tenant or school switch inherits its second-factor proof.
func (s *AccountAuthentication) adoptSourceFamily(ctx context.Context, accountID int64, familyID string, session *domain.AccountSession, now time.Time) error {
	origin, err := s.liveSourceLineage(ctx, accountID, familyID, now)
	if err != nil {
		return err
	}
	if familyID == "" {
		return nil
	}
	// The link is written under the account lock logout takes: a logout
	// that runs after this switch commits finds the new family through it
	// (#5422); one that ran before left no source family to switch from.
	session.SwitchedFromFamilyID = origin
	if session.PortalScope != domain.PortalScopeTenant && session.PortalScope != domain.PortalScopeSchool {
		return nil
	}
	verified, err := s.switchMFAProof(ctx, accountID, familyID, session.PortalScope)
	if err != nil {
		return err
	}
	session.MFAVerified = verified
	return nil
}

// liveSourceLineage returns the origin of the presented family's switch
// lineage, refusing a switch whose presented family is gone (#5194). It runs
// under the account lock logout takes as well, so a switch that started
// before a logout either commits first or finds the family revoked; it
// cannot mint a new session out of a revoked one.
func (s *AccountAuthentication) liveSourceLineage(ctx context.Context, accountID int64, familyID string, now time.Time) (string, error) {
	if familyID == "" {
		return "", nil
	}
	source, err := s.sessions.LatestAccountSessionInFamily(ctx, familyID)
	if errors.Is(err, identityaccess.ErrAccountSessionNotFound) {
		return "", identityaccess.ErrInvalidToken
	}
	if err != nil {
		return "", fmt.Errorf("inspect source session family: %w", err)
	}
	if source.AccountID != accountID || !source.Expiry.After(now) {
		return "", identityaccess.ErrInvalidToken
	}
	// Keep the root, not the immediate parent: cleanup may remove every
	// intermediate family while later descendants are still valid.
	return domain.SwitchLineage(domain.AccountSession(source)), nil
}
