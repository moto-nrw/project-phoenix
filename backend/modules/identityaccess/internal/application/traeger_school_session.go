package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// Absprung in eine OGS (ADR 0084 section 5, #4654). A person with the Träger
// right "In OGS wechseln" gets a normal OGS session (scope tenant) for one
// school of the Träger, without a new sign-in and without a mapping in
// auth.account_tenants. The session names the school in tenant_id and the
// Träger in organization_id; its token carries the school's admin role and
// the mark via_traeger. The person acts as themselves: no impersonation.
// Every mint and every refresh re-checks membership, right, the Träger of the
// school and the Träger itself, under the account lock.
//
// The Träger-Portal hands the session over to the school's host: the first
// refresh token lives only for domain.TraegerSchoolHandoffTTL, and its first
// rotation on the school's host gives the session its normal lifetime. The
// rotation makes the hand-off token single-use.

// StartTraegerSchoolSession mints the hand-off session of an Absprung and
// returns its refresh token. The Träger session of the request supplies the
// second-factor proof; the Absprung is filed in the school's authentication
// ledger in the same transaction.
func (s *AccountAuthentication) StartTraegerSchoolSession(ctx context.Context, request identityaccess.TraegerSchoolSessionRequest) (_ string, err error) {
	if s == nil {
		return "", errAccountAuthenticationUnavailable
	}
	defer func() { err = authenticationError(err) }()
	ctx = bindRuntime(s.attach, ctx)
	if s.traeger == nil {
		return "", failed("Träger school session", errTraegerMembershipsUnavailable)
	}
	if request.AccountID <= 0 || request.OrganizationID <= 0 {
		return "", failed("Träger school session", identityaccess.ErrInvalidToken)
	}
	if request.SchoolID <= 0 {
		return "", failed("Träger school session", identityaccess.ErrTraegerSchoolNotFound)
	}
	account, err := s.authenticatedAccount(ctx, "Träger school session", request.AccountID)
	if err != nil {
		return "", err
	}
	// The proof is read first to stamp the session; the guard reads it again
	// under the account lock MFA reset takes.
	var proof bool
	if err := globalIdentityRead(ctx, s.runtime, func(txCtx context.Context) error {
		var proofErr error
		proof, proofErr = s.traegerSessionMFAProof(txCtx, account.ID, request.OrganizationID, request.TraegerFamilyID)
		return proofErr
	}); err != nil {
		return "", err
	}
	var metadata *domain.AccountClaimsPayload
	session := s.newRefreshSession(account.ID, domain.ScopeTenant)
	session.OrganizationID = request.OrganizationID
	session.Expiry = time.Now().Add(domain.TraegerSchoolHandoffTTL)
	session.MFAVerified = proof
	guard := s.traegerSchoolMintGuard(request.OrganizationID, request.SchoolID, &metadata)
	session, err = s.persistNewSession(ctx, account, session, request.SchoolID, s.traegerSchoolHandoffGuard(request, proof, guard), "")
	if err != nil {
		return "", err
	}
	access, refresh := buildClaims(account, session, metadata, account.Email)
	_, refreshToken, err := s.codec.IssueTokenPair(access, refresh)
	if err != nil {
		return "", failed("generate tokens", err)
	}
	return refreshToken, nil
}

// traegerSchoolHandoffGuard adds to the school guard what only the Absprung
// needs: the second-factor proof of the Träger session must still hold under
// the account lock, and the Absprung enters the school's audit in the mint
// transaction, so no session exists without its audit entry.
func (s *AccountAuthentication) traegerSchoolHandoffGuard(request identityaccess.TraegerSchoolSessionRequest, proof bool, next mintGuard) mintGuard {
	return func(ctx context.Context, account domain.LoginAccount) error {
		if err := next(ctx, account); err != nil {
			return err
		}
		current, err := s.traegerSessionMFAProof(ctx, account.ID, request.OrganizationID, request.TraegerFamilyID)
		if err != nil {
			return err
		}
		if proof && !current {
			return failed("Träger school session", identityaccess.ErrInvalidToken)
		}
		ipAddress := request.IPAddress
		if ipAddress == "" {
			ipAddress = domain.AccessAuditIP
		}
		return s.audit.RecordAuthEvent(ctx, domain.AuthEvent{
			AccountID: account.ID, TenantID: request.SchoolID, Type: domain.AuthEventTraegerSchoolSwitch, Success: true,
			IPAddress: ipAddress, UserAgent: request.UserAgent, OrganizationID: request.OrganizationID,
		})
	}
}

