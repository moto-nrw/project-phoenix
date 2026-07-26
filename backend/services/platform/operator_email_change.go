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
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/platform"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const emailChangeRateLimit = 5

// InitiateEmailChange starts the email change flow: validates credentials,
// creates a verification token, and dispatches emails to both old and new addresses.
func (s *operatorAuthService) InitiateEmailChange(ctx context.Context, operatorID int64, newEmail, currentPassword string, clientIP net.IP) error {
	if s.FrontendURL == "" {
		return fmt.Errorf("email change requires frontend_url to be configured")
	}
	if s.EmailChangeTokenRepo == nil {
		return fmt.Errorf("email change requires email_change_token_repo to be configured")
	}
	if s.Dispatcher == nil {
		return fmt.Errorf("email change requires email dispatcher to be configured")
	}

	// 1. Fetch operator
	operator, err := s.OperatorRepo.FindByID(ctx, operatorID)
	if err != nil {
		return err
	}
	if operator == nil {
		return &OperatorNotFoundError{OperatorID: operatorID}
	}

	// 2. Check active status
	if !operator.Active {
		return &OperatorInactiveError{OperatorID: operator.ID}
	}

	// 3. Verify current password (Argon2id — expensive, done outside the tx)
	match, err := userpass.VerifyPassword(currentPassword, operator.PasswordHash)
	if err != nil || !match {
		return &PasswordMismatchError{}
	}
	// Snapshot the hash so we can detect a concurrent password rotation inside
	// the token-creation transaction (TOCTOU guard for ChangePassword race).
	preCheckPasswordHash := operator.PasswordHash

	// 4. Normalize new email
	newEmail = strings.TrimSpace(strings.ToLower(newEmail))

	// 5. Validate email format and canonicalize to bare mailbox address
	parsed, err := mail.ParseAddress(newEmail)
	if err != nil {
		return &InvalidDataError{Err: fmt.Errorf("invalid email format")}
	}
	newEmail = parsed.Address

	// 5b. Require a real domain with TLD (ParseAddress accepts "t@t" per RFC 5322)
	if !userModels.IsValidEmailFormat(newEmail) {
		return &InvalidDataError{Err: fmt.Errorf("invalid email format")}
	}

	// 5c. Reject oversized emails early — Operator.Validate() enforces 255-char
	// column limit, but catching it here gives a clear error instead of a
	// confusing "invalid link" on confirmation.
	if len(newEmail) > 255 {
		return &InvalidDataError{Err: fmt.Errorf("email address too long")}
	}

	// 6. Same-email guard
	if newEmail == operator.Email {
		return &EmailChangeSameEmailError{}
	}

	// 7. Create token and audit log in transaction.
	// Rate limit is checked BEFORE invalidating existing tokens so that a
	// blocked request never destroys the operator's last usable confirmation link.
	// The partial unique index idx_email_change_tokens_one_active_per_operator
	// serializes concurrent INSERTs: if two requests both pass the rate limit
	// check, the second INSERT fails with a unique violation mapped to
	// EmailChangeRateLimitError.
	var token *platform.OperatorEmailChangeToken
	// emailTaken is set inside the transaction when the address is already in use.
	// We return nil (silent success) after the tx to prevent email enumeration,
	// but the check runs after the rate limit so probing behavior is identical
	// regardless of whether the address exists.
	var emailTaken bool
	maskedEmail := maskEmail(newEmail)
	err = tenant.WithAdminTx(ctx, s.DB, func(txCtx context.Context, _ bun.Tx) error {
		// Lock the operator row to serialize concurrent email-change requests
		// for the same operator. Without this, two parallel transactions under
		// READ COMMITTED could both pass the rate-limit count and both insert,
		// relying solely on the partial unique index for conflict detection.
		if _, err := s.OperatorRepo.FindByIDForUpdate(txCtx, operatorID); err != nil {
			return fmt.Errorf("failed to lock operator row: %w", err)
		}

		// Rate limit FIRST — check before invalidating the active token so that
		// a blocked request does not destroy the operator's last usable link.
		count, err := s.EmailChangeTokenRepo.CountRecentByOperatorID(txCtx, operatorID, time.Now().Add(-1*time.Hour))
		if err != nil {
			return fmt.Errorf("failed to check rate limit: %w", err)
		}
		if count >= emailChangeRateLimit {
			return &EmailChangeRateLimitError{}
		}

		// TOCTOU guard: re-read the operator inside the transaction to detect
		// a concurrent ChangePassword that committed between our VerifyPassword
		// call and this point. Comparing hashes is cheap (no Argon2id needed).
		current, err := s.OperatorRepo.FindByID(txCtx, operatorID)
		if err != nil {
			return fmt.Errorf("failed to re-fetch operator in tx: %w", err)
		}
		if current == nil || current.PasswordHash != preCheckPasswordHash {
			return &PasswordMismatchError{}
		}

		// Check uniqueness AFTER rate limit — returning success silently for
		// taken addresses prevents email enumeration, and doing so after the
		// rate-limit check ensures the observable behavior (rate limit vs.
		// success) is the same regardless of whether the address is in use.
		existing, err := s.OperatorRepo.FindByEmail(txCtx, newEmail)
		if err != nil {
			return fmt.Errorf("failed to check email uniqueness: %w", err)
		}
		if existing != nil {
			emailTaken = true
			return nil
		}

		// Only invalidate after passing the rate limit — the old token stays
		// usable if this request is going to be rejected anyway.
		if err := s.EmailChangeTokenRepo.InvalidateByOperatorID(txCtx, operatorID); err != nil {
			return err
		}

		token = &platform.OperatorEmailChangeToken{
			OperatorID: operatorID,
			NewEmail:   newEmail,
			Token:      uuid.Must(uuid.NewV4()).String(),
			Expiry:     time.Now().Add(s.EmailChangeExpiry),
			Used:       false,
		}
		if err := s.EmailChangeTokenRepo.Create(txCtx, token); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		if base.IsUniqueViolation(err) {
			return &EmailChangeRateLimitError{}
		}
		return fmt.Errorf("failed to create email change token: %w", err)
	}

	// Anti-enumeration: silently return success when the email is already taken.
	// The rate-limit check ran inside the transaction so the observable behavior
	// is identical regardless of whether the address exists.
	if emailTaken {
		s.getLogger().Debug("email change skipped: address already in use",
			slog.Int64("operator_id", operatorID),
			slog.String("new_email", maskedEmail),
		)
		return nil
	}

	// 8. Audit log (outside transaction — fire-and-forget, consistent with Login/Confirm pattern).
	// Trade-off: if the audit INSERT fails, the email change succeeds without a trail entry.
	// This is acceptable because (a) the failure is logged at Error level for Grafana/Loki
	// alerting, and (b) making audit writes transactional would let audit DB errors block
	// legitimate operations.
	auditEntry := &platform.OperatorAuditLog{
		OperatorID:   operatorID,
		Action:       platform.ActionEmailChangeInitiated,
		ResourceType: platform.ResourceOperator,
		ResourceID:   &operatorID,
		RequestIP:    clientIP,
	}
	if err := auditEntry.SetChanges(map[string]any{"masked_new_email": maskedEmail}); err != nil {
		s.getLogger().Error("failed to set audit changes",
			slog.Int64("operator_id", operatorID),
			slog.Any("error", err),
		)
	}
	if err := s.AuditLogRepo.Create(ctx, auditEntry); err != nil {
		s.getLogger().Error("failed to create audit log for email change initiation",
			slog.Int64("operator_id", operatorID),
			slog.Any("error", err),
		)
	}

	// 10. Dispatch verification email to new address (fire-and-forget — dispatcher
	// nil is caught at method entry point, async delivery failures are handled
	// by the callback in persistEmailChangeDelivery)
	s.dispatchVerificationEmail(ctx, token, newEmail)

	// 11. Dispatch notification email to old address (best-effort, don't block on failure)
	s.dispatchNotificationEmail(ctx, operator, maskedEmail)

	return nil
}

// ConfirmEmailChange completes the email change by verifying the token and updating the operator's email.
// Returns the new email address on success.
func (s *operatorAuthService) ConfirmEmailChange(ctx context.Context, tokenStr string, clientIP net.IP) (string, error) {
	if s.EmailChangeTokenRepo == nil {
		return "", fmt.Errorf("email change requires email_change_token_repo to be configured")
	}

	var oldEmail string
	var newEmail string
	var operatorID int64
	var displayName string

	err := tenant.WithAdminTx(ctx, s.DB, func(txCtx context.Context, _ bun.Tx) error {
		// a. Atomically consume the token (UPDATE ... WHERE used = FALSE RETURNING *).
		// Only one concurrent transaction can succeed — the loser sees zero rows
		// and gets EmailChangeTokenInvalidError. If a later step fails, the entire
		// transaction rolls back and the token remains unconsumed.
		token, err := s.EmailChangeTokenRepo.ConsumeByToken(txCtx, tokenStr)
		if err != nil {
			return fmt.Errorf("failed to consume email change token: %w", err)
		}
		if token == nil {
			return &EmailChangeTokenInvalidError{}
		}

		// b. Re-check email uniqueness (race condition guard)
		existing, err := s.OperatorRepo.FindByEmail(txCtx, token.NewEmail)
		if err != nil {
			return fmt.Errorf("failed to check email uniqueness: %w", err)
		}
		if existing != nil {
			return &EmailAlreadyInUseError{}
		}

		// c. Fetch operator
		operator, err := s.OperatorRepo.FindByID(txCtx, token.OperatorID)
		if err != nil {
			return fmt.Errorf("failed to find operator: %w", err)
		}
		if operator == nil {
			return &EmailChangeTokenInvalidError{}
		}

		// d. Check active status
		if !operator.Active {
			return &OperatorInactiveError{OperatorID: operator.ID}
		}

		// e. Capture old email and display name for audit + notification
		oldEmail = operator.Email
		operatorID = operator.ID
		displayName = operator.DisplayName

		// f+g. Update and persist email
		newEmail = token.NewEmail
		operator.Email = token.NewEmail
		if err := s.OperatorRepo.Update(txCtx, operator); err != nil {
			if base.IsUniqueViolation(err) {
				return &EmailAlreadyInUseError{}
			}
			return fmt.Errorf("failed to update operator email: %w", err)
		}

		return nil
	})
	if err != nil {
		return "", err
	}

	// Audit log (outside transaction — fire-and-forget, see InitiateEmailChange for trade-off rationale)
	auditEntry := &platform.OperatorAuditLog{
		OperatorID:   operatorID,
		Action:       platform.ActionEmailChangeConfirmed,
		ResourceType: platform.ResourceOperator,
		ResourceID:   &operatorID,
		RequestIP:    clientIP,
	}
	if err := auditEntry.SetChanges(map[string]any{
		"masked_old_email": maskEmail(oldEmail),
		"masked_new_email": maskEmail(newEmail),
	}); err != nil {
		s.getLogger().Error("failed to set audit changes",
			slog.Int64("operator_id", operatorID),
			slog.Any("error", err),
		)
	}
	if err := s.AuditLogRepo.Create(ctx, auditEntry); err != nil {
		s.getLogger().Error("failed to create audit log for email change confirmation",
			slog.Int64("operator_id", operatorID),
			slog.Any("error", err),
		)
	}

	// Notify old email address that the change was completed (fire-and-forget).
	// If the account was compromised, the legitimate owner learns the email was
	// actually changed — not just requested.
	s.dispatchChangeConfirmedEmail(ctx, oldEmail, displayName)

	return newEmail, nil
}

// CleanupExpiredEmailChangeTokens marks expired tokens as used (freeing the
// unique index so operators can request new links), then deletes stale rows.
func (s *operatorAuthService) CleanupExpiredEmailChangeTokens(ctx context.Context) (int, error) {
	if s.EmailChangeTokenRepo == nil {
		return 0, nil
	}

	invalidated, err := s.EmailChangeTokenRepo.InvalidateExpiredTokens(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to invalidate expired tokens: %w", err)
	}
	if invalidated > 0 {
		s.getLogger().Info("invalidated expired email change tokens",
			slog.Int("count", invalidated),
		)
	}

	return s.EmailChangeTokenRepo.DeleteStaleTokens(ctx)
}

func (s *operatorAuthService) dispatchVerificationEmail(ctx context.Context, token *platform.OperatorEmailChangeToken, newEmail string) {
	if s.Dispatcher == nil {
		s.getLogger().Warn("email dispatcher unavailable, skipping email change verification",
			slog.Int64("operator_id", token.OperatorID),
		)
		return
	}

	// The URL goes through /operator/* → operator subdomain redirect (proxy.ts),
	// which preserves query params via nextUrl.search. The client strips the
	// token from the URL on mount via history.replaceState, so any Referer
	// from subsequent navigations on this page carries no token.
	verifyURL := fmt.Sprintf("%s/operator/email-confirm?token=%s", s.FrontendURL, token.Token)
	logoURL := fmt.Sprintf("%s/images/moto-logo-mit-schriftzug.png", s.FrontendURL)

	message := email.Message{
		From:     s.DefaultFrom,
		To:       email.NewEmail("", newEmail),
		Subject:  "E-Mail-Adresse bestätigen",
		Template: "operator-email-change-verify.html",
		Content: map[string]any{
			"VerifyURL":     verifyURL,
			"ExpiryMinutes": int(s.EmailChangeExpiry.Minutes()),
			"LogoURL":       logoURL,
		},
	}

	meta := email.DeliveryMetadata{
		Type:        "operator_email_change_verify",
		ReferenceID: token.ID,
		Token:       token.Token,
		Recipient:   newEmail,
	}

	baseRetry := token.EmailRetryCount

	s.Dispatcher.Dispatch(context.WithoutCancel(ctx), email.DeliveryRequest{
		Message:       message,
		Metadata:      meta,
		BackoffPolicy: []time.Duration{1 * time.Second, 5 * time.Second, 15 * time.Second},
		MaxAttempts:   3,
		Callback: func(cbCtx context.Context, result email.DeliveryResult) {
			s.persistEmailChangeDelivery(cbCtx, meta, baseRetry, result)
		},
	})
}

func (s *operatorAuthService) dispatchNotificationEmail(ctx context.Context, operator *platform.Operator, maskedNewEmail string) {
	if s.Dispatcher == nil {
		s.getLogger().Warn("email dispatcher unavailable, skipping email change notification",
			slog.Int64("operator_id", operator.ID),
		)
		return
	}

	logoURL := fmt.Sprintf("%s/images/moto-logo-mit-schriftzug.png", s.FrontendURL)

	message := email.Message{
		From:     s.DefaultFrom,
		To:       email.NewEmail(operator.DisplayName, operator.Email),
		Subject:  "E-Mail-Änderung angefordert",
		Template: "operator-email-change-notify.html",
		Content: map[string]any{
			"LogoURL":        logoURL,
			"MaskedNewEmail": maskedNewEmail,
			"DisplayName":    operator.DisplayName,
		},
	}

	s.Dispatcher.Dispatch(context.WithoutCancel(ctx), email.DeliveryRequest{
		Message:       message,
		Metadata:      email.DeliveryMetadata{Recipient: operator.Email},
		BackoffPolicy: []time.Duration{1 * time.Second, 5 * time.Second, 15 * time.Second},
		MaxAttempts:   3,
		Callback:      nil,
	})
}

func (s *operatorAuthService) dispatchChangeConfirmedEmail(ctx context.Context, oldEmail, displayName string) {
	if s.Dispatcher == nil {
		return
	}

	logoURL := fmt.Sprintf("%s/images/moto-logo-mit-schriftzug.png", s.FrontendURL)

	message := email.Message{
		From:     s.DefaultFrom,
		To:       email.NewEmail(displayName, oldEmail),
		Subject:  "E-Mail-Adresse wurde geändert",
		Template: "operator-email-change-confirmed.html",
		Content: map[string]any{
			"LogoURL":     logoURL,
			"DisplayName": displayName,
		},
	}

	s.Dispatcher.Dispatch(context.WithoutCancel(ctx), email.DeliveryRequest{
		Message:       message,
		Metadata:      email.DeliveryMetadata{Recipient: oldEmail},
		BackoffPolicy: []time.Duration{1 * time.Second, 5 * time.Second, 15 * time.Second},
		MaxAttempts:   3,
		Callback:      nil,
	})
}

func (s *operatorAuthService) persistEmailChangeDelivery(ctx context.Context, meta email.DeliveryMetadata, baseRetry int, result email.DeliveryResult) {
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

	if err := s.EmailChangeTokenRepo.UpdateDeliveryResult(ctx, meta.ReferenceID, sentAt, errText, retryCount); err != nil {
		s.getLogger().Error("failed to update email change delivery status",
			slog.Int64("token_id", meta.ReferenceID),
			slog.Any("error", err),
		)
		return
	}

	if result.Final && result.Status == email.DeliveryStatusFailed {
		s.getLogger().Error("email change verification email permanently failed",
			slog.Int64("token_id", meta.ReferenceID),
			slog.String("recipient", maskEmail(meta.Recipient)),
			slog.Any("error", result.Err),
		)
	}
}

// maskEmail masks an email address for display: shows first character + *** before @.
// For very short local parts (1-2 chars), the entire local part is hidden to avoid
// revealing a significant portion of the address.
func maskEmail(addr string) string {
	parts := strings.SplitN(addr, "@", 2)
	if len(parts) != 2 || parts[0] == "" {
		return "***"
	}
	if len(parts[0]) <= 2 {
		return "***@" + parts[1]
	}
	return string(parts[0][0]) + "***@" + parts[1]
}
