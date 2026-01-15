package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	authModels "github.com/moto-nrw/project-phoenix/internal/core/domain/auth"
	modelBase "github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	userModels "github.com/moto-nrw/project-phoenix/internal/core/domain/users"
	"github.com/moto-nrw/project-phoenix/internal/core/logger"
	"github.com/moto-nrw/project-phoenix/internal/core/port"
	authPort "github.com/moto-nrw/project-phoenix/internal/core/port/auth"
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

// InvitationServiceConfig holds configuration for the invitation service
type InvitationServiceConfig struct {
	InvitationRepo   authPort.InvitationTokenRepository
	AccountRepo      authPort.AccountRepository
	RoleRepo         authPort.RoleRepository
	AccountRoleRepo  authPort.AccountRoleRepository
	PersonRepo       userModels.PersonRepository
	StaffRepo        userModels.StaffRepository
	TeacherRepo      userModels.TeacherRepository
	Dispatcher       port.EmailDispatcher
	FrontendURL      string
	DefaultFrom      port.EmailAddress
	InvitationExpiry time.Duration
	DB               *bun.DB
}

type invitationService struct {
	invitationRepo   authPort.InvitationTokenRepository
	accountRepo      authPort.AccountRepository
	roleRepo         authPort.RoleRepository
	accountRoleRepo  authPort.AccountRoleRepository
	personRepo       userModels.PersonRepository
	staffRepo        userModels.StaffRepository
	teacherRepo      userModels.TeacherRepository
	dispatcher       port.EmailDispatcher
	frontendURL      string
	defaultFrom      port.EmailAddress
	invitationExpiry time.Duration
	db               *bun.DB
	txHandler        *modelBase.TxHandler
}

// NewInvitationService constructs a new invitation service instance.
func NewInvitationService(config InvitationServiceConfig) InvitationService {
	trimmedFrontend := strings.TrimRight(strings.TrimSpace(config.FrontendURL), "/")
	return &invitationService{
		invitationRepo:   config.InvitationRepo,
		accountRepo:      config.AccountRepo,
		roleRepo:         config.RoleRepo,
		accountRoleRepo:  config.AccountRoleRepo,
		personRepo:       config.PersonRepo,
		staffRepo:        config.StaffRepo,
		teacherRepo:      config.TeacherRepo,
		dispatcher:       config.Dispatcher,
		frontendURL:      trimmedFrontend,
		defaultFrom:      config.DefaultFrom,
		invitationExpiry: config.InvitationExpiry,
		db:               config.DB,
		txHandler:        modelBase.NewTxHandler(config.DB),
	}
}

// WithTx clones the service with repositories bound to the provided transaction when supported.
func (s *invitationService) WithTx(tx bun.Tx) any {
	var invitationRepo = s.invitationRepo
	var accountRepo = s.accountRepo
	var roleRepo = s.roleRepo
	var accountRoleRepo = s.accountRoleRepo
	var personRepo = s.personRepo
	var staffRepo = s.staffRepo
	var teacherRepo = s.teacherRepo

	if txRepo, ok := s.invitationRepo.(modelBase.TransactionalRepository); ok {
		invitationRepo = txRepo.WithTx(tx).(authPort.InvitationTokenRepository)
	}
	if txRepo, ok := s.accountRepo.(modelBase.TransactionalRepository); ok {
		accountRepo = txRepo.WithTx(tx).(authPort.AccountRepository)
	}
	if txRepo, ok := s.roleRepo.(modelBase.TransactionalRepository); ok {
		roleRepo = txRepo.WithTx(tx).(authPort.RoleRepository)
	}
	if txRepo, ok := s.accountRoleRepo.(modelBase.TransactionalRepository); ok {
		accountRoleRepo = txRepo.WithTx(tx).(authPort.AccountRoleRepository)
	}
	if txRepo, ok := s.personRepo.(modelBase.TransactionalRepository); ok {
		personRepo = txRepo.WithTx(tx).(userModels.PersonRepository)
	}
	if txRepo, ok := s.staffRepo.(modelBase.TransactionalRepository); ok {
		staffRepo = txRepo.WithTx(tx).(userModels.StaffRepository)
	}
	if txRepo, ok := s.teacherRepo.(modelBase.TransactionalRepository); ok {
		teacherRepo = txRepo.WithTx(tx).(userModels.TeacherRepository)
	}

	return &invitationService{
		invitationRepo:   invitationRepo,
		accountRepo:      accountRepo,
		roleRepo:         roleRepo,
		accountRoleRepo:  accountRoleRepo,
		personRepo:       personRepo,
		staffRepo:        staffRepo,
		teacherRepo:      teacherRepo,
		dispatcher:       s.dispatcher,
		frontendURL:      s.frontendURL,
		defaultFrom:      s.defaultFrom,
		invitationExpiry: s.invitationExpiry,
		db:               s.db,
		txHandler:        s.txHandler.WithTx(tx),
	}
}

