package application

import (
	"context"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// Operation names the invitation flows report.
const (
	opCreateInvitation = "create invitation"
	opAcceptInvitation = "accept invitation"
	opResendInvitation = "resend invitation"
	opRevokeInvitation = "revoke invitation"
	opFetchInvitation  = "fetch invitation"
)

// maxInvitationEmailLength is the column limit an address must fit.
const maxInvitationEmailLength = 255

// baseRoleUser is the tier a caregiver upgrade grants on top of the invited
// role; the public package owns the tier vocabulary.
const baseRoleUser = "user"

// SchoolInvitation runs the school invitation flows (#2722): creating the
// link with the role-grant check, the public preview, the acceptance that
// creates or reuses the account and provisions its identity, resend, revoke
// and the cleanup. auth.invitation_tokens and the account rows an
// acceptance writes belong to the module; the school facts, the role-grant
// policy, the identity chain, the owner-token check and the mail arrive
// through the ports the composition binds.
type SchoolInvitation struct {
	store     ports.SchoolInvitationStore
	logins    ports.AccountLoginStore
	roles     ports.RoleStore
	policy    ports.RoleAssignmentPolicy
	grants    ports.InvitationGrantPolicy
	identity  ports.SchoolIdentity
	schools   ports.SchoolDirectory
	owners    ports.InvitationOwnerTokens
	passwords ports.PasswordPolicy
	delivery  ports.SchoolInvitationDelivery
	runtime   ports.Runtime
	expiry    time.Duration
	now       func() time.Time
	logger    *slog.Logger
}

// SchoolInvitationDependencies are the ports the flows consume.
type SchoolInvitationDependencies struct {
	Store     ports.SchoolInvitationStore
	Logins    ports.AccountLoginStore
	Roles     ports.RoleStore
	Policy    ports.RoleAssignmentPolicy
	Grants    ports.InvitationGrantPolicy
	Identity  ports.SchoolIdentity
	Schools   ports.SchoolDirectory
	Owners    ports.InvitationOwnerTokens
	Passwords ports.PasswordPolicy
	Delivery  ports.SchoolInvitationDelivery
	Runtime   ports.Runtime
	Expiry    time.Duration
	Logger    *slog.Logger
}

func NewSchoolInvitation(deps SchoolInvitationDependencies) (*SchoolInvitation, error) {
	switch {
	case deps.Store == nil, deps.Logins == nil, deps.Roles == nil:
		return nil, fmt.Errorf("identity access school invitation: stores are required")
	case deps.Policy == nil, deps.Grants == nil, deps.Identity == nil, deps.Schools == nil:
		return nil, fmt.Errorf("identity access school invitation: role policy, grant policy, school identity and school directory are required")
	case deps.Owners == nil, deps.Passwords == nil, deps.Delivery == nil, deps.Runtime == nil:
		return nil, fmt.Errorf("identity access school invitation: owner tokens, password policy, delivery and tenant runtime are required")
	case deps.Expiry <= 0:
		return nil, fmt.Errorf("identity access school invitation: a positive link expiry is required")
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &SchoolInvitation{
		store: deps.Store, logins: deps.Logins, roles: deps.Roles, policy: deps.Policy, grants: deps.Grants,
		identity: deps.Identity, schools: deps.Schools, owners: deps.Owners, passwords: deps.Passwords,
		delivery: deps.Delivery, runtime: deps.Runtime, expiry: deps.Expiry, now: time.Now, logger: logger,
	}, nil
}

// CreateInvitation stores the invitation and queues its mail. The mail is
// queued for after the caller's transaction commits: the staff import
// creates invitations mid-transaction, and a rolled-back link must never
// reach an inbox.
func (s *SchoolInvitation) CreateInvitation(ctx context.Context, request domain.SchoolInvitationRequest) (result domain.SchoolInvitation, err error) {
	email, role, err := s.validateRequest(ctx, request)
	if err != nil {
		return domain.SchoolInvitation{}, err
	}
	tenantID := request.TenantID
	if tenantID <= 0 {
		tenantID = s.runtime.TenantID(ctx)
	}
	invitation := domain.SchoolInvitation{
		TenantID: tenantID, Email: email, Token: uuid.Must(uuid.NewV4()).String(), RoleID: request.RoleID,
		RoleName: role.Name, ExpiresAt: s.now().Add(s.expiry), FirstName: trimmed(request.FirstName),
		LastName: trimmed(request.LastName), Position: trimmed(request.Position),
		CaregiverEnabled: request.CaregiverEnabled, PersonID: request.PersonID,
	}
	if request.CreatedBy > 0 {
		creator := request.CreatedBy
		invitation.CreatedBy = &creator
	}
	if validateErr := invitation.Validate(s.now()); validateErr != nil {
		return domain.SchoolInvitation{}, failed(opCreateInvitation, validateErr)
	}
	// Spending the address's previous invitations and storing the new one
	// commit together, so a failed insert never burns the last usable link.
	err = s.runtime.RunInTx(s.runtime.WithTenantID(ctx, tenantID), func(txCtx context.Context) error {
		if _, _, revokeErr := s.store.RevokeSchoolInvitationsForEmail(txCtx, email); revokeErr != nil {
			return failed("invalidate invitations", revokeErr)
		}
		stored, _, insertErr := s.store.InsertSchoolInvitation(txCtx, invitation)
		if insertErr != nil {
			return failed(opCreateInvitation, insertErr)
		}
		result = stored
		result.RoleName = role.Name
		return nil
	})
	if err != nil {
		return domain.SchoolInvitation{}, err
	}
	s.logger.Info("invitation created",
		slog.Any("created_by", invitation.CreatedBy),
		slog.String("email", result.Email))

	portal := s.portalOf(role)
	schoolName := request.SchoolName
	if schoolName == "" {
		schoolName = s.schoolName(ctx, result.TenantID)
	}
	s.runtime.RegisterAfterCommit(ctx, func() {
		// Detach drops the request transaction and its commit hooks so the
		// mail cannot join them. It also clears the tenant; put the
		// invitation's school back so Reply-To still resolves (#1936).
		dispatchCtx := s.runtime.Detach(ctx)
		if result.TenantID > 0 {
			dispatchCtx = s.runtime.WithTenantID(dispatchCtx, result.TenantID)
		}
		s.delivery.DispatchSchoolInvitation(dispatchCtx, result, schoolName, portal, s.expiry)
	})
	return result, nil
}

// validateRequest checks the address and the role the request asks for.
func (s *SchoolInvitation) validateRequest(ctx context.Context, request domain.SchoolInvitationRequest) (string, domain.ManagedRole, error) {
	email := strings.TrimSpace(strings.ToLower(request.Email))
	switch {
	case email == "":
		return "", domain.ManagedRole{}, failed(opCreateInvitation, fmt.Errorf("email is required"))
	case len(email) > maxInvitationEmailLength:
		return "", domain.ManagedRole{}, failed(opCreateInvitation, fmt.Errorf("email address too long"))
	case request.RoleID <= 0:
		return "", domain.ManagedRole{}, failed(opCreateInvitation, fmt.Errorf("role id is required"))
	case request.CreatedBy < 0:
		return "", domain.ManagedRole{}, failed(opCreateInvitation, fmt.Errorf("created_by is invalid"))
	case request.TenantID < 0:
		return "", domain.ManagedRole{}, failed(opCreateInvitation, fmt.Errorf("tenant id is invalid"))
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil {
		return "", domain.ManagedRole{}, failed(opCreateInvitation, fmt.Errorf("invalid email address"))
	}
	email = parsed.Address

	if err := s.ensureTargetAllowed(ctx, email, request); err != nil {
		return "", domain.ManagedRole{}, err
	}
	role, err := s.ensureRoleAssignable(ctx, request)
	if err != nil {
		return "", domain.ManagedRole{}, err
	}
	return email, role, nil
}

// ensureTargetAllowed refuses an address that already signs in at the school
// the invitation is for.
func (s *SchoolInvitation) ensureTargetAllowed(ctx context.Context, email string, request domain.SchoolInvitationRequest) error {
	account, found, _, err := s.logins.FindLoginAccountByEmail(ctx, email)
	if err != nil {
		return failed(opCreateInvitation, err)
	}
	if !found {
		return nil
	}
	tenantID := request.TenantID
	if tenantID <= 0 {
		tenantID = s.runtime.TenantID(ctx)
	}
	if tenantID <= 0 {
		return failed(opCreateInvitation, domain.ErrEmailAlreadyExists)
	}
	member, _, err := s.logins.HasActiveAccountTenant(ctx, account.ID, tenantID)
	if err != nil {
		return failed(opCreateInvitation, err)
	}
	if member {
		return failed(opCreateInvitation, domain.ErrAccountAlreadyHasTenantAccess)
	}
	return nil
}

// ensureRoleAssignable verifies that the role may be handed out for this
// school at all, and that the inviting account may hand it out.
func (s *SchoolInvitation) ensureRoleAssignable(ctx context.Context, request domain.SchoolInvitationRequest) (domain.ManagedRole, error) {
	tenantID := request.TenantID
	if tenantID <= 0 {
		tenantID = s.runtime.TenantID(ctx)
	}
	role, found, err := s.roles.FindRoleIgnoringTenant(ctx, request.RoleID)
	if err != nil {
		return domain.ManagedRole{}, failed(opCreateInvitation, err)
	}
	if !found {
		// The policy owns the answer for a role that does not exist.
		return domain.ManagedRole{}, failed(opCreateInvitation, s.policy.ValidateAssignableSchoolRole(nil, tenantID))
	}
	facts := *role.Facts()
	if err := s.policy.ValidateAssignableSchoolRole(&facts, tenantID); err != nil {
		return domain.ManagedRole{}, failed(opCreateInvitation, err)
	}
	// The caregiver upgrade would hand a Lehrkraft the full user role plus a
	// caregiver profile; the combination is refused before a link exists
	// (#1772), for operator-issued invitations too.
	if request.CaregiverEnabled && s.policy.IsLehrkraftSystemRole(&facts) {
		return domain.ManagedRole{}, failed(opCreateInvitation, domain.ErrLehrkraftNoCaregiver)
	}
	if request.OperatorGrant {
		return role, nil
	}
	permissions, err := s.roles.ListRolePermissions(ctx, role.ID)
	if err != nil {
		return domain.ManagedRole{}, failed(opCreateInvitation, err)
	}
	names := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		names = append(names, permission.Name)
	}
	if !s.grants.CanGrantRole(facts, request.ActorPermissions, names) {
		s.logger.Warn("invitation role grant denied",
			slog.Int64("created_by", request.CreatedBy),
			slog.Int64("role_id", request.RoleID),
			slog.Int64("tenant_id", tenantID))
		return domain.ManagedRole{}, failed(opCreateInvitation, domain.ErrRoleGrantNotPermitted)
	}
	return role, nil
}

// ValidateInvitation returns the public details of a redeemable invitation.
func (s *SchoolInvitation) ValidateInvitation(ctx context.Context, token string) (result domain.InvitationPreview, err error) {
	err = s.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		invitation, fetchErr := s.redeemable(txCtx, token)
		if fetchErr != nil {
			return fetchErr
		}
		role, found, roleErr := s.roles.FindRoleIgnoringTenant(txCtx, invitation.RoleID)
		if roleErr != nil {
			return failed("lookup role", roleErr)
		}
		if !found {
			return failed("lookup role", domain.ErrRoleNotFound)
		}
		_, hasAccount, _, accountErr := s.logins.FindLoginAccountByEmail(txCtx, invitation.Email)
		if accountErr != nil {
			return failed(opFetchInvitation, accountErr)
		}
		result = domain.InvitationPreview{
			Portal: s.portalOf(role), RequiresAccountLogin: hasAccount, Email: invitation.Email, RoleName: role.Name,
			FirstName: invitation.FirstName, LastName: invitation.LastName, Position: invitation.Position,
			CaregiverEnabled: invitation.CaregiverEnabled, ExpiresAt: invitation.ExpiresAt,
		}
		return nil
	})
	return result, err
}

