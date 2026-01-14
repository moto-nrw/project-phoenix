package auth

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/spf13/viper"
	"github.com/uptrace/bun"
)

const (
	passwordResetRateLimitThreshold = 3
)

var passwordResetEmailBackoff = []time.Duration{
	time.Second,
	5 * time.Second,
	15 * time.Second,
}

// InitiatePasswordReset creates a password reset token for an account
func (s *Service) InitiatePasswordReset(ctx context.Context, emailAddress string) (*auth.PasswordResetToken, error) {
	// Normalize email
	emailAddress = strings.TrimSpace(strings.ToLower(emailAddress))

	// Get account by email
	account, err := s.repos.Account.FindByEmail(ctx, emailAddress)
	if err != nil {
		// Don't reveal whether the email exists or not
		return nil, nil
	}

	// Check rate limiting
	if err := s.checkPasswordResetRateLimit(ctx, emailAddress); err != nil {
		return nil, err
	}

	log.Printf("Password reset requested for email=%s", emailAddress)

	// Create password reset token in transaction
	resetToken, err := s.createPasswordResetTokenInTransaction(ctx, account.ID)
	if err != nil {
		return nil, err
	}

	log.Printf("Password reset token created for account=%d", account.ID)

	// Dispatch password reset email
	s.dispatchPasswordResetEmail(ctx, resetToken, account.Email)

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
func (s *Service) createPasswordResetTokenInTransaction(ctx context.Context, accountID int64) (*auth.PasswordResetToken, error) {
	var resetToken *auth.PasswordResetToken

	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		txService := s.WithTx(tx).(AuthService)

		if err := txService.(*Service).repos.PasswordResetToken.InvalidateTokensByAccountID(ctx, accountID); err != nil {
			log.Printf("Failed to invalidate reset tokens for account %d, rolling back: %v", accountID, err)
			return err
		}

		tokenStr := uuid.Must(uuid.NewV4()).String()
		resetToken = &auth.PasswordResetToken{
			AccountID: accountID,
			Token:     tokenStr,
			Expiry:    time.Now().Add(s.passwordResetExpiry),
			Used:      false,
		}

		if err := txService.(*Service).repos.PasswordResetToken.Create(ctx, resetToken); err != nil {
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
func (s *Service) dispatchPasswordResetEmail(ctx context.Context, resetToken *auth.PasswordResetToken, accountEmail string) {
	if s.dispatcher == nil {
		log.Printf("Email dispatcher unavailable; skipping password reset email account=%d", resetToken.AccountID)
		return
	}

	frontendURL := strings.TrimRight(s.frontendURL, "/")
	resetURL := fmt.Sprintf("%s/reset-password?token=%s", frontendURL, resetToken.Token)
	logoURL := fmt.Sprintf("%s/images/moto_transparent.png", frontendURL)

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

	s.dispatcher.Dispatch(ctx, email.DeliveryRequest{
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

	// Execute in transaction
	err = s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Get transactional service
		txService := s.WithTx(tx).(AuthService)

		// Update account password
		if err := txService.(*Service).repos.Account.UpdatePassword(ctx, resetToken.AccountID, passwordHash); err != nil {
			return err
		}

		// Mark token as used
		if err := txService.(*Service).repos.PasswordResetToken.MarkAsUsed(ctx, resetToken.ID); err != nil {
			return err
		}

		// Invalidate all existing auth tokens for security
		if err := txService.(*Service).repos.Token.DeleteByAccountID(ctx, resetToken.AccountID); err != nil {
			// Log error but don't fail the password reset
			log.Printf("Failed to delete tokens during password reset for account %d: %v", resetToken.AccountID, err)
		}

		return nil
	})

	if err != nil {
		return &AuthError{Op: "reset password transaction", Err: err}
	}

	return nil
}

// persistPasswordResetDelivery updates the delivery status of a password reset email
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
		log.Printf("Failed to update password reset delivery status token_id=%d err=%v", meta.ReferenceID, err)
		return
	}

	if result.Final && result.Status == email.DeliveryStatusFailed {
		log.Printf("Password reset email permanently failed id=%d recipient=%s err=%v", meta.ReferenceID, meta.Recipient, result.Err)
	}
}

// CleanupExpiredPasswordResetTokens removes expired password reset tokens
func (s *Service) CleanupExpiredPasswordResetTokens(ctx context.Context) (int, error) {
	count, err := s.repos.PasswordResetToken.DeleteExpiredTokens(ctx)
	if err != nil {
		return 0, &AuthError{Op: "cleanup expired password reset tokens", Err: err}
	}
	return count, nil
}

// CleanupExpiredRateLimits purges stale password reset rate limit windows.
func (s *Service) CleanupExpiredRateLimits(ctx context.Context) (int, error) {
	if s.repos.PasswordResetRateLimit == nil {
		return 0, nil
	}

	count, err := s.repos.PasswordResetRateLimit.CleanupExpired(ctx)
	if err != nil {
		return 0, &AuthError{Op: "cleanup password reset rate limits", Err: err}
	}

	log.Printf("Password reset rate limit cleanup removed %d records", count)
	return count, nil
}