// CreateInvitation creates an invitation token and queues the invitation email.
func (s *invitationService) CreateInvitation(ctx context.Context, req InvitationRequest) (*authModels.InvitationToken, error) {
	emailAddress, err := s.validateInvitationRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	if err := s.invalidatePreviousInvitations(ctx, emailAddress); err != nil {
		return nil, err
	}

	invitation := s.buildInvitationToken(emailAddress, req)
	if err := s.invitationRepo.Create(ctx, invitation); err != nil {
		return nil, &AuthError{Op: opCreateInvitation, Err: err}
	}

	logger.Logger.WithFields(map[string]interface{}{
		"account_id": req.CreatedBy,
		"email":      invitation.Email,
	}).Info("Invitation created")

	if err := s.attachRoleAndCreator(ctx, invitation); err != nil {
		return nil, err
	}

	roleName := ""
	if invitation.Role != nil {
		roleName = invitation.Role.Name
	}
	s.sendInvitationEmail(invitation, roleName)

	return invitation, nil
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

	if err := s.ensureEmailNotRegistered(ctx, emailAddress, opCreateInvitation); err != nil {
		return "", err
	}

	if req.RoleID <= 0 {
		return "", &AuthError{Op: opCreateInvitation, Err: fmt.Errorf("role id is required")}
	}

	if req.CreatedBy <= 0 {
		return "", &AuthError{Op: opCreateInvitation, Err: fmt.Errorf("created_by is required")}
	}

	if err := s.ensureRoleExists(ctx, req.RoleID); err != nil {
		return "", err
	}

	return emailAddress, nil
}

// ensureEmailNotRegistered checks that no account exists with the given email.
func (s *invitationService) ensureEmailNotRegistered(ctx context.Context, email, op string) error {
	_, err := s.accountRepo.FindByEmail(ctx, email)
	if err == nil {
		return &AuthError{Op: op, Err: ErrEmailAlreadyExists}
	}
	if !isNotFoundError(err) {
		return &AuthError{Op: op, Err: err}
	}
	return nil
}

// ensureRoleExists verifies the role ID is valid.
func (s *invitationService) ensureRoleExists(ctx context.Context, roleID int64) error {
	_, err := s.roleRepo.FindByID(ctx, roleID)
	if err == nil {
		return nil
	}
	if isNotFoundError(err) {
		return &AuthError{Op: opCreateInvitation, Err: fmt.Errorf("role not found")}
	}
	return &AuthError{Op: opCreateInvitation, Err: err}
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
		Email:     email,
		Token:     uuid.Must(uuid.NewV4()).String(),
		RoleID:    req.RoleID,
		CreatedBy: req.CreatedBy,
		ExpiresAt: time.Now().Add(s.invitationExpiry),
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
func (s *invitationService) attachRoleAndCreator(ctx context.Context, invitation *authModels.InvitationToken) error {
	roleName, _ := s.lookupRoleName(ctx, invitation.RoleID)
	if roleName != "" {
		invitation.Role = &authModels.Role{
			Model: modelBase.Model{ID: invitation.RoleID},
			Name:  roleName,
		}
	}

	creator, err := s.accountRepo.FindByID(ctx, invitation.CreatedBy)
	if err != nil && !isNotFoundError(err) {
		return &AuthError{Op: "lookup creator", Err: err}
	}
	if creator != nil {
		invitation.Creator = &authModels.Account{
			Model: modelBase.Model{ID: creator.ID},
			Email: creator.Email,
		}
	}

	return nil
}

// ValidateInvitation returns the public details for a token if it is still usable.
func (s *invitationService) ValidateInvitation(ctx context.Context, token string) (*InvitationValidationResult, error) {
	invitation, err := s.fetchValidInvitation(ctx, token)
	if err != nil {
		return nil, err
	}

	roleName, err := s.lookupRoleName(ctx, invitation.RoleID)
	if err != nil {
		return nil, err
	}

	return &InvitationValidationResult{
		Email:     invitation.Email,
		RoleName:  roleName,
		FirstName: invitation.FirstName,
		LastName:  invitation.LastName,
		Position:  invitation.Position,
		ExpiresAt: invitation.ExpiresAt,
	}, nil
}

// AcceptInvitation converts a token into a real account & person record.
func (s *invitationService) AcceptInvitation(ctx context.Context, token string, userData UserRegistrationData) (*authModels.Account, error) {
	invitation, err := s.fetchValidInvitation(ctx, token)
	if err != nil {
		return nil, err
	}

	passwordHash, err := s.validateAndHashPassword(userData)
	if err != nil {
		return nil, err
	}

	firstName, lastName, err := s.resolveNames(userData, invitation)
	if err != nil {
		return nil, err
	}

	if err := s.ensureEmailNotRegistered(ctx, invitation.Email, opAcceptInvitation); err != nil {
		return nil, err
	}

	var createdAccount *authModels.Account
	err = s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		txService := s.WithTx(tx).(*invitationService)
		account, txErr := txService.createAccountWithRole(ctx, invitation, passwordHash, firstName, lastName)
		if txErr != nil {
			return txErr
		}
		createdAccount = account
		return nil
	})

	if err != nil {
		return nil, err
	}

	logger.Logger.WithField("account_id", createdAccount.ID).Info("Invitation accepted")
	return createdAccount, nil
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
) (*authModels.Account, error) {
	person, err := s.createPerson(ctx, firstName, lastName)
	if err != nil {
		return nil, err
	}

	account, err := s.createAccount(ctx, invitation.Email, passwordHash)
	if err != nil {
		return nil, err
	}

	if err := s.personRepo.LinkToAccount(ctx, person.ID, account.ID); err != nil {
		return nil, &AuthError{Op: "link person to account", Err: err}
	}

	if err := s.assignRole(ctx, account.ID, invitation.RoleID); err != nil {
		return nil, err
	}

	if err := s.createStaffAndTeacherIfSystemRole(ctx, person.ID, invitation); err != nil {
		return nil, err
	}

	if err := s.invitationRepo.MarkAsUsed(ctx, invitation.ID); err != nil {
		return nil, &AuthError{Op: "mark invitation used", Err: err}
	}

	return account, nil
}

