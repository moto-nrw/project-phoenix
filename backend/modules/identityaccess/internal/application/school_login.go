package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/domain"
)

func (s *AccountAuthentication) loginSchoolWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie, targetTenantSlug string) (*domain.LoginResult, error) {
	account, err := s.validateLoginCredentials(ctx, email, password, ipAddress, userAgent)
	if err != nil {
		return nil, err
	}

	portalTenantID, err := s.schoolLoginTenant(ctx, account, targetTenantSlug, ipAddress, userAgent)
	if err != nil {
		return nil, err
	}
	// The liveness gates decide the response; the roles they hydrate feed
	// the MFA verdict below. The JWT is deliberately not built from this
	// pre-transaction snapshot: the mint guard reloads it under its lock.
	metadata, err := s.loadSchoolMetadataForTenant(ctx, account, portalTenantID)
	if err != nil {
		return nil, err
	}

	mfaRequired, enrolled, err := s.schoolLoginMFAVerdict(ctx, account.ID, portalTenantID, metadata.Permissions)
	if err != nil {
		return nil, err
	}
	if mfaRequired && !enrolled {
		return s.mfaEnrollmentResult(account, portalTenantID, domain.PortalSchool)
	}
	if mfaRequired && enrolled && !s.trustedDeviceVerified(ctx, account.ID, portalTenantID, trustedDeviceCookie) {
		return s.mfaChallengeResult(ctx, account, portalTenantID, domain.PortalSchool, ipAddress)
	}

	return s.mintSchoolLogin(ctx, account, portalTenantID, mfaRequired, enrolled, email, ipAddress, userAgent, trustedDeviceCookie)
}

// IssueSchoolTokensForVerifiedMFA mints a school-scope pair after the school
// MFA exchange. It re-validates the school-portal role at the tenant carried
// in the challenge, so a role revoked mid-challenge cannot be laundered into
// a school session.
func (s *AccountAuthentication) IssueSchoolTokensForVerifiedMFA(ctx context.Context, proof identityaccess.VerifiedMFAChallenge, ipAddress, userAgent string) (string, string, error) {
	return runFlow2(ctx, s.flow(), func(ctx context.Context) (string, string, error) {
		const op = "issue school tokens"
		account, err := s.loadActiveAccountForSchoolMint(ctx, op, proof.AccountID)
		if err != nil {
			return "", "", err
		}
		if err := s.requireSchoolTarget(ctx, op, account, proof.TenantID); err != nil {
			if errors.Is(err, identityaccess.ErrAccountNoSchoolPortalRole) {
				s.logFailedLogin(s.runtime.WithTenantID(ctx, proof.TenantID), account.ID, ipAddress, userAgent, "No school portal role at school token issue")
			}
			return "", "", err
		}
		minted, err := s.mintAccountSession(ctx, mintRequest{
			portal: domain.PortalSchool, account: account, tenantID: proof.TenantID, proof: challengeProof(&proof),
			audit: &mintAudit{event: domain.AuthEventLogin, ipAddress: ipAddress, userAgent: userAgent, schoolID: proof.TenantID},
		})
		if err != nil {
			return "", "", wrapSchoolMintError(op, err)
		}
		return minted.accessToken, minted.refreshToken, nil
	})
}

// requireSchoolTarget runs the checks that choose the response of a school
// mint before its transaction: the school and mapping are live, the account
// holds a school-portal role there and the school booked the portal.
func (s *AccountAuthentication) requireSchoolTarget(ctx context.Context, op string, account domain.LoginAccount, tenantID int64) error {
	if _, err := s.loadSchoolMetadataForTenant(ctx, account, tenantID); err != nil {
		return err
	}
	hasRole, err := s.hasSchoolPortalRoleAtTenant(ctx, account.ID, tenantID)
	if err != nil {
		return err
	}
	if !hasRole {
		return failed(op, identityaccess.ErrAccountNoSchoolPortalRole)
	}
	return s.requireSchoolPortal(ctx, op, account.ID, tenantID)
}

