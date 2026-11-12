package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/domain"
)

// The readers the portals' session facts and claims share. What an account
// must still hold per portal is sessionPolicies (session_policy.go).

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

// traegerClaims are the Träger roles and their rights; no school role
// enters them.
func (s *AccountAuthentication) traegerClaims(ctx context.Context, account domain.LoginAccount, organizationID int64) (*domain.AccountClaimsPayload, error) {
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

// traegerGrantsSchool: an active membership at an active Träger, the right
// "In OGS wechseln", and a live, active school of that Träger. A school that
// left the Träger ends the session at its next refresh; a school the Träger
// gained is reachable without a grant of its own. Mint and refresh hold the
// school under a share lock after the account lock, so retirement cannot
// finish while either still mints a session (#6305).
func (s *AccountAuthentication) traegerGrantsSchool(ctx context.Context, accountID, organizationID, schoolID int64, r factReads) error {
	if s.traeger == nil {
		return errTraegerMembershipsUnavailable
	}
	if err := s.requireTraegerSchoolSwitch(ctx, accountID, organizationID); err != nil {
		return err
	}
	school, found, err := r.school(ctx, s, schoolID)
	if err != nil {
		return fmt.Errorf("look up school %d of a Träger school session: %w", schoolID, err)
	}
	if !found || school.Deleted || !school.Active || school.OrganizationID != organizationID {
		return failed("Träger school session", identityaccess.ErrTraegerSchoolNotFound)
	}
	return nil
}

// requireCaterer: the account belongs to a caterer (ADR 0077). The live
// supplied school the session row is kept at is the storage key, never an
// access right: login pins it before the mint, refresh re-pins it in the
// rotation (catererSessionSchool).
func (s *AccountAuthentication) requireCaterer(ctx context.Context, accountID int64) error {
	if s.caterers == nil {
		return domain.ErrAccountNotCaterer
	}
	_, isCaterer, _, err := s.caterers.FindCatererOfAccount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("find caterer: %w", err)
	}
	if !isCaterer {
		return domain.ErrAccountNotCaterer
	}
	return nil
}