// AcceptInvitation spends the invitation and gives the invitee access: the
// account (created or the invitee's existing one), its school mapping, the
// role and the identity chain the role requires commit together.
func (s *SchoolInvitation) AcceptInvitation(ctx context.Context, token string, registration domain.InvitationRegistration) (result domain.LoginAccount, err error) {
	err = s.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		invitation, fetchErr := s.redeemable(txCtx, token)
		if fetchErr != nil {
			return fetchErr
		}
		// A shared lock on the school serializes with the exclusive lock a
		// soft delete takes, so the school cannot vanish under the acceptance.
		if invitation.TenantID > 0 {
			school, found, schoolErr := s.schools.LockSchoolShared(txCtx, invitation.TenantID)
			if schoolErr != nil {
				return failed(opAcceptInvitation, schoolErr)
			}
			if !found || school.Deleted {
				return failed(opAcceptInvitation, domain.ErrInvitationTenantDeleted)
			}
		}
		invitationCtx := s.runtime.WithTenantID(txCtx, invitation.TenantID)
		account, hasAccount, _, accountErr := s.logins.FindLoginAccountByEmail(invitationCtx, invitation.Email)
		if accountErr != nil {
			return failed(opAcceptInvitation, accountErr)
		}
		var passwordHash string
		if hasAccount {
			if ownerErr := s.verifyOwner(account, registration.OwnerAccessToken); ownerErr != nil {
				return failed(opAcceptInvitation, ownerErr)
			}
		} else {
			hash, hashErr := s.hashPassword(registration)
			if hashErr != nil {
				return hashErr
			}
			passwordHash = hash
		}
		firstName, lastName, nameErr := registration.Names(invitation)
		if nameErr != nil {
			return failed(opAcceptInvitation, nameErr)
		}
		created, grantErr := s.grantAccess(invitationCtx, invitation, account, hasAccount, passwordHash, firstName, lastName)
		if grantErr != nil {
			return grantErr
		}
		result = created
		return nil
	})
	if err != nil {
		return domain.LoginAccount{}, err
	}
	s.logger.Info("invitation accepted",
		slog.Int64("account_id", result.ID))
	return result, nil
}

