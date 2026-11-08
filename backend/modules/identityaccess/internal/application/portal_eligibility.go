package application

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// The eligibility of each portal: what an account must still hold, under the
// account lock of the mint or refresh transaction, to get a session there.
// Mint and refresh run the same guard; refresh only maps some refusals to the
// sentinels its route answers with.

// tenantPortalRefusal is the tenant portal's rule on the account's roles at
// the school: a guardian-only account belongs in the parents portal, a
// school-portal-only account in moto schule (#2207). Dual-role accounts pass.
func tenantPortalRefusal(metadata *domain.AccountClaimsPayload) error {
	if domain.IsGuardianOnly(metadata.RoleNames) {
		return identityaccess.ErrParentMustUseParentPortal
	}
	if domain.IsSchoolPortalOnly(metadata.Roles) {
		return identityaccess.ErrMustUseSchoolPortal
	}
	return nil
}

// refuseTenantPortal answers a login or token issue that the tenant portal
// refuses before it asks for a second factor, and files the refusal.
func (s *AccountAuthentication) refuseTenantPortal(ctx context.Context, op string, account domain.LoginAccount, metadata *domain.AccountClaimsPayload, ipAddress, userAgent string) error {
	err := tenantPortalRefusal(metadata)
	switch {
	case errors.Is(err, identityaccess.ErrParentMustUseParentPortal):
		s.logFailedLogin(ctx, account.ID, ipAddress, userAgent, "Guardian-only account at tenant "+op)
	case errors.Is(err, identityaccess.ErrMustUseSchoolPortal):
		s.logFailedLogin(ctx, account.ID, ipAddress, userAgent, "School-portal-only account at tenant "+op)
	default:
		return nil
	}
	return failed(op, err)
}

// tenantEligibility: an active mapping at the school and a role set the
// tenant portal serves. A refresh token from before the tenant claim names
// school zero and has no mapping to check; its successor gets the default
// mapping.
func (s *AccountAuthentication) tenantEligibility(op string, tenantID int64) eligibility {
	return func(ctx context.Context, account domain.LoginAccount) (*domain.AccountClaimsPayload, error) {
		if tenantID > 0 {
			mapped, _, err := s.store.LockActiveTenantMappingShared(ctx, account.ID, tenantID)
			if err != nil {
				return nil, fmt.Errorf("re-check membership of account %d at school %d: %w", account.ID, tenantID, err)
			}
			if !mapped {
				return nil, failed(op, identityaccess.ErrTenantAccessDenied)
			}
		}
		metadata, err := s.loadAccountMetadataForTenantInTx(ctx, account, tenantID)
		if err != nil {
			return nil, err
		}
		if err := tenantPortalRefusal(metadata); err != nil {
			return nil, failed(op, err)
		}
		return metadata, nil
	}
}

// tenantRefreshEligibility is the tenant guard of a refresh. A refresh token
// from before the scope claim may belong to a parent session: a guardian-only
// role set at its school renews it as a parent session.
func (s *AccountAuthentication) tenantRefreshEligibility(tenantID int64) eligibility {
	tenant := s.tenantEligibility("refresh token", tenantID)
	return func(ctx context.Context, account domain.LoginAccount) (*domain.AccountClaimsPayload, error) {
		metadata, err := tenant(ctx, account)
		if errors.Is(err, identityaccess.ErrParentMustUseParentPortal) {
			return buildParentMetadata(account), nil
		}
		return metadata, err
	}
}

// schoolEligibility: the school is alive and active, the mapping is active
// and the account holds a school-portal role there. With recheckMFAGate the
// MFA gate the login passed without a challenge is decided again on the
// roles read under the lock: a role granted or mfa_mode switched on in the
// gap aborts with errSchoolMFARequiredAtMint, and an unreadable policy fails
// closed.
func (s *AccountAuthentication) schoolEligibility(tenantID int64, recheckMFAGate bool) eligibility {
	return func(ctx context.Context, account domain.LoginAccount) (*domain.AccountClaimsPayload, error) {
		if err := s.checkSchoolMintPreconditions(ctx, account.ID, tenantID); err != nil {
			return nil, err
		}
		if _, err := s.store.LockAccountPermissionSources(ctx, account.ID, tenantID); err != nil {
			return nil, fmt.Errorf("lock permission sources of account %d at school %d: %w", account.ID, tenantID, err)
		}
		payload, err := s.schoolClaimsPayloadInTx(ctx, account, tenantID)
		if err != nil {
			return nil, err
		}
		if recheckMFAGate {
			if err := s.requireSchoolMintMFAPolicy(ctx, account.ID, tenantID, payload); err != nil {
				return nil, err
			}
		}
		return payload, nil
	}
}