// SwitchSchool re-authenticates a school-portal session to another school
// where the account holds a school-portal role. The signed source family must
// still carry live MFA proof if the target school's current policy requires it.
func (s *AccountAuthentication) SwitchSchool(ctx context.Context, accountID int64, tenantSlug, ipAddress, userAgent, sourceFamilyID string) (string, string, error) {
	return runFlow2(ctx, s.flow(), func(ctx context.Context) (string, string, error) {
		account, err := s.loadActiveAccountForSchoolMint(ctx, "switch school", accountID)
		if err != nil {
			return "", "", err
		}
		var targetTenantID int64
		if err := s.runtime.WithAdminTx(ctx, func(adminCtx context.Context) error {
			tenantID, _, resolveErr := s.resolveAccountTenantBySlug(adminCtx, accountID, tenantSlug)
			if resolveErr != nil {
				return resolveErr
			}
			targetTenantID = tenantID
			return nil
		}); err != nil {
			return "", "", err
		}
		if err := s.requireSchoolTarget(ctx, "switch school", account, targetTenantID); err != nil {
			return "", "", err
		}
		minted, err := s.mintAccountSession(ctx, mintRequest{
			portal: domain.PortalSchool, account: account, tenantID: targetTenantID,
			sourceFamilyID: sourceFamilyID, retireFamilyID: sourceFamilyID,
			audit: &mintAudit{event: domain.AuthEventTenantSwitch, ipAddress: ipAddress, userAgent: userAgent, schoolID: targetTenantID},
		})
		if err != nil {
			return "", "", wrapSchoolMintError("switch school", err)
		}
		return minted.accessToken, minted.refreshToken, nil
	})
}

// loadActiveAccountForSchoolMint fetches the account a non-password school
// mint is about to issue a token for. Only a genuine miss is
// ErrAccountNotFound; a database failure propagates as a retryable error
// instead of telling a Lehrkraft their credentials were wrong.
func (s *AccountAuthentication) loadActiveAccountForSchoolMint(ctx context.Context, op string, accountID int64) (domain.LoginAccount, error) {
	account, found, _, err := findLoginAccount(ctx, s.runtime, s.store, accountID, false)
	if err != nil {
		return domain.LoginAccount{}, failed(op, fmt.Errorf("look up account %d: %w", accountID, err))
	}
	if !found {
		return domain.LoginAccount{}, failed(op, domain.ErrAccountNotFound)
	}
	if !account.Active {
		return domain.LoginAccount{}, failed(op, identityaccess.ErrAccountInactive)
	}
	return account, nil
}

// errSchoolMFARequiredAtMint aborts a mint whose MFA gate has gone stale. It
// never leaves the flow: the school login answers with the challenge or
// enrollment token the login would have returned.
var errSchoolMFARequiredAtMint = errors.New("mfa became required before the school token was minted")

// wrapSchoolMintError gives a guard sentinel the operation envelope every
// school handler switches on, leaving errors that already carry one alone.
func wrapSchoolMintError(op string, err error) error {
	var wrapped *OperationError
	if errors.As(err, &wrapped) {
		return err
	}
	return failed(op, err)
}

// loadSchoolMetadataForTenant loads the tenant-scoped claims material,
// stamps the school scope and runs the liveness gates every school mint
// passes before it writes anything: the school must be alive and active and
// the mapping must be active. The gates decide the response; the guard
// repeats them inside the mint transaction.
func (s *AccountAuthentication) loadSchoolMetadataForTenant(ctx context.Context, account domain.LoginAccount, tenantID int64) (*domain.AccountClaimsPayload, error) {
	metadata, err := s.loadAccountMetadataForTenant(ctx, account, tenantID)
	if err != nil {
		return nil, err
	}
	metadata.Scope = domain.PortalSchool.Claim
	var (
		school        domain.School
		schoolFound   bool
		activeMapping bool
	)
	if txErr := s.runtime.WithAdminTx(ctx, func(adminCtx context.Context) error {
		var lookupErr error
		school, schoolFound, lookupErr = s.schools.FindSchool(adminCtx, tenantID)
		if lookupErr != nil {
			return fmt.Errorf("lookup school %d for school metadata: %w", tenantID, lookupErr)
		}
		activeMapping, _, lookupErr = s.store.HasActiveAccountTenant(adminCtx, account.ID, tenantID)
		if lookupErr != nil {
			return fmt.Errorf("verify account %d membership at school %d: %w", account.ID, tenantID, lookupErr)
		}
		return nil
	}); txErr != nil {
		return nil, txErr
	}
	if !schoolFound || !school.Live() {
		return nil, failed("load school metadata", identityaccess.ErrTenantNotFound)
	}
	if !activeMapping {
		return nil, failed("load school metadata", identityaccess.ErrTenantAccessDenied)
	}
	return metadata, nil
}

// schoolClaimsPayloadInTx loads the claims payload of a school token on the
// caller's transaction and stamps the school scope; no liveness gate runs
// here because the guard settled them microseconds earlier under its locks.
func (s *AccountAuthentication) schoolClaimsPayloadInTx(ctx context.Context, account domain.LoginAccount, tenantID int64) (*domain.AccountClaimsPayload, error) {
	metadata, err := s.loadAccountMetadataForTenantInTx(ctx, account, tenantID)
	if err != nil {
		return nil, err
	}
	metadata.Scope = domain.PortalSchool.Claim
	return metadata, nil
}