// traegerSchoolMintGuard re-authorizes a Träger school session inside its
// mint or refresh transaction, under the account lock, and assembles its
// claims: an active membership at an active Träger, the Träger right "In OGS
// wechseln", and a live, active school of that Träger. A school that left the
// Träger ends the session at its next refresh; a school the Träger gained is
// reachable without a grant of its own.
func (s *AccountAuthentication) traegerSchoolMintGuard(organizationID, schoolID int64, out **domain.AccountClaimsPayload) mintGuard {
	return func(ctx context.Context, presented domain.LoginAccount) error {
		if s.traeger == nil {
			return errTraegerMembershipsUnavailable
		}
		account, err := s.lockLiveTraegerAccount(ctx, presented.ID)
		if err != nil {
			return err
		}
		if err := s.requireTraegerSchoolSwitch(ctx, account.ID, organizationID); err != nil {
			return err
		}
		school, found, err := s.schools.FindSchool(ctx, schoolID)
		if err != nil {
			return fmt.Errorf("look up school %d of a Träger school session: %w", schoolID, err)
		}
		if !found || school.Deleted || !school.Active || school.OrganizationID != organizationID {
			return failed("Träger school session", identityaccess.ErrTraegerSchoolNotFound)
		}
		metadata, err := s.traegerSchoolClaims(ctx, account, schoolID, organizationID)
		if err != nil {
			return err
		}
		*out = metadata
		return nil
	}
}

// requireTraegerSchoolSwitch checks the active membership and the right "In
// OGS wechseln" of the account's current Träger roles.
func (s *AccountAuthentication) requireTraegerSchoolSwitch(ctx context.Context, accountID, organizationID int64) error {
	organizations, err := s.traeger.ListAccountTraegerOrganizations(ctx, accountID)
	if err != nil {
		return err
	}
	if !containsTraeger(organizations, organizationID) {
		return failed("Träger school session", identityaccess.ErrNoTraegerMembership)
	}
	roles, _, err := s.store.ListTraegerRoles(ctx, organizationID, accountID)
	if err != nil {
		return err
	}
	if !slices.Contains(identityaccess.TraegerRights(roles), identityaccess.TraegerRightSchoolSwitch) {
		return failed("Träger school session", identityaccess.ErrTraegerSchoolSwitchForbidden)
	}
	return nil
}

// traegerSchoolClaims is the claims material of a Träger school session: the
// school's admin role and its permissions (ADR 0084 section 5), the person's
// names, and the mark via_traeger.
func (s *AccountAuthentication) traegerSchoolClaims(ctx context.Context, account domain.LoginAccount, schoolID, organizationID int64) (*domain.AccountClaimsPayload, error) {
	role, permissions, found, _, err := s.store.FindTraegerSchoolRole(ctx, domain.AdminRoleName, schoolID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("school %d has no %s role for a Träger school session", schoolID, domain.AdminRoleName)
	}
	firstName, lastName, err := s.loadPersonNamesForTenant(ctx, account.ID, schoolID)
	if err != nil {
		return nil, err
	}
	roleNames := domain.RoleNames([]domain.RoleAssignment{role})
	return &domain.AccountClaimsPayload{
		RoleNames:   roleNames,
		Roles:       []domain.RoleAssignment{role},
		Permissions: permissions,
		Username:    account.Username,
		FirstName:   firstName,
		LastName:    lastName,
		IsAdmin:     domain.IsAdmin(roleNames),
		TenantID:    schoolID,
		OrgID:       organizationID,
		Scope:       domain.ScopeTenant,
		ViaTraeger:  true,
	}, nil
}