// schoolRefreshEligibility reports a revoked portal role as
// ErrTenantAccessDenied, the sentinel the refresh route maps, and logs every
// refusal with the other refresh rejections.
func (s *AccountAuthentication) schoolRefreshEligibility(tenantID int64) eligibility {
	school := s.schoolEligibility(tenantID, false)
	return func(ctx context.Context, account domain.LoginAccount) (*domain.AccountClaimsPayload, error) {
		payload, err := school(ctx, account)
		switch {
		case errors.Is(err, identityaccess.ErrAccountNoSchoolPortalRole):
			s.logRefreshDecision("refresh_session_rejected", "school_portal_role_revoked", account.ID, tenantID)
			return nil, identityaccess.ErrTenantAccessDenied
		case errors.Is(err, identityaccess.ErrTenantAccessDenied):
			s.logRefreshDecision("refresh_session_rejected", "tenant_access_revoked", account.ID, tenantID)
		}
		return payload, err
	}
}

// parentEligibility: a guardian role at a live school that booked Eltern-App
// (#4895). The booking is read before the transaction; the role and the
// school are read again under the lock.
func (s *AccountAuthentication) parentEligibility(op string, bookedSchoolIDs []int64) eligibility {
	return func(ctx context.Context, account domain.LoginAccount) (*domain.AccountClaimsPayload, error) {
		guardianSchoolIDs, err := s.guardianSchoolsInTx(ctx, account.ID, true)
		if err != nil {
			return nil, err
		}
		if !slices.ContainsFunc(guardianSchoolIDs, func(id int64) bool { return slices.Contains(bookedSchoolIDs, id) }) {
			return nil, failed(op, identityaccess.ErrAccountNoGuardianRole)
		}
		return buildParentMetadata(account), nil
	}
}

// traegerEligibility: an active membership at the active Träger. The claims
// are the Träger roles and their rights; no school role enters them.
func (s *AccountAuthentication) traegerEligibility(organizationID int64) eligibility {
	return func(ctx context.Context, account domain.LoginAccount) (*domain.AccountClaimsPayload, error) {
		if err := s.requireTraegerMember(ctx, account.ID, organizationID); err != nil {
			return nil, err
		}
		roles, _, err := s.store.ListTraegerRoles(ctx, organizationID, account.ID)
		if err != nil {
			return nil, err
		}
		if roles == nil {
			roles = []string{}
		}
		return &domain.AccountClaimsPayload{
			RoleNames:   roles,
			Permissions: identityaccess.TraegerRights(roles),
			Username:    account.Username,
			OrgID:       organizationID,
			Scope:       domain.PortalOrg.Claim,
		}, nil
	}
}

func (s *AccountAuthentication) requireTraegerMember(ctx context.Context, accountID, organizationID int64) error {
	if s.traeger == nil {
		return errTraegerMembershipsUnavailable
	}
	organizations, err := s.traeger.ListAccountTraegerOrganizations(ctx, accountID)
	if err != nil {
		return err
	}
	if !containsTraeger(organizations, organizationID) {
		return failed("Träger session", identityaccess.ErrNoTraegerMembership)
	}
	return nil
}

// traegerSchoolEligibility: an active membership at an active Träger, the
// right "In OGS wechseln", and a live, active school of that Träger. A school
// that left the Träger ends the session at its next refresh; a school the
// Träger gained is reachable without a grant of its own.
func (s *AccountAuthentication) traegerSchoolEligibility(organizationID, schoolID int64) eligibility {
	return func(ctx context.Context, account domain.LoginAccount) (*domain.AccountClaimsPayload, error) {
		if err := s.traegerGrantsSchool(ctx, account.ID, organizationID, schoolID); err != nil {
			return nil, err
		}
		return s.traegerSchoolClaims(ctx, account, schoolID, organizationID)
	}
}

func (s *AccountAuthentication) traegerGrantsSchool(ctx context.Context, accountID, organizationID, schoolID int64) error {
	if s.traeger == nil {
		return errTraegerMembershipsUnavailable
	}
	if err := s.requireTraegerSchoolSwitch(ctx, accountID, organizationID); err != nil {
		return err
	}
	school, found, err := s.schools.FindSchool(ctx, schoolID)
	if err != nil {
		return fmt.Errorf("look up school %d of a Träger school session: %w", schoolID, err)
	}
	if !found || school.Deleted || !school.Active || school.OrganizationID != organizationID {
		return failed("Träger school session", identityaccess.ErrTraegerSchoolNotFound)
	}
	return nil
}

// catererEligibility: the account belongs to a caterer (ADR 0077). The live
// supplied school the session row is kept at is the storage key, never an
// access right: login pins it before the mint, refresh re-pins it in the
// rotation (catererSessionSchool).
func (s *AccountAuthentication) catererEligibility() eligibility {
	return func(ctx context.Context, account domain.LoginAccount) (*domain.AccountClaimsPayload, error) {
		if s.caterers == nil {
			return nil, domain.ErrAccountNotCaterer
		}
		_, isCaterer, _, err := s.caterers.FindCatererOfAccount(ctx, account.ID)
		if err != nil {
			return nil, fmt.Errorf("find caterer: %w", err)
		}
		if !isCaterer {
			return nil, domain.ErrAccountNotCaterer
		}
		return buildCatererMetadata(account), nil
	}
}
