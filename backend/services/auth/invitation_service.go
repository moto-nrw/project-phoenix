package auth

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/email"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Operation name constants for AuthError.
const (
	opCreateInvitation = "create invitation"
	opAcceptInvitation = "accept invitation"
	opResendInvitation = "resend invitation"
	opRevokeInvitation = "revoke invitation"
	opFetchInvitation  = "fetch invitation"
)

// systemRoleTranslations maps English system role names to German display names.
// Used for user-facing content like emails.
var systemRoleTranslations = map[string]string{
	"admin":     "Administrator",
	"user":      "Betreuer",
	"guest":     "Gast",
	"guardian":  "Erziehungsberechtigter",
	"lehrkraft": "Lehrkraft",
}

// translateRoleNameToGerman translates system role names to German.
// Falls back to the original name if no translation exists.
func translateRoleNameToGerman(roleName string) string {
	if translated, ok := systemRoleTranslations[strings.ToLower(roleName)]; ok {
		return translated
	}
	return roleName
}

// InvitationServiceConfig holds configuration for the invitation service
type InvitationServiceConfig struct {
	InvitationRepo    authModels.InvitationTokenRepository
	AccountRepo       authModels.AccountRepository
	AccountTenantRepo authModels.AccountTenantRepository
	RoleRepo          authModels.RoleRepository
	PermissionRepo    authModels.PermissionRepository
	AccountRoleRepo   authModels.AccountRoleRepository
	PersonRepo        userModels.PersonRepository
	StaffRepo         userModels.StaffRepository
	TeacherRepo       userModels.TeacherRepository
	StudentRepo       userModels.StudentRepository
	SchoolRepo        platformModels.SchoolRepository
	Mailer            email.Mailer
	Dispatcher        *email.Dispatcher
	FrontendURL       string
	// SchoolURL is the school-portal base URL (#2207). Invitations for a
	// school-portal role (today: lehrkraft) link there instead of the staff
	// frontend, because that is where the accept flow for those accounts
	// lives.
	SchoolURL        string
	DefaultFrom      email.Email
	InvitationExpiry time.Duration
	// MailIdentity points replies to a staff invitation at the OGS instead of
	// moto (#1936). This send bypasses the outbox, so it stamps the header
	// itself. Optional: nil sends without a Reply-To, exactly as before.
	MailIdentity email.ReplyToResolver
	DB           *bun.DB
	Logger       *slog.Logger
}

type invitationService struct {
	invitationRepo    authModels.InvitationTokenRepository
	accountRepo       authModels.AccountRepository
	accountTenantRepo authModels.AccountTenantRepository
	roleRepo          authModels.RoleRepository
	permissionRepo    authModels.PermissionRepository
	accountRoleRepo   authModels.AccountRoleRepository
	personRepo        userModels.PersonRepository
	staffRepo         userModels.StaffRepository
	teacherRepo       userModels.TeacherRepository
	studentRepo       userModels.StudentRepository
	schoolRepo        platformModels.SchoolRepository
	dispatcher        *email.Dispatcher
	frontendURL       string
	schoolURL         string
	defaultFrom       email.Email
	invitationExpiry  time.Duration
	mailIdentity      email.ReplyToResolver
	db                *bun.DB
	logger            *slog.Logger
	tenantRuntime     *tenant.UnitOfWork
}

// getLogger returns the service's logger, falling back to slog.Default() if nil.
func (s *invitationService) getLogger() *slog.Logger {
	return cmp.Or(s.logger, slog.Default())
}

// NewInvitationService constructs a new invitation service instance.
func NewInvitationService(config InvitationServiceConfig) InvitationService {
	trimmedFrontend := strings.TrimRight(strings.TrimSpace(config.FrontendURL), "/")
	dispatcher := config.Dispatcher
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if dispatcher == nil && config.Mailer != nil {
		dispatcher = email.NewDispatcher(config.Mailer, logger.With("component", "email"))
	}
	return &invitationService{
		invitationRepo:    config.InvitationRepo,
		accountRepo:       config.AccountRepo,
		accountTenantRepo: config.AccountTenantRepo,
		roleRepo:          config.RoleRepo,
		permissionRepo:    config.PermissionRepo,
		accountRoleRepo:   config.AccountRoleRepo,
		personRepo:        config.PersonRepo,
		staffRepo:         config.StaffRepo,
		teacherRepo:       config.TeacherRepo,
		studentRepo:       config.StudentRepo,
		schoolRepo:        config.SchoolRepo,
		dispatcher:        dispatcher,
		frontendURL:       trimmedFrontend,
		schoolURL:         strings.TrimRight(strings.TrimSpace(config.SchoolURL), "/"),
		defaultFrom:       config.DefaultFrom,
		mailIdentity:      config.MailIdentity,
		invitationExpiry:  config.InvitationExpiry,
		db:                config.DB,
		logger:            logger,
	}
}

