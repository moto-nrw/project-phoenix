package auth

import (
	"context"
	"strings"
	"time"

	authModels "github.com/moto-nrw/project-phoenix/internal/core/domain/auth"
	modelBase "github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/logger"
	"github.com/moto-nrw/project-phoenix/internal/core/port"
	authPort "github.com/moto-nrw/project-phoenix/internal/core/port/auth"
	userPort "github.com/moto-nrw/project-phoenix/internal/core/port/users"
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
	PersonRepo       userPort.PersonRepository
	StaffRepo        userPort.StaffRepository
	TeacherRepo      userPort.TeacherRepository
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
	personRepo       userPort.PersonRepository
	staffRepo        userPort.StaffRepository
	teacherRepo      userPort.TeacherRepository
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
		personRepo = txRepo.WithTx(tx).(userPort.PersonRepository)
	}
	if txRepo, ok := s.staffRepo.(modelBase.TransactionalRepository); ok {
		staffRepo = txRepo.WithTx(tx).(userPort.StaffRepository)
	}
	if txRepo, ok := s.teacherRepo.(modelBase.TransactionalRepository); ok {
		teacherRepo = txRepo.WithTx(tx).(userPort.TeacherRepository)
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
