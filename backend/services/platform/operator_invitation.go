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
	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/models/platform"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// InviteOperator creates an invitation token and sends an email to the invitee.
func (s *operatorAuthService) InviteOperator(ctx context.Context, inviteeEmail string, displayName *string, createdByID int64, clientIP net.IP) error {
	if s.invitationTokenRepo == nil {
		return fmt.Errorf("operator invitation requires invitation_token_repo to be configured")
	}
	if s.operatorFrontendURL == "" {
		return fmt.Errorf("operator invitation requires operator_frontend_url to be configured")
	}
	if s.dispatcher == nil {
		return fmt.Errorf("operator invitation requires email dispatcher to be configured")
	}

	// 1. Normalize email
	inviteeEmail = strings.TrimSpace(strings.ToLower(inviteeEmail))

	// 2. Validate email format
	parsed, err := mail.ParseAddress(inviteeEmail)
	if err != nil {
		return &InvalidDataError{Err: fmt.Errorf("invalid email format")}
	}
	inviteeEmail = parsed.Address

	if !emailFormatRegex.MatchString(inviteeEmail) {
		return &InvalidDataError{Err: fmt.Errorf("invalid email format")}
	}
	if len(inviteeEmail) > 255 {
		return &InvalidDataError{Err: fmt.Errorf("email address too long")}
	}

	// 3. Check email not already an active operator
	existing, err := s.operatorRepo.FindByEmail(ctx, inviteeEmail)
	if err != nil {
		return fmt.Errorf("failed to check email uniqueness: %w", err)
	}
	if existing != nil {
		return &OperatorInvitationEmailExistsError{}
	}

	// 4. Invalidate old invitations and create new token atomically.
	// Without the transaction, a failed insert would leave the old token burned.
	var token *platform.OperatorInvitationToken
	if err := tenant.WithAdminTx(ctx, s.db, func(txCtx context.Context, _ bun.Tx) error {
		if _, txErr := s.invitationTokenRepo.InvalidateByEmail(txCtx, inviteeEmail); txErr != nil {
			return fmt.Errorf("failed to invalidate previous invitations: %w", txErr)
		}

		token = &platform.OperatorInvitationToken{
			Email:       inviteeEmail,
			Token:       uuid.Must(uuid.NewV4()).String(),
			ExpiresAt:   time.Now().Add(s.invitationExpiry),
			CreatedBy:   createdByID,
			DisplayName: displayName,
		}

		if txErr := s.invitationTokenRepo.Create(txCtx, token); txErr != nil {
			if isUniqueViolation(txErr) {
				return &OperatorInvitationEmailExistsError{}
			}
			return fmt.Errorf("failed to create invitation token: %w", txErr)
		}
		return nil
	}); err != nil {
		return err
	}

	// 6. Audit log (fire-and-forget)
	auditEntry := &platform.OperatorAuditLog{
		OperatorID:   createdByID,
		Action:       platform.ActionInvitationCreated,
		ResourceType: platform.ResourceInvitation,
		ResourceID:   &token.ID,
		RequestIP:    clientIP,
	}
	if err := s.auditLogRepo.Create(ctx, auditEntry); err != nil {
		s.getLogger().Error("failed to create audit log for operator invitation",
			slog.Int64("operator_id", createdByID),
			slog.Any("error", err),
		)
	}

	// 7. Dispatch invitation email
	s.dispatchOperatorInvitationEmail(ctx, token, createdByID)

	return nil
}

// ValidateOperatorInvitation validates an invitation token and returns its data.
func (s *operatorAuthService) ValidateOperatorInvitation(ctx context.Context, tokenStr string) (*platform.OperatorInvitationToken, error) {
	if s.invitationTokenRepo == nil {
		return nil, fmt.Errorf("operator invitation requires invitation_token_repo to be configured")
	}

	token, err := s.invitationTokenRepo.FindValidByToken(ctx, tokenStr)
	if err != nil {
		return nil, fmt.Errorf("failed to validate invitation token: %w", err)
	}
	if token == nil {
		return nil, &OperatorInvitationNotFoundError{}
	}

	return token, nil
}

