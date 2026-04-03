package platform

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/mail"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/auth/userpass"
	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/models/platform"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// OperatorInvitationService handles operator invitation flows
type OperatorInvitationService interface {
	CreateInvitation(ctx context.Context, email, displayName string, inviterID int64) (*platform.OperatorInvitationToken, error)
	ValidateInvitation(ctx context.Context, token string) (*OperatorInvitationValidation, error)
	AcceptInvitation(ctx context.Context, token, displayName, password string, clientIP net.IP) (*platform.Operator, error)
	ResendInvitation(ctx context.Context, invitationID, actorID int64) error
	RevokeInvitation(ctx context.Context, invitationID, actorID int64) error
	ListPendingInvitations(ctx context.Context) ([]*platform.OperatorInvitationToken, error)
	CleanupExpiredOperatorInvitations(ctx context.Context) (int, error)
}

// OperatorInvitationValidation is the public-safe view of an invitation
type OperatorInvitationValidation struct {
	Email       string  `json:"email"`
	DisplayName *string `json:"display_name,omitempty"`
	ExpiresAt   string  `json:"expires_at"`
	InviterName string  `json:"inviter_name"`
}

// OperatorInvitationServiceConfig holds configuration for the operator invitation service
type OperatorInvitationServiceConfig struct {
	InvitationRepo   platform.OperatorInvitationTokenRepository
	OperatorRepo     platform.OperatorRepository
	AuditLogRepo     platform.OperatorAuditLogRepository
	DB               *bun.DB
	Logger           *slog.Logger
	Dispatcher       *email.Dispatcher
	DefaultFrom      email.Email
	FrontendURL      string
	InvitationExpiry time.Duration
}

type operatorInvitationService struct {
	invitationRepo platform.OperatorInvitationTokenRepository
	operatorRepo   platform.OperatorRepository
	auditLogRepo   platform.OperatorAuditLogRepository
	db             *bun.DB
	logger         *slog.Logger
	dispatcher     *email.Dispatcher
	defaultFrom    email.Email
	frontendURL    string
	invitationExp  time.Duration
}

// NewOperatorInvitationService creates a new operator invitation service
func NewOperatorInvitationService(cfg OperatorInvitationServiceConfig) OperatorInvitationService {
	exp := cfg.InvitationExpiry
	if exp <= 0 {
		exp = 48 * time.Hour
	}

	return &operatorInvitationService{
		invitationRepo: cfg.InvitationRepo,
		operatorRepo:   cfg.OperatorRepo,
		auditLogRepo:   cfg.AuditLogRepo,
		db:             cfg.DB,
		logger:         cfg.Logger,
		dispatcher:     cfg.Dispatcher,
		defaultFrom:    cfg.DefaultFrom,
		frontendURL:    cfg.FrontendURL,
		invitationExp:  exp,
	}
}

func (s *operatorInvitationService) getLogger() *slog.Logger {
	if s.logger != nil {
		return s.logger
	}
	return slog.Default()
}