func (s *SchoolInvitation) hashPassword(registration domain.InvitationRegistration) (string, error) {
	if registration.Password != registration.ConfirmPassword {
		return "", failed(opAcceptInvitation, domain.ErrPasswordMismatch)
	}
	if err := s.passwords.ValidatePasswordStrength(registration.Password); err != nil {
		return "", failed(opAcceptInvitation, err)
	}
	hash, err := s.passwords.HashPassword(registration.Password)
	if err != nil {
		return "", failed(opAcceptInvitation, err)
	}
	return hash, nil
}

// verifyOwner refuses an acceptance for an existing account unless the
// caller proved they hold that account's session: an invitation grants
// membership, never authority over someone else's credentials.
func (s *SchoolInvitation) verifyOwner(account domain.LoginAccount, accessToken string) error {
	if accessToken == "" {
		return domain.ErrInvitationOwnerRequired
	}
	ownerID, err := s.owners.AccountOfAccessToken(accessToken)
	if err != nil || ownerID <= 0 {
		return domain.ErrInvitationOwnerRequired
	}
	if ownerID != account.ID {
		return domain.ErrInvitationOwnerMismatch
	}
	if !account.Active {
		return domain.ErrAccountInactive
	}
	return nil
}

// grantAccess creates or reuses the account and gives it the school access
// the invitation promises.
func (s *SchoolInvitation) grantAccess(
	ctx context.Context,
	invitation domain.SchoolInvitation,
	existing domain.LoginAccount,
	hasAccount bool,
	passwordHash, firstName, lastName string,
) (domain.LoginAccount, error) {
	account := existing
	if !hasAccount {
		created, _, err := s.store.InsertAccount(ctx, invitation.Email, passwordHash)
		if err != nil {
			return domain.LoginAccount{}, failed("create account", err)
		}
		account = created
	}
	if _, err := s.store.EnsureAccountTenant(ctx, account.ID, invitation.TenantID); err != nil {
		return domain.LoginAccount{}, failed("create account-tenant mapping", err)
	}
	if err := s.roles.CreateAccountRole(ctx, account.ID, invitation.RoleID, invitation.TenantID); err != nil {
		return domain.LoginAccount{}, failed("assign role", err)
	}
	role, found, err := s.roles.FindRoleIgnoringTenant(ctx, invitation.RoleID)
	if err != nil {
		return domain.LoginAccount{}, failed("provision school identity", err)
	}
	if !found {
		return domain.LoginAccount{}, failed("provision school identity", domain.ErrRoleNotFound)
	}
	facts := *role.Facts()
	lehrkraft := s.policy.IsLehrkraftSystemRole(&facts)
	if err := s.assignCaregiverRole(ctx, account.ID, invitation, facts, lehrkraft); err != nil {
		return domain.LoginAccount{}, err
	}
	var position string
	if invitation.Position != nil {
		position = *invitation.Position
	}
	if _, err := s.identity.EnsureSchoolIdentity(ctx, domain.SchoolIdentityInput{
		AccountID: account.ID, TenantID: invitation.TenantID, Role: &facts,
		FirstName: firstName, LastName: lastName, Position: position,
		// The staff import created the person before the invitation went out;
		// accepting reuses it instead of filing a second one (#2600).
		PersonID: invitation.PersonID,
		// A Lehrkraft never gets a caregiver profile, caregiver_enabled or
		// not — the same invariant the role assignment applies (#1772).
		CaregiverUpgrade: invitation.CaregiverEnabled && !lehrkraft,
		CreatePerson:     true,
	}); err != nil {
		return domain.LoginAccount{}, failed("provision school identity", err)
	}
	redeemed, _, err := s.store.SpendSchoolInvitation(ctx, invitation.ID)
	if err != nil {
		return domain.LoginAccount{}, failed("mark invitation used", err)
	}
	if !redeemed {
		return domain.LoginAccount{}, failed(opAcceptInvitation, domain.ErrInvitationUsed)
	}
	return account, nil
}

