package auth

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/spf13/viper"
	"github.com/uptrace/bun"
)

// Password Reset

type passwordResetScope string

const (
	passwordResetScopeStaff  passwordResetScope = "staff"
	passwordResetScopeParent passwordResetScope = "parent"
	passwordResetScopeSchool passwordResetScope = "school"
)

type passwordResetOptions struct {
	scope   passwordResetScope
	baseURL string
}

// InitiatePasswordReset creates a password reset token for an account
func (s *Service) InitiatePasswordReset(ctx context.Context, emailAddress string) (*auth.PasswordResetToken, error) {
	return s.initiatePasswordReset(ctx, emailAddress, passwordResetOptions{
		scope:   passwordResetScopeStaff,
		baseURL: s.frontendURL,
	})
}

// InitiateParentPasswordReset creates a password reset token for guardian accounts only.
func (s *Service) InitiateParentPasswordReset(ctx context.Context, emailAddress string) (*auth.PasswordResetToken, error) {
	return s.initiatePasswordReset(ctx, emailAddress, passwordResetOptions{
		scope:   passwordResetScopeParent,
		baseURL: s.parentsURL,
	})
}

// InitiateSchoolPasswordReset creates a reset link for school-portal accounts
// only, so the link remains on the isolated school host.
func (s *Service) InitiateSchoolPasswordReset(ctx context.Context, emailAddress string) (*auth.PasswordResetToken, error) {
	return s.initiatePasswordReset(ctx, emailAddress, passwordResetOptions{
		scope:   passwordResetScopeSchool,
		baseURL: s.schoolURL,
	})
}

func (s *Service) initiatePasswordReset(ctx context.Context, emailAddress string, opts passwordResetOptions) (*auth.PasswordResetToken, error) {
	ctx = s.withTenantRuntime(ctx)
	// Normalize email
	emailAddress = strings.TrimSpace(strings.ToLower(emailAddress))

	// Get account by email
	account, err := s.repos.Account.FindByEmail(ctx, emailAddress)
	if err != nil {
		// Don't reveal whether the email exists or not
		return nil, nil
	}

	if opts.scope == passwordResetScopeParent {
		hasGuardianRole, _, err := s.findGuardianTenantForAccount(ctx, account.ID)
		if err != nil {
			return nil, err
		}
		if !hasGuardianRole {
			return nil, nil
		}
	}
	if opts.scope == passwordResetScopeSchool {
		hasSchoolRole, _, err := s.findSchoolPortalTenantForAccount(ctx, account.ID)
		if err != nil {
			return nil, err
		}
		if !hasSchoolRole {
			return nil, nil
		}
	}

	// Rate-limit only after confirming a real, actionable account. The
	// per-email limiter writes a row to auth.password_reset_rate_limits, so
	// keying it on accounts we will actually email keeps an unauthenticated
	// caller from inflating that table with rows for arbitrary nonexistent
	// addresses. Volumetric abuse and account-enumeration probing on the
	// public reset routes are bounded ahead of this path by the IP-keyed auth
	// rate limiter (api.base wires it via SetAuthRateLimiter on both the staff
	// and parent /auth groups).
	if err := s.checkPasswordResetRateLimit(ctx, emailAddress); err != nil {
		return nil, err
	}

	s.getLogger().Info("password reset requested",
		slog.String("scope", string(opts.scope)))

	// Create password reset token in transaction
	resetToken, err := s.createPasswordResetTokenInTransaction(ctx, account.ID)
	if err != nil {
		return nil, err
	}

	s.getLogger().Info("password reset token created",
		slog.Int64("account_id", account.ID))

	// Dispatch password reset email
	s.dispatchPasswordResetEmail(ctx, resetToken, account.Email, opts.baseURL)

	return resetToken, nil
}

// checkPasswordResetRateLimit checks if the email has exceeded rate limits
func (s *Service) checkPasswordResetRateLimit(ctx context.Context, emailAddress string) error {
	rateLimitEnabled := viper.GetBool("rate_limit_enabled")
	if !rateLimitEnabled || s.repos.PasswordResetRateLimit == nil {
		return nil
	}

	state, err := s.repos.PasswordResetRateLimit.CheckRateLimit(ctx, emailAddress)
	if err != nil {
		return &AuthError{Op: "check password reset rate limit", Err: err}
	}

	now := time.Now()
	if state != nil && state.Attempts >= passwordResetRateLimitThreshold && state.RetryAt.After(now) {
		return &AuthError{
			Op: "initiate password reset",
			Err: &RateLimitError{
				Err:      ErrRateLimitExceeded,
				Attempts: state.Attempts,
				RetryAt:  state.RetryAt,
			},
		}
	}

	state, err = s.repos.PasswordResetRateLimit.IncrementAttempts(ctx, emailAddress)
	if err != nil {
		return &AuthError{Op: "increment password reset rate limit", Err: err}
	}

	now = time.Now()
	if state != nil && state.Attempts > passwordResetRateLimitThreshold && state.RetryAt.After(now) {
		return &AuthError{
			Op: "initiate password reset",
			Err: &RateLimitError{
				Err:      ErrRateLimitExceeded,
				Attempts: state.Attempts,
				RetryAt:  state.RetryAt,
			},
		}
	}

	return nil
}