// createPerson creates a new person record.
func (s *invitationService) createPerson(ctx context.Context, firstName, lastName string) (*userModels.Person, error) {
	person := &userModels.Person{
		FirstName: firstName,
		LastName:  lastName,
	}
	if err := s.personRepo.Create(ctx, person); err != nil {
		return nil, &AuthError{Op: "create person", Err: err}
	}
	return person, nil
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

// assignRole assigns a role to an account.
func (s *invitationService) assignRole(ctx context.Context, accountID, roleID int64) error {
	accountRole := &authModels.AccountRole{
		AccountID: accountID,
		RoleID:    roleID,
	}
	if err := s.accountRoleRepo.Create(ctx, accountRole); err != nil {
		return &AuthError{Op: "assign role", Err: err}
	}
	return nil
}

// createStaffAndTeacherIfSystemRole creates staff and teacher records for system roles.
func (s *invitationService) createStaffAndTeacherIfSystemRole(
	ctx context.Context,
	personID int64,
	invitation *authModels.InvitationToken,
) error {
	role, err := s.roleRepo.FindByID(ctx, invitation.RoleID)
	if err != nil || role == nil || !role.IsSystem {
		return nil // Not a system role or error looking up - skip staff/teacher creation
	}

	staff := &userModels.Staff{PersonID: personID}
	if err := s.staffRepo.Create(ctx, staff); err != nil {
		return &AuthError{Op: "create staff", Err: err}
	}

	teacher := &userModels.Teacher{StaffID: staff.ID}
	if invitation.Position != nil {
		teacher.Role = *invitation.Position
	}
	if err := s.teacherRepo.Create(ctx, teacher); err != nil {
		return &AuthError{Op: "create teacher", Err: err}
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
	if invitation.IsExpired() {
		return &AuthError{Op: opResendInvitation, Err: ErrInvitationExpired}
	}

	roleName, err := s.lookupRoleName(ctx, invitation.RoleID)
	if err != nil {
		return err
	}

	invitation.EmailSentAt = nil
	invitation.EmailError = nil
	invitation.UpdatedAt = time.Now()
	if err := s.invitationRepo.Update(ctx, invitation); err != nil {
		return &AuthError{Op: opResendInvitation, Err: err}
	}

	logger.Logger.WithFields(map[string]interface{}{
		"invitation_id": invitation.ID,
		"account_id":    actorAccountID,
	}).Info("Invitation resent")

	s.sendInvitationEmail(invitation, roleName)
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

	logger.Logger.WithFields(map[string]interface{}{
		"invitation_id": invitation.ID,
		"account_id":    actorAccountID,
	}).Info("Invitation revoked")
	return nil
}

// CleanupExpiredInvitations removes invitations that are no longer useful.
func (s *invitationService) CleanupExpiredInvitations(ctx context.Context) (int, error) {
	count, err := s.invitationRepo.DeleteExpired(ctx, time.Now())
	if err != nil {
		return 0, &AuthError{Op: "cleanup invitations", Err: err}
	}

	if count > 0 {
		logger.Logger.WithField("count", count).Info("Invitation cleanup removed records")
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

	if invitation.IsExpired() {
		return nil, &AuthError{Op: opFetchInvitation, Err: ErrInvitationExpired}
	}

	return invitation, nil
}

func (s *invitationService) lookupRoleName(ctx context.Context, roleID int64) (string, error) {
	role, err := s.roleRepo.FindByID(ctx, roleID)
	if err != nil {
		if isNotFoundError(err) {
			return "", &AuthError{Op: "lookup role", Err: fmt.Errorf("role not found")}
		}
		return "", &AuthError{Op: "lookup role", Err: err}
	}
	return role.Name, nil
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, sql.ErrNoRows) {
		return true
	}

	var dbErr *modelBase.DatabaseError
	if errors.As(err, &dbErr) {
		return errors.Is(dbErr.Err, sql.ErrNoRows)
	}

	return false
}