// hasSchoolPortalRoleAtTenant reports whether the account holds a
// school-portal role at the tenant; query errors propagate so a transient
// failure never masquerades as a revocation.
func (s *AccountAuthentication) hasSchoolPortalRoleAtTenant(ctx context.Context, accountID, tenantID int64) (bool, error) {
	var hasRole bool
	err := s.runtime.WithAdminTx(ctx, func(adminCtx context.Context) error {
		roles, _, rolesErr := s.store.ListAccountRolesAtTenant(adminCtx, accountID, tenantID, false)
		if rolesErr != nil {
			return rolesErr
		}
		hasRole = domain.HasSchoolPortalRole(roles)
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("load school portal role for account %d at tenant %d: %w", accountID, tenantID, err)
	}
	return hasRole, nil
}

// findSchoolPortalTenantForAccount checks every active mapping for a
// school-portal role. Without requireBooked it returns the first matching
// school that is alive and active; when required, it continues until it finds
// a school that also booked Unterricht am Vormittag. A full sweep of school
// lookup failures is not masked as "no portal role".
func (s *AccountAuthentication) findSchoolPortalTenantForAccount(ctx context.Context, accountID int64, requireBooked bool) (bool, int64, error) {
	hasPortalRole := false
	var firstPortalTenantID int64
	var portalTenantIDs []int64
	if err := s.runtime.WithAdminTx(ctx, func(adminCtx context.Context) error {
		var err error
		if requireBooked {
			portalTenantIDs, err = s.schoolPortalTenants(adminCtx, accountID)
			hasPortalRole = len(portalTenantIDs) > 0
			return err
		}
		hasPortalRole, firstPortalTenantID, err = s.firstSchoolPortalTenant(adminCtx, accountID)
		return err
	}); err != nil {
		return false, 0, err
	}
	if requireBooked {
		return s.findBookedSchoolPortalTenant(ctx, portalTenantIDs)
	}
	return hasPortalRole, firstPortalTenantID, nil
}

func (s *AccountAuthentication) findBookedSchoolPortalTenant(ctx context.Context, portalTenantIDs []int64) (bool, int64, error) {
	var firstNotBooked error
	for _, tenantID := range portalTenantIDs {
		err := s.portalModules.RequireSchoolPortal(ctx, tenantID)
		if err == nil {
			return true, tenantID, nil
		}
		if _, notBooked := identityaccess.PortalNotBooked(err); !notBooked {
			return false, 0, err
		}
		if firstNotBooked == nil {
			firstNotBooked = err
		}
	}
	if firstNotBooked != nil {
		return true, portalTenantIDs[0], firstNotBooked
	}
	return false, 0, nil
}

// HasSchoolPortalAccess reports whether the account may still hold a school
// session at this school: the account is active, the school is alive and
// active, the mapping is active and the account holds a school-portal role.
// A revoked fact answers (false, nil); lookup errors propagate so a database
// blip is not a revocation.
func (s *AccountAuthentication) HasSchoolPortalAccess(ctx context.Context, accountID, tenantID int64) (_ bool, err error) {
	if s == nil {
		return false, errAccountAuthenticationUnavailable
	}
	defer func() { err = authenticationError(err) }()
	ctx = bindRuntime(s.attach, ctx)
	account, found, _, err := findLoginAccount(ctx, s.runtime, s.store, accountID, false)
	if err != nil {
		return false, fmt.Errorf("look up account %d for school portal access: %w", accountID, err)
	}
	if !found || !account.Active {
		return false, nil
	}
	var (
		school        domain.School
		schoolFound   bool
		activeMapping bool
	)
	if txErr := s.runtime.WithAdminTx(ctx, func(adminCtx context.Context) error {
		var lookupErr error
		school, schoolFound, lookupErr = s.schools.FindSchool(adminCtx, tenantID)
		if lookupErr != nil {
			return fmt.Errorf("look up school %d for school portal access: %w", tenantID, lookupErr)
		}
		activeMapping, _, lookupErr = s.store.HasActiveAccountTenant(adminCtx, accountID, tenantID)
		if lookupErr != nil {
			return fmt.Errorf("verify account %d membership at school %d: %w", accountID, tenantID, lookupErr)
		}
		return nil
	}); txErr != nil {
		return false, txErr
	}
	if !schoolFound || !school.Live() || !activeMapping {
		return false, nil
	}
	return s.hasSchoolPortalRoleAtTenant(ctx, accountID, tenantID)
}

// findSchoolPortalTenant is the first live school at which the account
// holds a school-portal role: the password reset routes such an account to
// the school reset link.
func (s *AccountAuthentication) findSchoolPortalTenant(ctx context.Context, accountID int64) (bool, int64, error) {
	return s.findSchoolPortalTenantForAccount(ctx, accountID, false)
}
