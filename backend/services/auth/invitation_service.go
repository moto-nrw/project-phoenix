package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/mail"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/email"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

type invitationService struct {
	invitationRepo   authModels.InvitationTokenRepository
	accountRepo      authModels.AccountRepository
	roleRepo         authModels.RoleRepository
	accountRoleRepo  authModels.AccountRoleRepository
	personRepo       userModels.PersonRepository
	dispatcher       *email.Dispatcher
	frontendURL      string
	defaultFrom      email.Email
	invitationExpiry time.Duration
	db               *bun.DB
	txHandler        *modelBase.TxHandler
}

// NewInvitationService constructs a new invitation service instance.
func NewInvitationService(
	invitationRepo authModels.InvitationTokenRepository,
	accountRepo authModels.AccountRepository,
	roleRepo authModels.RoleRepository,
	accountRoleRepo authModels.AccountRoleRepository,
	personRepo userModels.PersonRepository,
	mailer email.Mailer,
	dispatcher *email.Dispatcher,
	frontendURL string,
	defaultFrom email.Email,
	invitationExpiry time.Duration,
	db *bun.DB,
) InvitationService {
	trimmedFrontend := strings.TrimRight(strings.TrimSpace(frontendURL), "/")
	if dispatcher == nil && mailer != nil {
		dispatcher = email.NewDispatcher(mailer)
	}
	return &invitationService{
		invitationRepo:   invitationRepo,
		accountRepo:      accountRepo,
		roleRepo:         roleRepo,
		accountRoleRepo:  accountRoleRepo,
		personRepo:       personRepo,
		dispatcher:       dispatcher,
		frontendURL:      trimmedFrontend,
		defaultFrom:      defaultFrom,
		invitationExpiry: invitationExpiry,
		db:               db,
		txHandler:        modelBase.NewTxHandler(db),
	}
}