// CreateInvitation creates a new operator invitation and dispatches an email.
func (s *operatorInvitationService) CreateInvitation(ctx context.Context, emailAddr, displayName string, inviterID int64) (*platform.OperatorInvitationToken, error) {
	// 1. Validate email format
	emailAddr = strings.TrimSpace(strings.ToLower(emailAddr))
	parsed, err := mail.ParseAddress(emailAddr)
	if err != nil {
		return nil, &InvalidDataError{Err: fmt.Errorf("invalid email format")}
	}
	emailAddr = parsed.Address

	if !emailFormatRegex.MatchString(emailAddr) {
		return nil, &InvalidDataError{Err: fmt.Errorf("invalid email format")}
	}

	if len(emailAddr) > 255 {
		return nil, &InvalidDataError{Err: fmt.Errorf("email address too long")}
	}

	displayName = strings.TrimSpace(displayName)

	// 2. Check if email is already registered as an operator
	existing, err := s.operatorRepo.FindByEmail(ctx, emailAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to check email uniqueness: %w", err)
	}
	if existing != nil {
		return nil, &EmailAlreadyInUseError{}
	}

	// 3. Get inviter info for email template
	inviter, err := s.operatorRepo.FindByID(ctx, inviterID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch inviter: %w", err)
	}
	if inviter == nil {
		return nil, &OperatorNotFoundError{OperatorID: inviterID}
	}

	// 4. Invalidate previous pending invitations for this email and create new token
	var token *platform.OperatorInvitationToken
	err = tenant.WithAdminTx(ctx, s.db, func(txCtx context.Context, _ bun.Tx) error {
		if err := s.invitationRepo.InvalidateByEmail(txCtx, emailAddr); err != nil {
			return fmt.Errorf("failed to invalidate previous invitations: %w", err)
		}

		token = &platform.OperatorInvitationToken{
			Email:     emailAddr,
			Token:     uuid.Must(uuid.NewV4()).String(),
			Expiry:    time.Now().Add(s.invitationExp),
			Used:      false,
			InvitedBy: inviterID,
		}
		if displayName != "" {
			token.DisplayName = &displayName
		}

		if err := s.invitationRepo.Create(txCtx, token); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		if isUniqueViolation(err) {
			return nil, &EmailAlreadyInUseError{}
		}
		return nil, fmt.Errorf("failed to create operator invitation: %w", err)
	}

	// Attach inviter for response
	token.Inviter = inviter

	// 5. Audit log (fire-and-forget)
	s.logAudit(ctx, inviterID, platform.ActionInviteOperator, &token.ID, map[string]any{
		"invited_email": emailAddr,
	}, nil)

	// 6. Dispatch invitation email
	s.dispatchInvitationEmail(ctx, token, inviter.DisplayName)

	return token, nil
}

// ValidateInvitation validates an invitation token and returns public-safe data.
func (s *operatorInvitationService) ValidateInvitation(ctx context.Context, tokenStr string) (*OperatorInvitationValidation, error) {
	tokenStr = strings.TrimSpace(tokenStr)
	if tokenStr == "" {
		return nil, &OperatorInvitationExpiredOrUsedError{}
	}

	// Run in admin tx to bypass RLS (public endpoint, no tenant context)
	var result *OperatorInvitationValidation
	err := tenant.WithAdminTx(ctx, s.db, func(txCtx context.Context, _ bun.Tx) error {
		token, err := s.invitationRepo.FindValidByToken(txCtx, tokenStr)
		if err != nil {
			return fmt.Errorf("failed to validate invitation token: %w", err)
		}
		if token == nil {
			return &OperatorInvitationExpiredOrUsedError{}
		}

		inviterName := ""
		if token.Inviter != nil {
			inviterName = token.Inviter.DisplayName
		}

		result = &OperatorInvitationValidation{
			Email:       token.Email,
			DisplayName: token.DisplayName,
			ExpiresAt:   token.Expiry.Format(time.RFC3339),
			InviterName: inviterName,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// AcceptInvitation accepts an invitation, creating a new operator account.
func (s *operatorInvitationService) AcceptInvitation(ctx context.Context, tokenStr, displayName, password string, clientIP net.IP) (*platform.Operator, error) {
	tokenStr = strings.TrimSpace(tokenStr)
	if tokenStr == "" {
		return nil, &OperatorInvitationExpiredOrUsedError{}
	}

	// 1. Validate password strength (cheap check before anything else)
	if err := authSvc.ValidatePasswordStrength(password); err != nil {
		return nil, &InvalidDataError{Err: fmt.Errorf("password doesn't meet complexity requirements")}
	}

	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return nil, &InvalidDataError{Err: fmt.Errorf("display name is required")}
	}
	if len(displayName) > 100 {
		return nil, &InvalidDataError{Err: fmt.Errorf("display name too long")}
	}

	// 2. Verify token exists before expensive Argon2id hashing.
	// This is an unauthenticated endpoint — without this check, any request
	// with a random/expired token would still pay the full hash cost.
	existing, err := s.invitationRepo.FindValidByToken(ctx, tokenStr)
	if err != nil {
		return nil, fmt.Errorf("failed to validate invitation token: %w", err)
	}
	if existing == nil {
		return nil, &OperatorInvitationExpiredOrUsedError{}
	}

	// 3. Hash password outside transaction (expensive Argon2id)
	passwordHash, err := userpass.HashPassword(password, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// 4. Consume token and create operator atomically
	var newOperator *platform.Operator
	var consumedTokenID int64
	err = tenant.WithAdminTx(ctx, s.db, func(txCtx context.Context, _ bun.Tx) error {
		// Atomically consume token
		token, err := s.invitationRepo.ConsumeByToken(txCtx, tokenStr)
		if err != nil {
			return fmt.Errorf("failed to consume invitation token: %w", err)
		}
		if token == nil {
			return &OperatorInvitationExpiredOrUsedError{}
		}
		consumedTokenID = token.ID

		// Check email uniqueness (race condition guard)
		existing, err := s.operatorRepo.FindByEmail(txCtx, token.Email)
		if err != nil {
			return fmt.Errorf("failed to check email uniqueness: %w", err)
		}
		if existing != nil {
			return &EmailAlreadyInUseError{}
		}

		// Create operator
		newOperator = &platform.Operator{
			Email:        token.Email,
			DisplayName:  displayName,
			PasswordHash: passwordHash,
			Active:       true,
		}
		if err := s.operatorRepo.Create(txCtx, newOperator); err != nil {
			return fmt.Errorf("failed to create operator: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// 4. Audit log (fire-and-forget, outside transaction)
	s.logAudit(ctx, newOperator.ID, platform.ActionAcceptInvitation, &consumedTokenID, map[string]any{
		"email": newOperator.Email,
	}, clientIP)

	s.getLogger().Info("operator invitation accepted",
		slog.Int64("operator_id", newOperator.ID),
		slog.String("email", newOperator.Email),
	)

	return newOperator, nil
}

// ResendInvitation resends the invitation email for a pending invitation.
func (s *operatorInvitationService) ResendInvitation(ctx context.Context, invitationID, actorID int64) error {
	token, err := s.invitationRepo.FindByID(ctx, invitationID)
	if err != nil {
		return fmt.Errorf("failed to find invitation: %w", err)
	}
	if token == nil {
		return &OperatorInvitationNotFoundError{InvitationID: invitationID}
	}

	if token.Used {
		return &OperatorInvitationAlreadyAcceptedError{InvitationID: invitationID}
	}
	if token.Expiry.Before(time.Now()) {
		return &OperatorInvitationExpiredOrUsedError{}
	}

	// Get inviter name for the email
	inviterName := ""
	if token.Inviter != nil {
		inviterName = token.Inviter.DisplayName
	} else {
		inviter, err := s.operatorRepo.FindByID(ctx, token.InvitedBy)
		if err == nil && inviter != nil {
			inviterName = inviter.DisplayName
		}
	}

	// Clear delivery metadata for fresh retry
	if err := s.invitationRepo.UpdateDeliveryResult(ctx, invitationID, nil, nil, 0); err != nil {
		s.getLogger().Error("failed to clear delivery metadata for resend",
			slog.Int64("invitation_id", invitationID),
			slog.Any("error", err),
		)
	}

	s.dispatchInvitationEmail(ctx, token, inviterName)

	s.logAudit(ctx, actorID, platform.ActionResendInvitation, &invitationID, map[string]any{
		"email": token.Email,
	}, nil)

	return nil
}

// RevokeInvitation marks an invitation as used, preventing redemption.
func (s *operatorInvitationService) RevokeInvitation(ctx context.Context, invitationID, actorID int64) error {
	token, err := s.invitationRepo.FindByID(ctx, invitationID)
	if err != nil {
		return fmt.Errorf("failed to find invitation: %w", err)
	}
	if token == nil {
		return &OperatorInvitationNotFoundError{InvitationID: invitationID}
	}

	if token.Used {
		return &OperatorInvitationAlreadyAcceptedError{InvitationID: invitationID}
	}

	if err := s.invitationRepo.InvalidateByEmail(ctx, token.Email); err != nil {
		return fmt.Errorf("failed to revoke invitation: %w", err)
	}

	s.logAudit(ctx, actorID, platform.ActionRevokeInvitation, &invitationID, map[string]any{
		"email": token.Email,
	}, nil)

	s.getLogger().Info("operator invitation revoked",
		slog.Int64("invitation_id", invitationID),
		slog.Int64("actor_id", actorID),
	)

	return nil
}

// ListPendingInvitations returns all non-expired, non-used invitations.
func (s *operatorInvitationService) ListPendingInvitations(ctx context.Context) ([]*platform.OperatorInvitationToken, error) {
	return s.invitationRepo.ListPending(ctx)
}

// CleanupExpiredOperatorInvitations invalidates expired tokens and deletes stale ones.
func (s *operatorInvitationService) CleanupExpiredOperatorInvitations(ctx context.Context) (int, error) {
	invalidated, err := s.invitationRepo.InvalidateExpiredTokens(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to invalidate expired operator invitations: %w", err)
	}

	deleted, err := s.invitationRepo.DeleteStaleTokens(ctx)
	if err != nil {
		return invalidated, fmt.Errorf("failed to delete stale operator invitations: %w", err)
	}

	total := invalidated + deleted
	if total > 0 {
		s.getLogger().Info("operator invitation cleanup completed",
			slog.Int("invalidated", invalidated),
			slog.Int("deleted", deleted),
		)
	}

	return total, nil
}

// dispatchInvitationEmail sends the invitation email asynchronously.
func (s *operatorInvitationService) dispatchInvitationEmail(ctx context.Context, token *platform.OperatorInvitationToken, inviterName string) {
	if s.dispatcher == nil {
		s.getLogger().Warn("email dispatcher not configured, skipping invitation email",
			slog.Int64("invitation_id", token.ID),
		)
		return
	}

	// Use a URL fragment (#token=...) instead of a query parameter to prevent the
	// token from appearing in Referer headers, server access logs, or CDN logs.
	// Matches the pattern used by the operator email-change flow.
	invitationURL := fmt.Sprintf("%s/operator/invite#token=%s", s.frontendURL, token.Token)
	logoURL := fmt.Sprintf("%s/images/moto_transparent.png", s.frontendURL)

	displayName := ""
	if token.DisplayName != nil {
		displayName = *token.DisplayName
	}

	msg := email.Message{
		From:     s.defaultFrom,
		To:       email.NewEmail("", token.Email),
		Subject:  "Einladung zum moto Operator-Dashboard",
		Template: "operator-invitation.html",
		Content: map[string]any{
			"InvitationURL": invitationURL,
			"LogoURL":       logoURL,
			"ExpiryHours":   int(s.invitationExp.Hours()),
			"DisplayName":   displayName,
			"InviterName":   inviterName,
		},
	}

	meta := email.DeliveryMetadata{
		Type:        "operator_invitation",
		ReferenceID: token.ID,
		Token:       token.Token,
		Recipient:   token.Email,
	}

	baseRetry := token.EmailRetryCount

	s.dispatcher.Dispatch(context.WithoutCancel(ctx), email.DeliveryRequest{
		Message:  msg,
		Metadata: meta,
		BackoffPolicy: []time.Duration{
			time.Second,
			5 * time.Second,
			15 * time.Second,
		},
		Callback: func(cbCtx context.Context, result email.DeliveryResult) {
			s.persistDelivery(cbCtx, meta, baseRetry, result)
		},
	})
}

// persistDelivery stores email delivery results back to the token record.
func (s *operatorInvitationService) persistDelivery(ctx context.Context, meta email.DeliveryMetadata, baseRetry int, result email.DeliveryResult) {
	var sentAt *time.Time
	var errText *string
	retryCount := baseRetry + result.Attempt

	switch result.Status {
	case email.DeliveryStatusSent:
		t := result.SentAt
		sentAt = &t
	case email.DeliveryStatusFailed:
		if result.Err != nil {
			msg := result.Err.Error()
			errText = &msg
		}
	}

	if err := s.invitationRepo.UpdateDeliveryResult(ctx, meta.ReferenceID, sentAt, errText, retryCount); err != nil {
		s.getLogger().Error("failed to persist operator invitation delivery result",
			slog.Int64("invitation_id", meta.ReferenceID),
			slog.Any("error", err),
		)
	}

	if result.Final && result.Status == email.DeliveryStatusFailed {
		s.getLogger().Error("operator invitation email permanently failed",
			slog.Int64("invitation_id", meta.ReferenceID),
			slog.String("recipient", meta.Recipient),
		)
	}
}

// logAudit creates an audit log entry (fire-and-forget).
func (s *operatorInvitationService) logAudit(ctx context.Context, operatorID int64, action string, resourceID *int64, changes map[string]any, clientIP net.IP) {
	entry := &platform.OperatorAuditLog{
		OperatorID:   operatorID,
		Action:       action,
		ResourceType: platform.ResourceOperatorInvitation,
		ResourceID:   resourceID,
		RequestIP:    clientIP,
	}
	if changes != nil {
		if err := entry.SetChanges(changes); err != nil {
			s.getLogger().Error("failed to set audit changes",
				slog.String("action", action),
				slog.Any("error", err),
			)
		}
	}
	if err := s.auditLogRepo.Create(ctx, entry); err != nil {
		s.getLogger().Error("failed to create audit log for operator invitation",
			slog.String("action", action),
			slog.Any("error", err),
		)
	}
}