// assignCaregiverRole grants the user role a caregiver upgrade needs; a
// platform caregiver role and the Lehrkraft role never receive it.
func (s *SchoolInvitation) assignCaregiverRole(ctx context.Context, accountID int64, invitation domain.SchoolInvitation, facts domain.RoleFacts, lehrkraft bool) error {
	if !invitation.CaregiverEnabled || lehrkraft || s.policy.IsPlatformCaregiverRole(&facts) {
		return nil
	}
	userRole, found, err := s.roles.FindSystemRoleByName(ctx, baseRoleUser)
	if err != nil {
		return failed("assign caregiver role", err)
	}
	if !found {
		return failed("assign caregiver role", fmt.Errorf("user role not found"))
	}
	if err := s.roles.CreateAccountRole(ctx, accountID, userRole.ID, invitation.TenantID); err != nil {
		return failed("assign caregiver role", err)
	}
	return nil
}

// ResendInvitation extends a still redeemable invitation and queues its mail
// again; an expired or spent invitation is never revived.
func (s *SchoolInvitation) ResendInvitation(ctx context.Context, invitationID, actorAccountID int64) error {
	invitation, found, _, err := s.store.FindSchoolInvitation(ctx, invitationID)
	if err != nil {
		return failed(opResendInvitation, err)
	}
	if !found {
		return failed(opResendInvitation, domain.ErrInvitationNotFound)
	}
	if invitation.UsedAt != nil {
		return failed(opResendInvitation, domain.ErrInvitationUsed)
	}
	now := s.now()
	if !invitation.ExpiresAt.After(now) {
		return failed(opResendInvitation, domain.ErrInvitationExpired)
	}
	role, roleFound, err := s.roles.FindRoleIgnoringTenant(ctx, invitation.RoleID)
	if err != nil {
		return failed("lookup role", err)
	}
	if !roleFound {
		return failed("lookup role", domain.ErrRoleNotFound)
	}
	extended, _, err := s.store.ExtendSchoolInvitation(ctx, invitationID, now.Add(s.expiry), now)
	if err != nil {
		return failed(opResendInvitation, err)
	}
	if !extended {
		return failed(opResendInvitation, domain.ErrInvitationNotFound)
	}
	invitation.ExpiresAt = now.Add(s.expiry)
	// The delivery bookkeeping starts over; the retry count of the previous
	// attempts is not carried into the new send.
	if _, err := s.store.RecordSchoolInvitationDelivery(ctx, invitationID, domain.TokenDelivery{RetryCount: invitation.Delivery.RetryCount}); err != nil {
		return failed(opResendInvitation, err)
	}
	invitation.Delivery = domain.TokenDelivery{RetryCount: invitation.Delivery.RetryCount}
	invitation.RoleName = role.Name
	s.logger.Info("invitation resent",
		slog.Int64("invitation_id", invitation.ID),
		slog.Int64("actor_account_id", actorAccountID))
	s.delivery.DispatchSchoolInvitation(ctx, invitation, s.schoolName(ctx, invitation.TenantID), s.portalOf(role), s.expiry)
	return nil
}