// WithTx clones the service with repositories bound to the provided transaction when supported.
func (s *invitationService) WithTx(tx bun.Tx) interface{} {
	var invitationRepo = s.invitationRepo
	var accountRepo = s.accountRepo
	var roleRepo = s.roleRepo
	var accountRoleRepo = s.accountRoleRepo
	var personRepo = s.personRepo

	if txRepo, ok := s.invitationRepo.(modelBase.TransactionalRepository); ok {
		invitationRepo = txRepo.WithTx(tx).(authModels.InvitationTokenRepository)
	}
	if txRepo, ok := s.accountRepo.(modelBase.TransactionalRepository); ok {
		accountRepo = txRepo.WithTx(tx).(authModels.AccountRepository)
	}
	if txRepo, ok := s.roleRepo.(modelBase.TransactionalRepository); ok {
		roleRepo = txRepo.WithTx(tx).(authModels.RoleRepository)
	}
	if txRepo, ok := s.accountRoleRepo.(modelBase.TransactionalRepository); ok {
		accountRoleRepo = txRepo.WithTx(tx).(authModels.AccountRoleRepository)
	}
	if txRepo, ok := s.personRepo.(modelBase.TransactionalRepository); ok {
		personRepo = txRepo.WithTx(tx).(userModels.PersonRepository)
	}

	return &invitationService{
		invitationRepo:   invitationRepo,
		accountRepo:      accountRepo,
		roleRepo:         roleRepo,
		accountRoleRepo:  accountRoleRepo,
		personRepo:       personRepo,
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
	emailAddress := strings.TrimSpace(strings.ToLower(req.Email))
	if emailAddress == "" {
		return nil, &AuthError{Op: "create invitation", Err: fmt.Errorf("email is required")}
	}

	if _, err := mail.ParseAddress(emailAddress); err != nil {
		return nil, &AuthError{Op: "create invitation", Err: fmt.Errorf("invalid email address")}
	}

	// Prevent generating invitations for emails that already belong to an account.
	if _, err := s.accountRepo.FindByEmail(ctx, emailAddress); err == nil {
		return nil, &AuthError{Op: "create invitation", Err: ErrEmailAlreadyExists}
	} else if !isNotFoundError(err) {
		return nil, &AuthError{Op: "create invitation", Err: err}
	}

	if req.RoleID <= 0 {
		return nil, &AuthError{Op: "create invitation", Err: fmt.Errorf("role id is required")}
	}

	if req.CreatedBy <= 0 {
		return nil, &AuthError{Op: "create invitation", Err: fmt.Errorf("created_by is required")}
	}

	if _, err := s.roleRepo.FindByID(ctx, req.RoleID); err != nil {
		if isNotFoundError(err) {
			return nil, &AuthError{Op: "create invitation", Err: fmt.Errorf("role not found")}
		}
		return nil, &AuthError{Op: "create invitation", Err: err}
	}

	// Mark any previous pending invitations for this email as used (effectively invalidating them).
	if _, err := s.invitationRepo.InvalidateByEmail(ctx, emailAddress); err != nil {
		return nil, &AuthError{Op: "invalidate invitations", Err: err}
	}

	token := uuid.Must(uuid.NewV4()).String()
	expiry := time.Now().Add(s.invitationExpiry)

	invitation := &authModels.InvitationToken{
		Email:     emailAddress,
		Token:     token,
		RoleID:    req.RoleID,
		CreatedBy: req.CreatedBy,
		ExpiresAt: expiry,
	}

	if req.FirstName != nil {
		firstName := strings.TrimSpace(*req.FirstName)
		invitation.FirstName = &firstName
	}
	if req.LastName != nil {
		lastName := strings.TrimSpace(*req.LastName)
		invitation.LastName = &lastName
	}

	if err := s.invitationRepo.Create(ctx, invitation); err != nil {
		return nil, &AuthError{Op: "create invitation", Err: err}
	}

	log.Printf("Invitation created by account=%d for email=%s", req.CreatedBy, invitation.Email)

	roleName, _ := s.lookupRoleName(ctx, invitation.RoleID)
	if roleName != "" {
		invitation.Role = &authModels.Role{
			Model: modelBase.Model{ID: invitation.RoleID},
			Name:  roleName,
		}
	}

	if creator, err := s.accountRepo.FindByID(ctx, invitation.CreatedBy); err == nil && creator != nil {
		invitation.Creator = &authModels.Account{
			Model: modelBase.Model{ID: creator.ID},
			Email: creator.Email,
		}
	} else if err != nil && !isNotFoundError(err) {
		return nil, &AuthError{Op: "lookup creator", Err: err}
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
		ExpiresAt: invitation.ExpiresAt,
	}, nil
}

// AcceptInvitation converts a token into a real account & person record.
func (s *invitationService) AcceptInvitation(ctx context.Context, token string, userData UserRegistrationData) (*authModels.Account, error) {
	invitation, err := s.fetchValidInvitation(ctx, token)
	if err != nil {
		return nil, err
	}

	if userData.Password != userData.ConfirmPassword {
		return nil, &AuthError{Op: "accept invitation", Err: ErrPasswordMismatch}
	}

	if err := ValidatePasswordStrength(userData.Password); err != nil {
		return nil, &AuthError{Op: "accept invitation", Err: err}
	}

	passwordHash, err := HashPassword(userData.Password)
	if err != nil {
		return nil, &AuthError{Op: "accept invitation", Err: err}
	}

	firstName := strings.TrimSpace(userData.FirstName)
	lastName := strings.TrimSpace(userData.LastName)

	if firstName == "" && invitation.FirstName != nil {
		firstName = strings.TrimSpace(*invitation.FirstName)
	}
	if lastName == "" && invitation.LastName != nil {
		lastName = strings.TrimSpace(*invitation.LastName)
	}

	if firstName == "" || lastName == "" {
		return nil, &AuthError{Op: "accept invitation", Err: fmt.Errorf("first name and last name are required")}
	}

	// Ensure account does not already exist.
	if _, err := s.accountRepo.FindByEmail(ctx, invitation.Email); err == nil {
		return nil, &AuthError{Op: "accept invitation", Err: ErrEmailAlreadyExists}
	} else if !isNotFoundError(err) {
		return nil, &AuthError{Op: "accept invitation", Err: err}
	}

	var createdAccount *authModels.Account

	err = s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		txService := s.WithTx(tx).(*invitationService)

		person := &userModels.Person{
			FirstName: firstName,
			LastName:  lastName,
		}
		if err := txService.personRepo.Create(ctx, person); err != nil {
			return &AuthError{Op: "create person", Err: err}
		}

		account := &authModels.Account{
			Email:        invitation.Email,
			Active:       true,
			PasswordHash: &passwordHash,
		}
		if err := txService.accountRepo.Create(ctx, account); err != nil {
			return &AuthError{Op: "create account", Err: err}
		}

		if err := txService.personRepo.LinkToAccount(ctx, person.ID, account.ID); err != nil {
			return &AuthError{Op: "link person to account", Err: err}
		}

		accountRole := &authModels.AccountRole{
			AccountID: account.ID,
			RoleID:    invitation.RoleID,
		}
		if err := txService.accountRoleRepo.Create(ctx, accountRole); err != nil {
			return &AuthError{Op: "assign role", Err: err}
		}

		if err := txService.invitationRepo.MarkAsUsed(ctx, invitation.ID); err != nil {
			return &AuthError{Op: "mark invitation used", Err: err}
		}

		createdAccount = account
		return nil
	})

	if err != nil {
		return nil, err
	}

	log.Printf("Invitation accepted for account=%d", createdAccount.ID)

	return createdAccount, nil
}