func (s *invitationService) withTenantRuntime(ctx context.Context) context.Context {
	if s.tenantRuntime == nil {
		return ctx
	}
	return tenant.WithUnitOfWork(ctx, *s.tenantRuntime)
}

func (s *invitationService) SetTenantRuntime(runtime tenant.UnitOfWork) {
	s.tenantRuntime = &runtime
}

// CreateInvitation creates an invitation token and queues the invitation email.
func (s *invitationService) CreateInvitation(ctx context.Context, req InvitationRequest) (*authModels.InvitationToken, error) {
	emailAddress, err := s.validateInvitationRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	invalidationCtx := scopedInvitationTenantContext(ctx, req.TenantID)
	if err := s.invalidatePreviousInvitations(invalidationCtx, emailAddress); err != nil {
		return nil, err
	}

	invitation := s.buildInvitationToken(emailAddress, req)
	tenantID := req.TenantID
	if tenantID <= 0 {
		tenantID = tenant.FromContext(ctx)
	}
	invitation.SetTenantID(tenantID)
	if err := s.invitationRepo.Create(ctx, invitation); err != nil {
		return nil, &AuthError{Op: opCreateInvitation, Err: err}
	}

	s.getLogger().Info("invitation created",
		slog.Any("created_by", nullableCreatedBy(req.CreatedBy)),
		slog.String("email", invitation.Email))

	role, err := s.attachRoleAndCreator(ctx, invitation)
	if err != nil {
		return nil, err
	}

	roleName := ""
	if invitation.Role != nil {
		roleName = invitation.Role.Name
	}
	// A school-portal invitation must link to the school portal, not the
	// staff frontend — decided here, before the send is queued (#2207).
	schoolPortal := isSchoolPortalRole(role)
	// Queue the email until the surrounding tenant transaction commits: the
	// staff import creates invitations mid-transaction, and a rolled-back
	// token must never reach an inbox as a dead link. Outside a tenant tx
	// the hook runs synchronously.
	tenant.RegisterAfterCommit(ctx, func() {
		// The surrounding transaction is committed when this callback runs.
		// Detach it before resolving the tenant mail identity so its settings
		// lookup opens a fresh RLS transaction rather than using the closed one.
		postCommitCtx := detachedTenantContext(s.withTenantRuntime(ctx))
		s.sendInvitationEmail(postCommitCtx, invitation, roleName, req.SchoolName, schoolPortal)
	})

	return invitation, nil
}

// ensureRoleAssignable verifies that the requested role may be handed out for
// this school at all, and that the inviting account is allowed to grant it.
// Both halves matter: the first keeps guardian and retired roles out of the
// staff flow, the second stops an account from inviting someone into a role
// more powerful than its own.
func (s *invitationService) ensureRoleAssignable(ctx context.Context, req InvitationRequest) error {
	tenantID := req.TenantID
	if tenantID <= 0 {
		tenantID = tenant.FromContext(ctx)
	}

	role, err := ValidateAssignableSchoolRole(ctx, s.roleRepo, req.RoleID, tenantID)
	if err != nil {
		return &AuthError{Op: opCreateInvitation, Err: err}
	}

	// The caregiver upgrade would hand a Lehrkraft the full user role plus a
	// caregiver profile — refuse the combination at creation, before any
	// token exists (#1772). Checked before the OperatorGrant early-return:
	// the invariant holds for operator-created invitations too.
	if req.CaregiverEnabled && IsLehrkraftSystemRole(role) {
		return &AuthError{Op: opCreateInvitation, Err: ErrLehrkraftNoCaregiver}
	}

	if req.OperatorGrant {
		return nil
	}
	if s.permissionRepo != nil {
		role.Permissions, err = s.permissionRepo.FindByRoleID(ctx, role.ID)
		if err != nil {
			return &AuthError{Op: opCreateInvitation, Err: err}
		}
	}

	if !authorize.CanGrantRole(role, req.ActorPermissions) {
		s.getLogger().Warn("invitation role grant denied",
			slog.Any("created_by", nullableCreatedBy(req.CreatedBy)),
			slog.Int64("role_id", req.RoleID),
			slog.Int64("tenant_id", tenantID))
		return &AuthError{Op: opCreateInvitation, Err: ErrRoleGrantNotPermitted}
	}

	return nil
}

// invalidatePreviousInvitations marks any pending invitations for this email as used.
func (s *invitationService) invalidatePreviousInvitations(ctx context.Context, email string) error {
	_, err := s.invitationRepo.InvalidateByEmail(ctx, email)
	if err != nil {
		return &AuthError{Op: "invalidate invitations", Err: err}
	}
	return nil
}