// ListPendingInvitations returns the redeemable invitations of the school in
// context.
func (s *SchoolInvitation) ListPendingInvitations(ctx context.Context) ([]domain.SchoolInvitation, error) {
	invitations, _, err := s.store.ListRedeemableSchoolInvitations(ctx, s.now())
	if err != nil {
		return nil, failed("list invitations", err)
	}
	return invitations, nil
}

// RevokeInvitation spends an invitation so it can no longer be accepted.
func (s *SchoolInvitation) RevokeInvitation(ctx context.Context, invitationID, actorAccountID int64) error {
	invitation, found, _, err := s.store.FindSchoolInvitation(ctx, invitationID)
	if err != nil {
		return failed(opRevokeInvitation, err)
	}
	if !found {
		return failed(opRevokeInvitation, domain.ErrInvitationNotFound)
	}
	if invitation.UsedAt != nil {
		return failed(opRevokeInvitation, domain.ErrInvitationUsed)
	}
	revoked, _, err := s.store.SpendSchoolInvitation(ctx, invitationID)
	if err != nil {
		return failed(opRevokeInvitation, err)
	}
	if !revoked {
		return failed(opRevokeInvitation, domain.ErrInvitationUsed)
	}
	s.logger.Info("invitation revoked",
		slog.Int64("invitation_id", invitationID),
		slog.Int64("actor_account_id", actorAccountID))
	return nil
}