// ResendInvitation queues another email for an existing invitation if it is still valid.
func (s *invitationService) ResendInvitation(ctx context.Context, invitationID int64, actorAccountID int64) error {
	invitation, err := s.invitationRepo.FindByID(ctx, invitationID)
	if err != nil {
		if isNotFoundError(err) {
			return &AuthError{Op: "resend invitation", Err: ErrInvitationNotFound}
		}
		return &AuthError{Op: "resend invitation", Err: err}
	}

	if invitation.IsUsed() {
		return &AuthError{Op: "resend invitation", Err: ErrInvitationUsed}
	}
	if invitation.IsExpired() {
		return &AuthError{Op: "resend invitation", Err: ErrInvitationExpired}
	}

	roleName, err := s.lookupRoleName(ctx, invitation.RoleID)
	if err != nil {
		return err
	}

	invitation.EmailSentAt = nil
	invitation.EmailError = nil
	invitation.UpdatedAt = time.Now()
	if err := s.invitationRepo.Update(ctx, invitation); err != nil {
		return &AuthError{Op: "resend invitation", Err: err}
	}

	log.Printf("Invitation resent (id=%d) by account=%d", invitation.ID, actorAccountID)

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
			return &AuthError{Op: "revoke invitation", Err: ErrInvitationNotFound}
		}
		return &AuthError{Op: "revoke invitation", Err: err}
	}

	if invitation.IsUsed() {
		return &AuthError{Op: "revoke invitation", Err: ErrInvitationUsed}
	}

	if err := s.invitationRepo.MarkAsUsed(ctx, invitation.ID); err != nil {
		return &AuthError{Op: "revoke invitation", Err: err}
	}

	log.Printf("Invitation revoked (id=%d) by account=%d", invitation.ID, actorAccountID)
	return nil
}

// CleanupExpiredInvitations removes invitations that are no longer useful.
func (s *invitationService) CleanupExpiredInvitations(ctx context.Context) (int, error) {
	count, err := s.invitationRepo.DeleteExpired(ctx, time.Now())
	if err != nil {
		return 0, &AuthError{Op: "cleanup invitations", Err: err}
	}

	if count > 0 {
		log.Printf("Invitation cleanup removed %d records", count)
	}
	return count, nil
}

func (s *invitationService) fetchValidInvitation(ctx context.Context, token string) (*authModels.InvitationToken, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, &AuthError{Op: "fetch invitation", Err: ErrInvitationNotFound}
	}

	invitation, err := s.invitationRepo.FindByToken(ctx, token)
	if err != nil {
		if isNotFoundError(err) {
			return nil, &AuthError{Op: "fetch invitation", Err: ErrInvitationNotFound}
		}
		return nil, &AuthError{Op: "fetch invitation", Err: err}
	}

	if invitation.IsUsed() {
		return nil, &AuthError{Op: "fetch invitation", Err: ErrInvitationUsed}
	}

	if invitation.IsExpired() {
		return nil, &AuthError{Op: "fetch invitation", Err: ErrInvitationExpired}
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

var invitationEmailBackoff = []time.Duration{
	time.Second,
	5 * time.Second,
	15 * time.Second,
}

func (s *invitationService) sendInvitationEmail(invitation *authModels.InvitationToken, roleName string) {
	if s.dispatcher == nil {
		log.Printf("Email dispatcher unavailable; skipping invitation email id=%d", invitation.ID)
		return
	}

	frontend := s.frontendURL
	if frontend == "" {
		frontend = "http://localhost:3000"
	}

	invitationURL := fmt.Sprintf("%s/invite?token=%s", frontend, invitation.Token)
	logoURL := fmt.Sprintf("%s/images/moto_transparent.png", frontend)
	expiryHours := int(s.invitationExpiry / time.Hour)

	message := email.Message{
		From:     s.defaultFrom,
		To:       email.NewEmail("", invitation.Email),
		Subject:  "Einladung zu moto",
		Template: "invitation.html",
		Content: map[string]any{
			"InvitationURL": invitationURL,
			"RoleName":      roleName,
			"FirstName":     invitation.FirstName,
			"LastName":      invitation.LastName,
			"ExpiryHours":   expiryHours,
			"LogoURL":       logoURL,
		},
	}

	meta := email.DeliveryMetadata{
		Type:        "invitation",
		ReferenceID: invitation.ID,
		Token:       invitation.Token,
		Recipient:   invitation.Email,
	}

	baseRetry := invitation.EmailRetryCount

	s.dispatcher.Dispatch(email.DeliveryRequest{
		Message:       message,
		Metadata:      meta,
		BackoffPolicy: invitationEmailBackoff,
		MaxAttempts:   3,
		Context:       context.Background(),
		Callback: func(ctx context.Context, result email.DeliveryResult) {
			s.persistInvitationDelivery(ctx, meta, baseRetry, result)
		},
	})
}

func (s *invitationService) persistInvitationDelivery(ctx context.Context, meta email.DeliveryMetadata, baseRetry int, result email.DeliveryResult) {
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

	if err := s.invitationRepo.UpdateDeliveryResult(ctx, meta.ReferenceID, sentAt, errText, retryCount); err != nil {
		log.Printf("Failed to update invitation delivery status id=%d err=%v", meta.ReferenceID, err)
		return
	}

	if result.Final && result.Status == email.DeliveryStatusFailed {
		log.Printf("Invitation email permanently failed id=%d recipient=%s err=%v", meta.ReferenceID, meta.Recipient, result.Err)
	}
}

func sanitizeEmailError(err error) string {
	if err == nil {
		return ""
	}
	return strings.TrimSpace(err.Error())
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