// AcceptOperatorInvitation creates a new operator from a valid invitation token.
func (s *operatorAuthService) AcceptOperatorInvitation(ctx context.Context, tokenStr, displayName, password string, clientIP net.IP) (*platform.Operator, error) {
	if s.invitationTokenRepo == nil {
		return nil, fmt.Errorf("operator invitation requires invitation_token_repo to be configured")
	}

	// 1. Validate inputs (cheap — reject obviously invalid requests before any DB work)
	if err := authSvc.ValidatePasswordStrength(password); err != nil {
		return nil, &InvalidDataError{Err: fmt.Errorf("password doesn't meet complexity requirements")}
	}

	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return nil, &InvalidDataError{Err: fmt.Errorf("display name is required")}
	}
	if len(displayName) > 100 {
		return nil, &InvalidDataError{Err: fmt.Errorf("display name must not exceed 100 characters")}
	}

	var operator *platform.Operator
	err := tenant.WithAdminTx(ctx, s.db, func(txCtx context.Context, _ bun.Tx) error {
		// 2. Atomically consume the token FIRST (cheap DB operation).
		// This prevents CPU amplification: bogus/expired tokens are rejected before
		// the expensive Argon2id hash. The tx rollback on any later failure unconsumeS
		// the token, so it remains usable if account creation fails.
		token, consumeErr := s.invitationTokenRepo.ConsumeByToken(txCtx, tokenStr)
		if consumeErr != nil {
			return fmt.Errorf("failed to consume invitation token: %w", consumeErr)
		}
		if token == nil {
			return &OperatorInvitationNotFoundError{}
		}

		// 3. Check email uniqueness (race guard)
		existing, findErr := s.operatorRepo.FindByEmail(txCtx, token.Email)
		if findErr != nil {
			return fmt.Errorf("failed to check email uniqueness: %w", findErr)
		}
		if existing != nil {
			return &OperatorInvitationEmailExistsError{}
		}

		// 4. Hash password (expensive, but only reached for valid tokens)
		hash, hashErr := authSvc.HashPassword(password)
		if hashErr != nil {
			return fmt.Errorf("failed to hash password: %w", hashErr)
		}

		// 5. Create operator
		operator = &platform.Operator{
			Email:        token.Email,
			DisplayName:  displayName,
			PasswordHash: hash,
			Active:       true,
		}
		if createErr := s.operatorRepo.Create(txCtx, operator); createErr != nil {
			if isUniqueViolation(createErr) {
				return &OperatorInvitationEmailExistsError{}
			}
			return fmt.Errorf("failed to create operator: %w", createErr)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// 4. Audit log (fire-and-forget)
	auditEntry := &platform.OperatorAuditLog{
		OperatorID:   operator.ID,
		Action:       platform.ActionInvitationAccepted,
		ResourceType: platform.ResourceOperator,
		ResourceID:   &operator.ID,
		RequestIP:    clientIP,
	}
	if err := s.auditLogRepo.Create(ctx, auditEntry); err != nil {
		s.getLogger().Error("failed to create audit log for operator invitation acceptance",
			slog.Int64("operator_id", operator.ID),
			slog.Any("error", err),
		)
	}

	return operator, nil
}

// ListPendingOperatorInvitations returns all pending operator invitations.
func (s *operatorAuthService) ListPendingOperatorInvitations(ctx context.Context) ([]*platform.OperatorInvitationToken, error) {
	if s.invitationTokenRepo == nil {
		return nil, nil
	}
	return s.invitationTokenRepo.ListPending(ctx)
}

// RevokeOperatorInvitation marks an invitation as used.
func (s *operatorAuthService) RevokeOperatorInvitation(ctx context.Context, invitationID int64, actorID int64, clientIP net.IP) error {
	if s.invitationTokenRepo == nil {
		return fmt.Errorf("operator invitation requires invitation_token_repo to be configured")
	}

	token, err := s.invitationTokenRepo.FindByID(ctx, invitationID)
	if err != nil {
		return fmt.Errorf("failed to find invitation: %w", err)
	}
	if token == nil || !token.IsValid() {
		return &OperatorInvitationNotFoundError{}
	}

	revoked, err := s.invitationTokenRepo.MarkAsUsed(ctx, invitationID)
	if err != nil {
		return fmt.Errorf("failed to revoke invitation: %w", err)
	}
	if !revoked {
		return &OperatorInvitationNotFoundError{}
	}

	auditEntry := &platform.OperatorAuditLog{
		OperatorID:   actorID,
		Action:       platform.ActionInvitationRevoked,
		ResourceType: platform.ResourceInvitation,
		ResourceID:   &invitationID,
		RequestIP:    clientIP,
	}
	if err := s.auditLogRepo.Create(ctx, auditEntry); err != nil {
		s.getLogger().Error("failed to create audit log for invitation revocation",
			slog.Int64("operator_id", actorID),
			slog.Any("error", err),
		)
	}

	return nil
}

// ResendOperatorInvitation re-sends the invitation email.
func (s *operatorAuthService) ResendOperatorInvitation(ctx context.Context, invitationID int64, actorID int64, clientIP net.IP) error {
	if s.invitationTokenRepo == nil {
		return fmt.Errorf("operator invitation requires invitation_token_repo to be configured")
	}
	if s.dispatcher == nil {
		return fmt.Errorf("operator invitation resend requires email dispatcher to be configured")
	}

	token, err := s.invitationTokenRepo.FindByID(ctx, invitationID)
	if err != nil {
		return fmt.Errorf("failed to find invitation: %w", err)
	}
	if token == nil || !token.IsValid() {
		return &OperatorInvitationNotFoundError{}
	}

	// Extend expiry atomically — fails if the token was consumed concurrently
	newExpiresAt := time.Now().Add(s.invitationExpiry)
	extended, err := s.invitationTokenRepo.ExtendExpiry(ctx, invitationID, newExpiresAt)
	if err != nil {
		return fmt.Errorf("failed to extend invitation expiry: %w", err)
	}
	if !extended {
		return &OperatorInvitationNotFoundError{}
	}
	token.ExpiresAt = newExpiresAt

	// Reset delivery tracking
	if err := s.invitationTokenRepo.UpdateDeliveryResult(ctx, invitationID, nil, nil, 0); err != nil {
		return fmt.Errorf("failed to reset delivery tracking: %w", err)
	}
	token.EmailRetryCount = 0

	// Re-dispatch email
	s.dispatchOperatorInvitationEmail(ctx, token, actorID)

	auditEntry := &platform.OperatorAuditLog{
		OperatorID:   actorID,
		Action:       platform.ActionInvitationResent,
		ResourceType: platform.ResourceInvitation,
		ResourceID:   &invitationID,
		RequestIP:    clientIP,
	}
	if err := s.auditLogRepo.Create(ctx, auditEntry); err != nil {
		s.getLogger().Error("failed to create audit log for invitation resend",
			slog.Int64("operator_id", actorID),
			slog.Any("error", err),
		)
	}

	return nil
}

// CleanupExpiredOperatorInvitations removes expired invitation tokens.
func (s *operatorAuthService) CleanupExpiredOperatorInvitations(ctx context.Context) (int, error) {
	if s.invitationTokenRepo == nil {
		return 0, nil
	}
	return s.invitationTokenRepo.DeleteExpired(ctx)
}

func (s *operatorAuthService) dispatchOperatorInvitationEmail(ctx context.Context, token *platform.OperatorInvitationToken, inviterID int64) {
	if s.dispatcher == nil {
		s.getLogger().Warn("email dispatcher unavailable, skipping operator invitation email",
			slog.Int64("invitation_id", token.ID),
		)
		return
	}

	// Look up inviter name for the email template
	inviterName := "Ein Operator"
	inviter, err := s.operatorRepo.FindByID(ctx, inviterID)
	if err == nil && inviter != nil {
		inviterName = inviter.DisplayName
	}

	// Use the operator-specific URL to avoid cross-host redirects that drop fragments.
	// The path is /invite (not /operator/invite) because the operator subdomain proxy
	// rewrites /invite → /operator/invite internally.
	invitationURL := fmt.Sprintf("%s/invite#token=%s", s.operatorFrontendURL, token.Token)
	logoURL := fmt.Sprintf("%s/images/moto_transparent.png", s.frontendURL)

	message := email.Message{
		From:     s.defaultFrom,
		To:       email.NewEmail("", token.Email),
		Subject:  "Einladung als Operator zu moto",
		Template: "operator-invitation.html",
		Content: map[string]any{
			"InvitationURL": invitationURL,
			"ExpiryHours":   int(s.invitationExpiry.Hours()),
			"LogoURL":       logoURL,
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
		Message:       message,
		Metadata:      meta,
		BackoffPolicy: []time.Duration{1 * time.Second, 5 * time.Second, 15 * time.Second},
		MaxAttempts:   3,
		Callback: func(cbCtx context.Context, result email.DeliveryResult) {
			s.persistInvitationDelivery(cbCtx, meta, baseRetry, result)
		},
	})
}

func (s *operatorAuthService) persistInvitationDelivery(ctx context.Context, meta email.DeliveryMetadata, baseRetry int, result email.DeliveryResult) {
	retryCount := baseRetry + result.Attempt
	var sentAt *time.Time
	var errText *string

	if result.Status == email.DeliveryStatusSent {
		sentTime := result.SentAt
		sentAt = &sentTime
	} else if result.Err != nil {
		msg := strings.TrimSpace(result.Err.Error())
		errText = &msg
	}

	if err := s.invitationTokenRepo.UpdateDeliveryResult(ctx, meta.ReferenceID, sentAt, errText, retryCount); err != nil {
		s.getLogger().Error("failed to update invitation delivery status",
			slog.Int64("token_id", meta.ReferenceID),
			slog.Any("error", err),
		)
		return
	}

	if result.Final && result.Status == email.DeliveryStatusFailed {
		s.getLogger().Error("operator invitation email permanently failed",
			slog.Int64("token_id", meta.ReferenceID),
			slog.String("recipient", maskEmail(meta.Recipient)),
			slog.Any("error", result.Err),
		)
	}
}