// RecordInvitationDelivery stores the outcome of mailing an invitation.
func (s *SchoolInvitation) RecordInvitationDelivery(ctx context.Context, id int64, delivery domain.TokenDelivery) error {
	_, err := s.store.RecordSchoolInvitationDelivery(ctx, id, delivery.Bounded())
	return err
}

// InvitationSubdomain resolves the school subdomain an accepted invitation
// belongs to; tenant routing resolves hosts by subdomain, not slug (#1977).
// Best-effort: an error or a deleted school yields "".
func (s *SchoolInvitation) InvitationSubdomain(ctx context.Context, token string) string {
	var subdomain string
	_ = s.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		invitation, found, _, err := s.store.FindSchoolInvitationByToken(txCtx, token)
		if err != nil || !found {
			return err
		}
		school, schoolFound, err := s.schools.FindSchool(txCtx, invitation.TenantID)
		if err != nil || !schoolFound || school.Deleted {
			return err
		}
		subdomain = school.Subdomain
		return nil
	})
	return subdomain
}

// redeemable resolves an invitation that can still be accepted and reports
// why it cannot otherwise.
func (s *SchoolInvitation) redeemable(ctx context.Context, token string) (domain.SchoolInvitation, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return domain.SchoolInvitation{}, failed(opFetchInvitation, domain.ErrInvitationNotFound)
	}
	invitation, found, _, err := s.store.FindSchoolInvitationByToken(ctx, token)
	if err != nil {
		return domain.SchoolInvitation{}, failed(opFetchInvitation, err)
	}
	if !found {
		return domain.SchoolInvitation{}, failed(opFetchInvitation, domain.ErrInvitationNotFound)
	}
	if invitation.UsedAt != nil {
		return domain.SchoolInvitation{}, failed(opFetchInvitation, domain.ErrInvitationUsed)
	}
	if !invitation.ExpiresAt.After(s.now()) {
		return domain.SchoolInvitation{}, failed(opFetchInvitation, domain.ErrInvitationExpired)
	}
	return invitation, nil
}

// portalOf decides where the invitee accepts: a school-portal role accepts
// on the school portal, because that is where its login lives (#2207).
func (s *SchoolInvitation) portalOf(role domain.ManagedRole) domain.InvitationPortal {
	if s.policy.IsLehrkraftSystemRole(role.Facts()) {
		return domain.InvitationPortalSchool
	}
	return domain.InvitationPortalTenant
}

// schoolName resolves the school's display name for the mail; it is
// best-effort and never blocks the flow.
func (s *SchoolInvitation) schoolName(ctx context.Context, tenantID int64) string {
	if tenantID <= 0 {
		return ""
	}
	school, found, err := s.schools.FindSchool(ctx, tenantID)
	if err != nil {
		s.logger.Warn("failed to look up school name for invitation email",
			slog.Int64("tenant_id", tenantID),
			slog.Any("error", err))
		return ""
	}
	if !found || school.Deleted {
		return ""
	}
	return school.Name
}

func trimmed(value *string) *string {
	if value == nil {
		return nil
	}
	text := strings.TrimSpace(*value)
	return &text
}