// traegerSessionMFAProof reads the second-factor proof of the Träger session
// the Absprung starts from. Only the account's live, unrotated session of the
// same Träger counts; a revoked, expired or foreign family carries none.
func (s *AccountAuthentication) traegerSessionMFAProof(ctx context.Context, accountID, organizationID int64, familyID string) (bool, error) {
	if familyID == "" {
		return false, nil
	}
	verified, err := s.switchMFAProof(ctx, accountID, familyID, domain.PortalScopeOrg)
	if err != nil || !verified {
		return false, err
	}
	source, err := s.sessions.LatestAccountSessionInFamily(ctx, familyID)
	if err != nil {
		return false, identityaccess.ErrMFAStatusUnavailable
	}
	return source.OrganizationID == organizationID, nil
}

// requireTraegerSchoolMFA holds a Träger school session to the second factor
// of its Träger session: it carries the school's admin rights, which the
// privileged floor never lets pass without proof (ADR 0093), and the Träger
// portal requires the second factor of every member (ADR 0084 section 3).
// Other sessions pass.
func (s *AccountAuthentication) requireTraegerSchoolMFA(session domain.AccountSession) error {
	if !domain.IsTraegerSchoolSession(session) || !s.mfa.Configured() || session.MFAVerified {
		return nil
	}
	return identityaccess.ErrInvalidToken
}

// traegerSchoolFamily reports the Träger of the presented family when it is
// a live Träger school session of the account; zero otherwise.
func (s *AccountAuthentication) traegerSchoolFamily(ctx context.Context, accountID int64, familyID string) (int64, error) {
	if familyID == "" {
		return 0, nil
	}
	var organizationID int64
	err := globalIdentityRead(ctx, s.runtime, func(txCtx context.Context) error {
		source, err := s.sessions.LatestAccountSessionInFamily(txCtx, familyID)
		if err != nil {
			if errors.Is(err, identityaccess.ErrAccountSessionNotFound) {
				return nil
			}
			return err
		}
		if source.AccountID == accountID && domain.IsTraegerSchoolSession(domain.AccountSession(source)) {
			organizationID = source.OrganizationID
		}
		return nil
	})
	return organizationID, err
}

// traegerSwitchTarget resolves the target of a tenant switch that a Träger
// school session presents: ok is true when the school of the slug belongs to
// the session's Träger and the account holds no mapping there. A school
// where the person works themselves keeps their own role.
func (s *AccountAuthentication) traegerSwitchTarget(ctx context.Context, accountID int64, tenantSlug, presentedFamilyID string) (domain.School, bool, error) {
	organizationID, err := s.traegerSchoolFamily(ctx, accountID, presentedFamilyID)
	if err != nil || organizationID <= 0 || tenantSlug == "" {
		return domain.School{}, false, err
	}
	var school domain.School
	var found, mapped bool
	err = globalIdentityRead(ctx, s.runtime, func(txCtx context.Context) error {
		var lookupErr error
		school, found, lookupErr = s.schools.FindSchoolBySubdomain(txCtx, tenantSlug)
		if lookupErr != nil || !found {
			return lookupErr
		}
		mapped, _, lookupErr = s.store.HasActiveAccountTenant(txCtx, accountID, school.ID)
		return lookupErr
	})
	if err != nil {
		return domain.School{}, false, fmt.Errorf("resolve the switch target of a Träger school session: %w", err)
	}
	return school, found && !mapped && school.OrganizationID == organizationID, nil
}

// switchTraegerSchool moves a Träger school session to another school of the
// same Träger, the switch a second tab triggers (#4654). The presented family
// is retired like any switch and supplies the second-factor proof.
func (s *AccountAuthentication) switchTraegerSchool(ctx context.Context, account domain.LoginAccount, school domain.School, presentedFamilyID string) (string, string, error) {
	var metadata *domain.AccountClaimsPayload
	session := s.newRefreshSession(account.ID, domain.ScopeTenant)
	session.OrganizationID = school.OrganizationID
	session, err := s.persistNewSession(ctx, account, session, school.ID, s.traegerSchoolMintGuard(school.OrganizationID, school.ID, &metadata), presentedFamilyID)
	if err != nil {
		return "", "", err
	}
	access, refresh := buildClaims(account, session, metadata, account.Email)
	return s.generateAndLogTokens(s.runtime.WithTenantID(ctx, school.ID), account.ID, access, refresh, "", "", domain.AuthEventTenantSwitch)
}