// createPasswordResetTokenInTransaction creates a password reset token in a transaction
//
// Phase 3 deviation: RunInTx retained because this is a public password reset route (no JWT/tenant context).
// Handler-level WithTenantTx requires authenticated tenant context, which doesn't exist
// at password-reset request time. Tenant is not needed for token creation.
func (s *Service) createPasswordResetTokenInTransaction(ctx context.Context, accountID int64) (*auth.PasswordResetToken, error) {
	var resetToken *auth.PasswordResetToken

	err := tenant.WithinAdmin(s.withTenantRuntime(ctx), func(ctx context.Context) error {

		if err := s.repos.PasswordResetToken.InvalidateTokensByAccountID(ctx, accountID); err != nil {
			s.getLogger().Error("failed to invalidate reset tokens, rolling back",
				slog.Int64("account_id", accountID),
				slog.Any("error", err),
			)
			return err
		}

		tokenStr := uuid.Must(uuid.NewV4()).String()
		resetToken = &auth.PasswordResetToken{
			AccountID: accountID,
			Token:     tokenStr,
			Expiry:    time.Now().Add(s.passwordResetExpiry),
			Used:      false,
		}

		if err := s.repos.PasswordResetToken.Create(ctx, resetToken); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, &AuthError{Op: "initiate password reset transaction", Err: err}
	}

	return resetToken, nil
}

// dispatchPasswordResetEmail sends the password reset email asynchronously
func (s *Service) dispatchPasswordResetEmail(ctx context.Context, resetToken *auth.PasswordResetToken, accountEmail, baseURL string) {
	if s.dispatcher == nil {
		s.getLogger().Warn("email dispatcher unavailable, skipping password reset email",
			slog.Int64("account_id", resetToken.AccountID))
		return
	}

	frontendURL := strings.TrimRight(baseURL, "/")
	resetURL := fmt.Sprintf("%s/reset-password?token=%s", frontendURL, resetToken.Token)
	logoURL := fmt.Sprintf("%s/images/moto-logo-mit-schriftzug.png", frontendURL)

	message := email.Message{
		From:     s.defaultFrom,
		To:       email.NewEmail("", accountEmail),
		Subject:  "Passwort zurücksetzen",
		Template: "password-reset.html",
		Content: map[string]any{
			"ResetURL":      resetURL,
			"ExpiryMinutes": int(s.passwordResetExpiry.Minutes()),
			"LogoURL":       logoURL,
		},
	}

	meta := email.DeliveryMetadata{
		Type:        "password_reset",
		ReferenceID: resetToken.ID,
		Token:       resetToken.Token,
		Recipient:   accountEmail,
	}

	baseRetry := resetToken.EmailRetryCount

	// Async delivery must outlive the HTTP request without retaining its tx.
	s.dispatcher.Dispatch(detachedTenantContext(s.withTenantRuntime(ctx)), email.DeliveryRequest{
		Message:       message,
		Metadata:      meta,
		BackoffPolicy: passwordResetEmailBackoff,
		MaxAttempts:   3,
		Callback: func(cbCtx context.Context, result email.DeliveryResult) {
			s.persistPasswordResetDelivery(cbCtx, meta, baseRetry, result)
		},
	})
}

// ResetPassword resets a password using a reset token
func (s *Service) ResetPassword(ctx context.Context, token, newPassword string) error {
	ctx = s.withTenantRuntime(ctx)
	// Find valid token
	resetToken, err := s.repos.PasswordResetToken.FindValidByToken(ctx, token)
	if err != nil {
		return &AuthError{Op: "reset password", Err: ErrInvalidToken}
	}

	// Validate new password
	if err := ValidatePasswordStrength(newPassword); err != nil {
		return &AuthError{Op: "reset password", Err: err}
	}

	// Hash new password
	passwordHash, err := HashPassword(newPassword)
	if err != nil {
		return &AuthError{Op: opHashPassword, Err: err}
	}

	// Uses WithAdminTx (BYPASSRLS) because password reset is a pre-authentication flow
	// with no JWT/tenant context. Token.DeleteByAccountID touches auth.tokens which has
	// RLS policies — phoenix_auth cannot satisfy them without tenant context.
	err = tenant.WithAdminTx(s.withTenantRuntime(ctx), s.db, func(ctx context.Context, tx bun.Tx) error {
		// Update account password
		if err := s.repos.Account.UpdatePassword(ctx, resetToken.AccountID, passwordHash); err != nil {
			return err
		}

		// Mark token as used
		if err := s.repos.PasswordResetToken.MarkAsUsed(ctx, resetToken.ID); err != nil {
			return err
		}

		// Invalidate all existing auth tokens for security
		if _, err := s.deleteAccountTokensWithAudit(ctx, resetToken.AccountID, "password_reset", "", ""); err != nil {
			return fmt.Errorf("revoke tokens during password reset: %w", err)
		}
		return nil
	})

	if err != nil {
		return &AuthError{Op: "reset password transaction", Err: err}
	}
	return nil
}

func (s *Service) persistPasswordResetDelivery(ctx context.Context, meta email.DeliveryMetadata, baseRetry int, result email.DeliveryResult) {
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

	if err := s.repos.PasswordResetToken.UpdateDeliveryResult(ctx, meta.ReferenceID, sentAt, errText, retryCount); err != nil {
		s.getLogger().Error("failed to update password reset delivery status",
			slog.Int64("token_id", meta.ReferenceID),
			slog.Any("error", err),
		)
		return
	}

	if result.Final && result.Status == email.DeliveryStatusFailed {
		s.getLogger().Error("password reset email permanently failed",
			slog.Int64("token_id", meta.ReferenceID),
			slog.String("recipient", meta.Recipient),
			slog.Any("error", result.Err),
		)
	}
}

// Note: sanitizeEmailError is defined in invitation_service.go and shared across the package
