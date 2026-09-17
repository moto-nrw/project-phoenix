package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Identity & Access owns the password reset flows (#2722). This file binds
// the reset link mail to the Delivery dispatcher with the portal hosts the
// root knows, and serves the retained auth service's consumer-owned port
// over the public module.

var passwordResetEmailBackoff = []time.Duration{
	time.Second,
	5 * time.Second,
	15 * time.Second,
}

// cleanupResetExpiry configures the reset flows of the maintenance roots,
// which only clean up and never issue a link.
const cleanupResetExpiry = time.Hour

// passwordResetWiring is the configuration the reset flows are composed
// with. dispatcher may be nil: the link is then not mailed, as before.
type passwordResetWiring struct {
	dispatcher       *email.Dispatcher
	defaultFrom      email.Email
	staffURL         string
	parentsURL       string
	schoolURL        string
	expiry           time.Duration
	rateLimitEnabled bool
	// backoff spaces the send retries; nil uses the production spacing.
	backoff []time.Duration
}

func passwordResetDependencies(wiring *passwordResetWiring, records func() identityaccess.PasswordResets, logger *slog.Logger) *identityaccessCompose.PasswordResetDependencies {
	if wiring == nil {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	backoff := wiring.backoff
	if backoff == nil {
		backoff = passwordResetEmailBackoff
	}
	return &identityaccessCompose.PasswordResetDependencies{
		Delivery: passwordResetDelivery{
			dispatcher: wiring.dispatcher, from: wiring.defaultFrom, records: records, logger: logger,
			backoff: backoff,
			hosts: map[identityaccess.PasswordResetScope]string{
				identityaccess.PasswordResetScopeStaff:  wiring.staffURL,
				identityaccess.PasswordResetScopeParent: wiring.parentsURL,
				identityaccess.PasswordResetScopeSchool: wiring.schoolURL,
			},
		},
		Passwords:        passwordPolicy{},
		Expiry:           wiring.expiry,
		RateLimitEnabled: wiring.rateLimitEnabled,
		Logger:           logger,
	}
}

// passwordResetDelivery mails a reset link and records the outcome through
// the module once the send settles.
type passwordResetDelivery struct {
	dispatcher *email.Dispatcher
	from       email.Email
	hosts      map[identityaccess.PasswordResetScope]string
	backoff    []time.Duration
	records    func() identityaccess.PasswordResets
	logger     *slog.Logger
}

func (d passwordResetDelivery) DispatchPasswordReset(ctx context.Context, link identityaccess.PasswordResetLink, recipient string, scope identityaccess.PasswordResetScope, expiry time.Duration) {
	if d.dispatcher == nil {
		d.logger.Warn("email dispatcher unavailable, skipping password reset email",
			slog.Int64("account_id", link.AccountID))
		return
	}
	frontendURL := strings.TrimRight(d.hosts[scope], "/")
	message := email.Message{
		From:     d.from,
		To:       email.NewEmail("", recipient),
		Subject:  "Passwort zurücksetzen",
		Template: "password-reset.html",
		Content: map[string]any{
			"ResetURL":      fmt.Sprintf("%s/reset-password?token=%s", frontendURL, link.Token),
			"ExpiryMinutes": int(expiry.Minutes()),
			"LogoURL":       fmt.Sprintf("%s/images/moto-logo-mit-schriftzug.png", frontendURL),
		},
	}
	meta := email.DeliveryMetadata{
		Type:        "password_reset",
		ReferenceID: link.ID,
		Token:       link.Token,
		Recipient:   recipient,
	}
	baseRetry := link.Delivery.RetryCount
	// Async delivery must outlive the HTTP request without retaining its tx.
	detached := tenant.ContextWithoutAfterCommitHooks(tenant.ContextWithoutTransaction(context.WithoutCancel(ctx)))
	d.dispatcher.Dispatch(detached, email.DeliveryRequest{
		Message:       message,
		Metadata:      meta,
		BackoffPolicy: d.backoff,
		MaxAttempts:   3,
		Callback: func(cbCtx context.Context, result email.DeliveryResult) {
			d.recordDelivery(cbCtx, meta, baseRetry, result)
		},
	})
}

func (d passwordResetDelivery) recordDelivery(ctx context.Context, meta email.DeliveryMetadata, baseRetry int, result email.DeliveryResult) {
	delivery := identityaccess.TokenDelivery{RetryCount: baseRetry + result.Attempt}
	if result.Status == email.DeliveryStatusSent {
		sentAt := result.SentAt
		delivery.SentAt = &sentAt
	} else if result.Err != nil {
		message := strings.TrimSpace(result.Err.Error())
		delivery.Error = &message
	}
	if err := d.records().RecordPasswordResetDelivery(ctx, meta.ReferenceID, delivery); err != nil {
		d.logger.Error("failed to update password reset delivery status",
			slog.Int64("token_id", meta.ReferenceID),
			slog.Any("error", err),
		)
		return
	}
	if result.Final && result.Status == email.DeliveryStatusFailed {
		d.logger.Error("password reset email permanently failed",
			slog.Int64("token_id", meta.ReferenceID),
			slog.String("recipient", meta.Recipient),
			slog.Any("error", result.Err),
		)
	}
}

// --- the retained auth service's consumer-owned reset port -----------------

var _ auth.PasswordResets = (*accountSessions)(nil)

func (a *accountSessions) InitiatePasswordReset(ctx context.Context, email string, scope auth.PasswordResetScope) (*auth.PasswordResetLink, error) {
	link, err := a.module.InitiatePasswordReset(ctx, email, identityaccess.PasswordResetScope(scope))
	if err != nil || link == nil {
		return nil, passwordResetServiceError(err)
	}
	return &auth.PasswordResetLink{
		ID: link.ID, AccountID: link.AccountID, Token: link.Token, Expiry: link.Expiry, CreatedAt: link.CreatedAt,
		EmailSentAt: link.Delivery.SentAt, EmailError: link.Delivery.Error, EmailRetryCount: link.Delivery.RetryCount,
	}, nil
}

func (a *accountSessions) ResetPassword(ctx context.Context, token, newPassword string) error {
	return passwordResetServiceError(a.module.ResetPassword(ctx, token, newPassword))
}

func (a *accountSessions) CleanupExpiredPasswordResetTokens(ctx context.Context) (int, error) {
	deleted, err := a.module.DeleteSpentPasswordResetTokens(ctx)
	return deleted, passwordResetServiceError(err)
}

func (a *accountSessions) CleanupExpiredRateLimits(ctx context.Context) (int, error) {
	deleted, err := a.module.DeleteStalePasswordResetWindows(ctx)
	return deleted, passwordResetServiceError(err)
}

// passwordResetServiceError translates the rate-limit rejection to the
// retained *auth.RateLimitError the reset routes read Retry-After from, and
// everything else like the session flows.
func passwordResetServiceError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, identityaccess.ErrPasswordResetUnavailable) {
		return &auth.AuthError{Op: "password reset", Err: auth.ErrPasswordResetUnavailable}
	}
	var operation *identityaccess.AuthenticationError
	if errors.As(err, &operation) && operation == err {
		return &auth.AuthError{Op: operation.Op, Err: passwordResetServiceError(operation.Err)}
	}
	var limited *identityaccess.PasswordResetRateLimitError
	if errors.As(err, &limited) {
		return &auth.RateLimitError{Err: auth.ErrRateLimitExceeded, Attempts: limited.Attempts, RetryAt: limited.RetryAt}
	}
	return authServiceError(err)
}