// buildInvitationToken constructs the invitation token with optional fields.
func (s *invitationService) buildInvitationToken(email string, req InvitationRequest) *authModels.InvitationToken {
	invitation := &authModels.InvitationToken{
		Email:            email,
		Token:            uuid.Must(uuid.NewV4()).String(),
		RoleID:           req.RoleID,
		ExpiresAt:        time.Now().Add(s.invitationExpiry),
		CaregiverEnabled: req.CaregiverEnabled,
		PersonID:         req.PersonID,
	}
	if req.CreatedBy > 0 {
		invitation.CreatedBy = nullableCreatedBy(req.CreatedBy)
	}

	if req.FirstName != nil {
		firstName := strings.TrimSpace(*req.FirstName)
		invitation.FirstName = &firstName
	}
	if req.LastName != nil {
		lastName := strings.TrimSpace(*req.LastName)
		invitation.LastName = &lastName
	}
	if req.Position != nil {
		position := strings.TrimSpace(*req.Position)
		invitation.Position = &position
	}

	return invitation
}

// attachRoleAndCreator populates the Role and Creator fields on the invitation.
// Returns the full resolved role (nil when the lookup failed) so callers can
// branch on role properties — e.g. the school-portal link decision — without a
// second lookup; the invitation itself keeps carrying only ID + Name.
func (s *invitationService) attachRoleAndCreator(ctx context.Context, invitation *authModels.InvitationToken) (*authModels.Role, error) {
	role, _ := s.lookupRole(ctx, invitation.RoleID)
	if role != nil && role.Name != "" {
		invitation.Role = &authModels.Role{
			Model: modelBase.Model{ID: invitation.RoleID},
			Name:  role.Name,
		}
	}

	if invitation.CreatedBy == nil {
		return role, nil
	}

	creator, err := s.accountRepo.FindByID(ctx, *invitation.CreatedBy)
	if err != nil && !isNotFoundError(err) {
		return role, &AuthError{Op: "lookup creator", Err: err}
	}
	if creator != nil {
		invitation.Creator = &authModels.Account{
			Model: modelBase.Model{ID: creator.ID},
			Email: creator.Email,
		}
	}

	return role, nil
}

// ValidateInvitation returns the public details for a token if it is still usable.
func (s *invitationService) ValidateInvitation(ctx context.Context, token string) (*InvitationValidationResult, error) {
	ctx = s.withTenantRuntime(ctx)
	var result *InvitationValidationResult
	err := tenant.WithAdminTxOrDirect(ctx, s.db, func(adminCtx context.Context) error {
		invitation, fetchErr := s.fetchValidInvitation(adminCtx, token)
		if fetchErr != nil {
			return fetchErr
		}

		roleName, roleErr := s.lookupRoleName(adminCtx, invitation.RoleID)
		if roleErr != nil {
			return roleErr
		}

		result = &InvitationValidationResult{
			Email:            invitation.Email,
			RoleName:         roleName,
			FirstName:        invitation.FirstName,
			LastName:         invitation.LastName,
			Position:         invitation.Position,
			CaregiverEnabled: invitation.CaregiverEnabled,
			ExpiresAt:        invitation.ExpiresAt,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// AcceptInvitation converts a token into a real account & person record.
func (s *invitationService) AcceptInvitation(ctx context.Context, token string, userData UserRegistrationData) (*authModels.Account, error) {
	ctx = s.withTenantRuntime(ctx)
	var createdAccount *authModels.Account
	err := tenant.WithAdminTxOrDirect(ctx, s.db, func(adminCtx context.Context) error {
		invitation, fetchErr := s.fetchValidInvitation(adminCtx, token)
		if fetchErr != nil {
			return fetchErr
		}

		// Reject invitations for soft-deleted schools. Uses FindByIDForShare to
		// acquire a shared lock on the school row, which serializes with the
		// exclusive lock taken by SoftDeleteSchool's UPDATE. This prevents the
		// race where AcceptInvitation reads the school as active, then
		// SoftDeleteSchool commits the deletion before the account is created.
		// The lock is held until this admin transaction commits.
		if invitation.TenantID > 0 && s.schoolRepo != nil {
			school, schoolErr := s.schoolRepo.FindByIDForShare(adminCtx, invitation.TenantID)
			if schoolErr != nil {
				return &AuthError{Op: opAcceptInvitation, Err: schoolErr}
			}
			if school == nil || school.IsDeleted() {
				return &AuthError{Op: opAcceptInvitation, Err: ErrInvitationTenantDeleted}
			}
		}

		passwordHash, hashErr := s.validateAndHashPassword(userData)
		if hashErr != nil {
			return hashErr
		}

		firstName, lastName, nameErr := s.resolveNames(userData, invitation)
		if nameErr != nil {
			return nameErr
		}

		invitationTenantID, tenantErr := tenant.NewTenantID(invitation.TenantID)
		if tenantErr != nil {
			return &AuthError{Op: opAcceptInvitation, Err: tenantErr}
		}
		invitationCtx := tenant.WithTenant(adminCtx, invitationTenantID)
		account, accountErr := s.findExistingAccountByEmail(invitationCtx, invitation.Email)
		if accountErr != nil {
			return &AuthError{Op: opAcceptInvitation, Err: accountErr}
		}
		created, txErr := s.createAccountWithRole(invitationCtx, invitation, passwordHash, firstName, lastName, account)
		if txErr != nil {
			return txErr
		}
		createdAccount = created
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.getLogger().Info("invitation accepted",
		slog.Int64("account_id", createdAccount.ID))
	return createdAccount, nil
}

func (s *invitationService) findExistingAccountByEmail(ctx context.Context, email string) (*authModels.Account, error) {
	account, err := s.accountRepo.FindByEmail(ctx, email)
	if err == nil {
		return account, nil
	}
	if isNotFoundError(err) {
		return nil, nil
	}
	return nil, err
}

// validateAndHashPassword validates password match and strength, then returns the hash.
func (s *invitationService) validateAndHashPassword(userData UserRegistrationData) (string, error) {
	if userData.Password != userData.ConfirmPassword {
		return "", &AuthError{Op: opAcceptInvitation, Err: ErrPasswordMismatch}
	}

	if err := ValidatePasswordStrength(userData.Password); err != nil {
		return "", &AuthError{Op: opAcceptInvitation, Err: err}
	}

	passwordHash, err := HashPassword(userData.Password)
	if err != nil {
		return "", &AuthError{Op: opAcceptInvitation, Err: err}
	}

	return passwordHash, nil
}

// resolveNames resolves first and last name from user data or invitation fallback.
func (s *invitationService) resolveNames(userData UserRegistrationData, invitation *authModels.InvitationToken) (string, string, error) {
	firstName := strings.TrimSpace(userData.FirstName)
	lastName := strings.TrimSpace(userData.LastName)

	if firstName == "" && invitation.FirstName != nil {
		firstName = strings.TrimSpace(*invitation.FirstName)
	}
	if lastName == "" && invitation.LastName != nil {
		lastName = strings.TrimSpace(*invitation.LastName)
	}

	if firstName == "" || lastName == "" {
		return "", "", &AuthError{Op: opAcceptInvitation, Err: ErrInvitationNameRequired}
	}

	return firstName, lastName, nil
}

// createAccountWithRole creates person, account, role assignment, and optional staff/teacher records.
func (s *invitationService) createAccountWithRole(
	ctx context.Context,
	invitation *authModels.InvitationToken,
	passwordHash, firstName, lastName string,
	existingAccount *authModels.Account,
) (*authModels.Account, error) {
	account, err := s.createOrUpdateAccount(ctx, invitation.Email, passwordHash, existingAccount)
	if err != nil {
		return nil, err
	}

	if err := s.createAccountTenant(ctx, account.ID, invitation.TenantID); err != nil {
		return nil, err
	}

	if err := s.assignRole(ctx, account.ID, invitation.RoleID, invitation.TenantID); err != nil {
		return nil, err
	}

	if err := s.assignCaregiverRoleIfRequested(ctx, account.ID, invitation); err != nil {
		return nil, err
	}

	if err := s.createAccountTenant(ctx, account.ID, invitation.TenantID); err != nil {
		return nil, err
	}

	if err := s.provisionSchoolIdentity(ctx, account.ID, firstName, lastName, invitation); err != nil {
		return nil, err
	}

	if err := s.invitationRepo.MarkAsUsed(ctx, invitation.ID); err != nil {
		return nil, &AuthError{Op: "mark invitation used", Err: err}
	}

	return account, nil
}

func (s *invitationService) createOrUpdateAccount(ctx context.Context, email, passwordHash string, existingAccount *authModels.Account) (*authModels.Account, error) {
	if existingAccount == nil {
		return s.createAccount(ctx, email, passwordHash)
	}
	if err := s.accountRepo.UpdatePassword(ctx, existingAccount.ID, passwordHash); err != nil {
		return nil, &AuthError{Op: "update account password", Err: err}
	}
	// Reactivate the existing account so the invitee can log in. Targeted
	// SetActive so the stale in-memory PasswordHash doesn't overwrite the
	// just-written hash from UpdatePassword above.
	if !existingAccount.Active {
		if err := s.accountRepo.SetActive(ctx, existingAccount.ID, true); err != nil {
			return nil, &AuthError{Op: "reactivate account on invitation", Err: err}
		}
		existingAccount.Active = true
	}
	return existingAccount, nil
}

// validateInvitationRequest validates all required fields and returns the normalized email.
func (s *invitationService) validateInvitationRequest(ctx context.Context, req InvitationRequest) (string, error) {
	emailAddress := strings.TrimSpace(strings.ToLower(req.Email))
	if emailAddress == "" {
		return "", &AuthError{Op: opCreateInvitation, Err: fmt.Errorf("email is required")}
	}

	if _, err := mail.ParseAddress(emailAddress); err != nil {
		return "", &AuthError{Op: opCreateInvitation, Err: fmt.Errorf("invalid email address")}
	}

	if err := s.ensureInvitationTargetAllowed(ctx, emailAddress, req); err != nil {
		return "", err
	}

	if req.RoleID <= 0 {
		return "", &AuthError{Op: opCreateInvitation, Err: fmt.Errorf("role id is required")}
	}

	if req.CreatedBy < 0 {
		return "", &AuthError{Op: opCreateInvitation, Err: fmt.Errorf("created_by is invalid")}
	}
	if req.TenantID < 0 {
		return "", &AuthError{Op: opCreateInvitation, Err: fmt.Errorf("tenant id is invalid")}
	}

	if err := s.ensureRoleAssignable(ctx, req); err != nil {
		return "", err
	}

	return emailAddress, nil
}

func (s *invitationService) ensureInvitationTargetAllowed(ctx context.Context, email string, req InvitationRequest) error {
	account, err := s.findExistingAccountByEmail(ctx, email)
	if err != nil {
		return &AuthError{Op: opCreateInvitation, Err: err}
	}
	if account == nil {
		return nil
	}
	targetTenantID := req.TenantID
	if targetTenantID <= 0 {
		targetTenantID = tenant.FromContext(ctx)
	}
	if targetTenantID <= 0 {
		return &AuthError{Op: opCreateInvitation, Err: ErrEmailAlreadyExists}
	}
	exists, err := s.accountTenantRepo.ExistsByAccountAndTenant(ctx, account.ID, targetTenantID)
	if err != nil {
		return &AuthError{Op: opCreateInvitation, Err: err}
	}
	if exists {
		return &AuthError{Op: opCreateInvitation, Err: ErrAccountAlreadyHasTenantAccess}
	}
	return nil
}

// createAccount creates a new account record.
func (s *invitationService) createAccount(ctx context.Context, email, passwordHash string) (*authModels.Account, error) {
	account := &authModels.Account{
		Email:        email,
		Active:       true,
		PasswordHash: &passwordHash,
	}
	if err := s.accountRepo.Create(ctx, account); err != nil {
		return nil, &AuthError{Op: "create account", Err: err}
	}
	return account, nil
}

// assignRole assigns a role to an account with the given tenant ID.
// tenantID is passed explicitly because invitation acceptance is a public route
// where tenant.FromContext(ctx) would return 0.
func (s *invitationService) assignRole(ctx context.Context, accountID, roleID, tenantID int64) error {
	accountRole := &authModels.AccountRole{
		AccountID: accountID,
		RoleID:    roleID,
	}
	accountRole.SetTenantID(tenantID)
	if err := s.accountRoleRepo.Create(ctx, accountRole); err != nil {
		return &AuthError{Op: "assign role", Err: err}
	}
	return nil
}

// createAccountTenant maps an account to a tenant so the user can log into this school.
// Must be called within a RunInTx block (tx stored in context for base.GetDB).
// EnsureActive (not Create) so accepting a re-invitation after staff
// offboarding reactivates the deactivated mapping, matching the guardian
// invitation flow.
func (s *invitationService) createAccountTenant(ctx context.Context, accountID, tenantID int64) error {
	now := time.Now()
	mapping := &authModels.AccountTenant{
		AccountID:   accountID,
		TenantID:    tenantID,
		Status:      authModels.AccountTenantStatusActive,
		ActivatedAt: &now,
	}
	if err := s.accountTenantRepo.EnsureActive(ctx, mapping); err != nil {
		return &AuthError{Op: "create account-tenant mapping", Err: err}
	}
	return nil
}

func (s *invitationService) assignCaregiverRoleIfRequested(ctx context.Context, accountID int64, invitation *authModels.InvitationToken) error {
	if !invitation.CaregiverEnabled {
		return nil
	}

	role, err := s.roleRepo.FindByID(ctx, invitation.RoleID)
	if err != nil {
		return &AuthError{Op: "assign caregiver role", Err: err}
	}
	if role == nil {
		return &AuthError{Op: "assign caregiver role", Err: fmt.Errorf("role not found")}
	}
	if IsPlatformCaregiverRole(role) {
		return nil
	}
	// Defense in depth for tokens minted before the creation-side check
	// existed: a Lehrkraft invitation never receives the user role (#1772).
	if IsLehrkraftSystemRole(role) {
		return nil
	}

	userRole, err := ResolveSystemRoleByName(ctx, s.roleRepo, authModels.BaseRoleUser)
	if err != nil {
		return &AuthError{Op: "assign caregiver role", Err: err}
	}
	if userRole == nil {
		return &AuthError{Op: "assign caregiver role", Err: fmt.Errorf("user role not found")}
	}

	return s.assignRole(ctx, accountID, userRole.ID, invitation.TenantID)
}

// ResolveSystemRoleByName looks up the system (tenant-independent) role with
// the given name via repo, matching case-insensitively. Returns (nil, nil)
// when no system role matches. Shared across the invitation, caregiver, and
// operator-provisioning services (audit B7 — was three private copies).
func ResolveSystemRoleByName(ctx context.Context, repo authModels.RoleRepository, name string) (*authModels.Role, error) {
	roles, err := repo.List(ctx, map[string]interface{}{
		"name":      strings.TrimSpace(strings.ToLower(name)),
		"is_system": true,
	})
	if err != nil {
		return nil, err
	}
	for _, role := range roles {
		if role == nil {
			continue
		}
		if role.TenantID == nil && role.IsSystem && strings.EqualFold(role.Name, name) {
			return role, nil
		}
	}
	return nil, nil
}

// provisionSchoolIdentity gives the invited account the person, staff and (for
// caregiver roles) teacher records it needs to be usable at the school.
//
// This used to run only for platform roles, which left an account invited with
// a school's own role holding a person and nothing else (#2222). Who is
// personnel is decided by role tier now, in one place, for every flow that
// grants school access.
//
// A failed role lookup is an error, not a reason to skip: skipping produces
// exactly the broken half-state, silently.
func (s *invitationService) provisionSchoolIdentity(
	ctx context.Context,
	accountID int64,
	firstName, lastName string,
	invitation *authModels.InvitationToken,
) error {
	role, err := s.roleRepo.FindByID(ctx, invitation.RoleID)
	if err != nil {
		return &AuthError{Op: "provision school identity", Err: err}
	}
	if role == nil {
		return &AuthError{Op: "provision school identity", Err: ErrRoleNotAssignable}
	}

	var position string
	if invitation.Position != nil {
		position = *invitation.Position
	}

	_, err = EnsureSchoolIdentity(ctx, SchoolIdentityRepos{
		Persons:  s.personRepo,
		Staff:    s.staffRepo,
		Teachers: s.teacherRepo,
		Students: s.studentRepo,
	}, SchoolIdentityInput{
		AccountID: accountID,
		TenantID:  invitation.TenantID,
		Role:      role,
		FirstName: firstName,
		LastName:  lastName,
		Position:  position,
		// The staff import created the person before the invitation went
		// out; accepting must reuse it, not file a second one (#2600).
		PersonID: invitation.PersonID,
		// A Lehrkraft never gets a caregiver profile, caregiver_enabled or not —
		// same invariant as assignCaregiverRoleIfRequested (#1772).
		CaregiverUpgrade: invitation.CaregiverEnabled && !IsLehrkraftSystemRole(role),
		CreatePerson:     true,
	})
	if err != nil {
		return &AuthError{Op: "provision school identity", Err: err}
	}
	return nil
}

// ResendInvitation queues another email for an existing invitation if it is still valid.
func (s *invitationService) ResendInvitation(ctx context.Context, invitationID int64, actorAccountID int64) error {
	invitation, err := s.invitationRepo.FindByID(ctx, invitationID)
	if err != nil {
		if isNotFoundError(err) {
			return &AuthError{Op: opResendInvitation, Err: ErrInvitationNotFound}
		}
		return &AuthError{Op: opResendInvitation, Err: err}
	}

	if invitation.IsUsed() {
		return &AuthError{Op: opResendInvitation, Err: ErrInvitationUsed}
	}
	if InvitationTokenExpired(invitation, time.Now()) {
		return &AuthError{Op: opResendInvitation, Err: ErrInvitationExpired}
	}

	role, err := s.lookupRole(ctx, invitation.RoleID)
	if err != nil {
		return err
	}

	invitation.EmailSentAt = nil
	invitation.EmailError = nil
	invitation.UpdatedAt = time.Now()
	if err := s.invitationRepo.Update(ctx, invitation); err != nil {
		return &AuthError{Op: opResendInvitation, Err: err}
	}

	s.getLogger().Info("invitation resent",
		slog.Int64("invitation_id", invitation.ID),
		slog.Int64("actor_account_id", actorAccountID))

	schoolName := s.lookupSchoolName(ctx, invitation.TenantID)
	s.sendInvitationEmail(ctx, invitation, role.Name, schoolName, isSchoolPortalRole(role))
	return nil
}

// ListPendingInvitations returns all invitations that are still valid.
func (s *invitationService) ListPendingInvitations(ctx context.Context) ([]*authModels.InvitationToken, error) {
	invitations, err := s.invitationRepo.List(ctx, map[string]interface{}{"pending": true})
	if err != nil {
		return nil, &AuthError{Op: "list invitations", Err: err}
	}
	return invitations, nil
}

// RevokeInvitation marks an invitation as used so it can no longer be accepted.
func (s *invitationService) RevokeInvitation(ctx context.Context, invitationID int64, actorAccountID int64) error {
	invitation, err := s.invitationRepo.FindByID(ctx, invitationID)
	if err != nil {
		if isNotFoundError(err) {
			return &AuthError{Op: opRevokeInvitation, Err: ErrInvitationNotFound}
		}
		return &AuthError{Op: opRevokeInvitation, Err: err}
	}

	if invitation.IsUsed() {
		return &AuthError{Op: opRevokeInvitation, Err: ErrInvitationUsed}
	}

	if err := s.invitationRepo.MarkAsUsed(ctx, invitation.ID); err != nil {
		return &AuthError{Op: opRevokeInvitation, Err: err}
	}

	s.getLogger().Info("invitation revoked",
		slog.Int64("invitation_id", invitation.ID),
		slog.Int64("actor_account_id", actorAccountID))
	return nil
}

// InvalidatePendingInvitationsByTenantID marks all pending invitations for a tenant as used.
// Used during soft-delete to prevent redemption of invitations for deleted schools.
func (s *invitationService) InvalidatePendingInvitationsByTenantID(ctx context.Context, tenantID int64) (int, error) {
	count, err := s.invitationRepo.InvalidateByTenantID(ctx, tenantID)
	if err != nil {
		return 0, &AuthError{Op: "invalidate invitations by tenant", Err: err}
	}
	if count > 0 {
		s.getLogger().Info("pending invitations invalidated for deleted tenant",
			slog.Int64("tenant_id", tenantID),
			slog.Int("count", count))
	}
	return count, nil
}

// CleanupExpiredInvitations removes invitations that are no longer useful.
func (s *invitationService) CleanupExpiredInvitations(ctx context.Context) (int, error) {
	count, err := s.invitationRepo.DeleteExpired(ctx, time.Now())
	if err != nil {
		return 0, &AuthError{Op: "cleanup invitations", Err: err}
	}

	if count > 0 {
		s.getLogger().Info("invitation cleanup completed",
			slog.Int("records_deleted", count))
	}
	return count, nil
}

func (s *invitationService) fetchValidInvitation(ctx context.Context, token string) (*authModels.InvitationToken, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, &AuthError{Op: opFetchInvitation, Err: ErrInvitationNotFound}
	}

	invitation, err := s.invitationRepo.FindByToken(ctx, token)
	if err != nil {
		if isNotFoundError(err) {
			return nil, &AuthError{Op: opFetchInvitation, Err: ErrInvitationNotFound}
		}
		return nil, &AuthError{Op: opFetchInvitation, Err: err}
	}

	if invitation.IsUsed() {
		return nil, &AuthError{Op: opFetchInvitation, Err: ErrInvitationUsed}
	}

	if InvitationTokenExpired(invitation, time.Now()) {
		return nil, &AuthError{Op: opFetchInvitation, Err: ErrInvitationExpired}
	}

	return invitation, nil
}

func (s *invitationService) lookupRoleName(ctx context.Context, roleID int64) (string, error) {
	role, err := s.lookupRole(ctx, roleID)
	if err != nil {
		return "", err
	}
	return role.Name, nil
}

// lookupRole resolves the full role record — needed where the caller must
// branch on role properties beyond the name (school-portal link decision).
func (s *invitationService) lookupRole(ctx context.Context, roleID int64) (*authModels.Role, error) {
	role, err := s.roleRepo.FindByID(ctx, roleID)
	if err != nil {
		if isNotFoundError(err) {
			return nil, &AuthError{Op: "lookup role", Err: fmt.Errorf("role not found")}
		}
		return nil, &AuthError{Op: "lookup role", Err: err}
	}
	return role, nil
}

var invitationEmailBackoff = []time.Duration{
	time.Second,
	5 * time.Second,
	15 * time.Second,
}

// lookupSchoolName resolves the tenant display name for use in emails.
// Returns empty string on failure (best-effort, never blocks the caller).
func (s *invitationService) lookupSchoolName(ctx context.Context, tenantID int64) string {
	if tenantID == 0 || s.schoolRepo == nil {
		return ""
	}
	school, err := s.schoolRepo.FindByID(ctx, tenantID)
	if err != nil {
		s.getLogger().Warn("failed to lookup school name for invitation email",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()))
		return ""
	}
	if school == nil || school.IsDeleted() {
		return ""
	}
	return school.Name
}

func (s *invitationService) sendInvitationEmail(ctx context.Context, invitation *authModels.InvitationToken, roleName string, schoolName string, schoolPortal bool) {
	if s.dispatcher == nil {
		s.getLogger().Warn("email dispatcher unavailable, skipping invitation email",
			slog.Int64("invitation_id", invitation.ID))
		return
	}

	frontend := s.frontendURL
	if frontend == "" {
		frontend = "http://localhost:3000"
	}
	// School-portal roles accept their invitation on the school portal
	// (#2207): the link must land where their login lives, otherwise the
	// Lehrkraft sets a password in the staff portal and then cannot use it
	// there.
	if schoolPortal && s.schoolURL != "" {
		frontend = s.schoolURL
	}

	invitationURL := fmt.Sprintf("%s/invite?token=%s", frontend, invitation.Token)
	logoURL := fmt.Sprintf("%s/images/moto-logo-mit-schriftzug.png", frontend)
	expiryHours := int(s.invitationExpiry / time.Hour)

	subject := "Einladung zu moto"
	if schoolName != "" {
		subject = fmt.Sprintf("Einladung zu moto – %s", schoolName)
	}

	// An invited Mitarbeiter answering this mail ("wer lädt mich ein?") must
	// reach the OGS, not moto (#1936).
	replyIdentity := email.ResolveReplyToIdentity(ctx, s.mailIdentity, tenant.FromContext(ctx), s.getLogger())

	message := email.Message{
		From:     s.defaultFrom,
		ReplyTo:  email.NewEmail(replyIdentity.Name, replyIdentity.Address),
		To:       email.NewEmail("", invitation.Email),
		Subject:  subject,
		Template: "invitation.html",
		Content: map[string]any{
			"InvitationURL": invitationURL,
			"RoleName":      translateRoleNameToGerman(roleName),
			"FirstName":     invitation.FirstName,
			"LastName":      invitation.LastName,
			"ExpiryHours":   expiryHours,
			"LogoURL":       logoURL,
			"SchoolName":    schoolName,
		},
	}

	meta := email.DeliveryMetadata{
		Type:        "invitation",
		ReferenceID: invitation.ID,
		Token:       invitation.Token,
		Recipient:   invitation.Email,
	}

	baseRetry := invitation.EmailRetryCount

	s.dispatcher.Dispatch(detachedTenantContext(s.withTenantRuntime(ctx)), email.DeliveryRequest{
		Message:       message,
		Metadata:      meta,
		BackoffPolicy: invitationEmailBackoff,
		MaxAttempts:   3,
		Callback: func(cbCtx context.Context, result email.DeliveryResult) {
			s.persistInvitationDelivery(cbCtx, meta, baseRetry, result)
		},
	})
}

func (s *invitationService) persistInvitationDelivery(ctx context.Context, meta email.DeliveryMetadata, baseRetry int, result email.DeliveryResult) {
	ctx = s.withTenantRuntime(ctx)
	retryCount := baseRetry + result.Attempt
	var sentAt *time.Time
	var errText *string

	if result.Status == email.DeliveryStatusSent {
		sentTime := result.SentAt
		sentAt = &sentTime
	} else if result.Err != nil {
		msg := sanitizeEmailError(result.Err)
		errText = &msg
	}

	err := tenant.WithAdminTxOrDirect(ctx, s.db, func(adminCtx context.Context) error {
		return s.invitationRepo.UpdateDeliveryResult(adminCtx, meta.ReferenceID, sentAt, errText, retryCount)
	})
	if err != nil {
		s.getLogger().Error("failed to update invitation delivery status",
			slog.Int64("invitation_id", meta.ReferenceID),
			slog.Any("error", err),
		)
		return
	}

	if result.Final && result.Status == email.DeliveryStatusFailed {
		s.getLogger().Error("invitation email permanently failed",
			slog.Int64("invitation_id", meta.ReferenceID),
			slog.String("recipient", meta.Recipient),
			slog.Any("error", result.Err),
		)
	}
}

func sanitizeEmailError(err error) string {
	if err == nil {
		return ""
	}
	return strings.TrimSpace(err.Error())
}

func scopedInvitationTenantContext(ctx context.Context, tenantID int64) context.Context {
	if tenantID <= 0 {
		return ctx
	}
	if tenant.FromContext(ctx) == tenantID {
		return ctx
	}
	return tenant.WithTenantID(ctx, tenantID)
}

func nullableCreatedBy(id int64) *int64 {
	if id <= 0 {
		return nil
	}
	value := id
	return &value
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, sql.ErrNoRows) {
		return true
	}
	if errors.Is(err, userModels.ErrGuardianProfileNotFound) {
		return true
	}

	var dbErr *modelBase.DatabaseError
	if errors.As(err, &dbErr) {
		return errors.Is(dbErr.Err, sql.ErrNoRows)
	}

	return false
}

// GetTenantSubdomainForToken resolves the tenant subdomain from an invitation
// token. Tenant routing resolves hosts by subdomain, not slug (#1977).
// Best-effort: returns "" on any error so the accept response still succeeds.
func (s *invitationService) GetTenantSubdomainForToken(ctx context.Context, token string) string {
	ctx = s.withTenantRuntime(ctx)
	var subdomain string
	_ = tenant.WithAdminTx(ctx, s.db, func(txCtx context.Context, _ bun.Tx) error {
		invitation, err := s.invitationRepo.FindByToken(txCtx, token)
		if err != nil {
			return err
		}
		if invitation == nil {
			return nil
		}
		school, err := s.schoolRepo.FindByID(txCtx, invitation.TenantID)
		if err != nil {
			return err
		}
		if school == nil || school.IsDeleted() {
			return nil
		}
		subdomain = school.Subdomain
		return nil
	})
	return subdomain
}
