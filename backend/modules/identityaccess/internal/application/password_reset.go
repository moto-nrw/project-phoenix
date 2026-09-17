package application

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// staleResetWindowAge keeps a rate-limit window for a day before cleanup
// removes it, so the table stays compact.
const staleResetWindowAge = 24 * time.Hour

// PasswordReset runs the password reset flows (#2722): issuing a one-time
// link for staff, parent and school accounts, setting the new password
// through it, and the cleanup of spent links and stale rate-limit windows.
// The link is mailed through the delivery port once its transaction
// committed; a new password revokes the account's sessions in the same
// transaction.
type PasswordReset struct {
	auth             *AccountAuthentication
	store            ports.PasswordResetStore
	logins           ports.AccountLoginStore
	passwords        ports.PasswordPolicy
	delivery         ports.PasswordResetDelivery
	runtime          ports.Runtime
	expiry           time.Duration
	rateLimitEnabled bool
	now              func() time.Time
	logger           *slog.Logger
}

// PasswordResetDependencies are the ports and settings the flows consume.
type PasswordResetDependencies struct {
	Store            ports.PasswordResetStore
	Logins           ports.AccountLoginStore
	Passwords        ports.PasswordPolicy
	Delivery         ports.PasswordResetDelivery
	Runtime          ports.Runtime
	Expiry           time.Duration
	RateLimitEnabled bool
	Logger           *slog.Logger
}

// NewPasswordReset composes the flows over the account authentication, whose
// portal-role lookups and session revocation the flows reuse.
func NewPasswordReset(auth *AccountAuthentication, deps PasswordResetDependencies) (*PasswordReset, error) {
	switch {
	case auth == nil:
		return nil, fmt.Errorf("identity access password reset: account authentication is required")
	case deps.Store == nil, deps.Logins == nil:
		return nil, fmt.Errorf("identity access password reset: stores are required")
	case deps.Passwords == nil, deps.Delivery == nil, deps.Runtime == nil:
		return nil, fmt.Errorf("identity access password reset: password policy, delivery and tenant runtime are required")
	case deps.Expiry <= 0:
		return nil, fmt.Errorf("identity access password reset: a positive link expiry is required")
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &PasswordReset{
		auth: auth, store: deps.Store, logins: deps.Logins, passwords: deps.Passwords, delivery: deps.Delivery,
		runtime: deps.Runtime, expiry: deps.Expiry, rateLimitEnabled: deps.RateLimitEnabled, now: time.Now, logger: logger,
	}, nil
}

// InitiatePasswordReset issues a reset link for the address and mails it. An
// unknown address, or an account without a role of the scope's portal,
// yields no link and no error, so the response never reveals whether an
// account exists. The rate limit counts only requests that would send.
func (r *PasswordReset) InitiatePasswordReset(ctx context.Context, email string, scope domain.PasswordResetScope) (*domain.PasswordResetToken, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	account, found, _, err := r.logins.FindLoginAccountByEmail(ctx, email)
	if err != nil {
		// The answer stays neutral, but a failing lookup is an incident, not
		// an unknown address: it must be visible in the logs.
		r.logger.Error("password reset: account lookup failed, answering neutrally",
			slog.String("scope", string(scope)),
			slog.Any("error", err),
		)
		return nil, nil
	}
	if !found {
		return nil, nil
	}
	eligible, err := r.portalEligible(ctx, account.ID, scope)
	if err != nil || !eligible {
		return nil, err
	}
	// Rate-limit only after confirming a real, actionable account: keying the
	// limiter on accounts that will be mailed keeps an unauthenticated caller
	// from filling the table with arbitrary addresses. The IP-keyed limiter in
	// front of the public routes bounds volumetric probing.
	if err := r.checkRateLimit(ctx, email); err != nil {
		return nil, err
	}
	r.logger.Info("password reset requested",
		slog.String("scope", string(scope)))

	var token domain.PasswordResetToken
	err = r.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		if _, revokeErr := r.store.RevokePasswordResetTokens(txCtx, account.ID); revokeErr != nil {
			r.logger.Error("failed to invalidate reset tokens, rolling back",
				slog.Int64("account_id", account.ID),
				slog.Any("error", revokeErr),
			)
			return revokeErr
		}
		now := r.now()
		pending := domain.PasswordResetToken{AccountID: account.ID, Token: uuid.Must(uuid.NewV4()).String(), Expiry: now.Add(r.expiry)}
		if validateErr := pending.Validate(now); validateErr != nil {
			return validateErr
		}
		stored, _, insertErr := r.store.InsertPasswordResetToken(txCtx, pending)
		token = stored
		return insertErr
	})
	if err != nil {
		return nil, failed("initiate password reset transaction", err)
	}
	r.logger.Info("password reset token created",
		slog.Int64("account_id", account.ID))

	r.delivery.DispatchPasswordReset(ctx, token, account.Email, scope, r.expiry)
	return &token, nil
}

func (r *PasswordReset) portalEligible(ctx context.Context, accountID int64, scope domain.PasswordResetScope) (bool, error) {
	switch scope {
	case domain.PasswordResetScopeParent:
		eligible, _, err := r.auth.FindGuardianTenant(ctx, accountID)
		return eligible, err
	case domain.PasswordResetScopeSchool:
		eligible, _, err := r.auth.FindSchoolPortalTenant(ctx, accountID)
		return eligible, err
	default:
		return true, nil
	}
}

// checkRateLimit rejects a request while the window already holds the
// threshold, and again after counting when this request crossed it.
func (r *PasswordReset) checkRateLimit(ctx context.Context, email string) error {
	if !r.rateLimitEnabled {
		return nil
	}
	window, _, _, err := r.store.PasswordResetWindow(ctx, email)
	if err != nil {
		return failed("check password reset rate limit", err)
	}
	if window.Exceeded(domain.PasswordResetRateLimitThreshold-1, r.now()) {
		return failed("initiate password reset", &domain.PasswordResetRateLimitError{Attempts: window.Attempts, RetryAt: window.RetryAt})
	}
	window, _, err = r.store.CountPasswordResetRequest(ctx, email)
	if err != nil {
		return failed("increment password reset rate limit", err)
	}
	if window.Exceeded(domain.PasswordResetRateLimitThreshold, r.now()) {
		return failed("initiate password reset", &domain.PasswordResetRateLimitError{Attempts: window.Attempts, RetryAt: window.RetryAt})
	}
	return nil
}

// ResetPassword sets the new password through a redeemable link. The
// password change, the spent link and the revocation of every session at
// the account's schools commit together.
func (r *PasswordReset) ResetPassword(ctx context.Context, token, newPassword string) error {
	link, found, _, err := r.store.FindRedeemablePasswordResetToken(ctx, token, r.now())
	if err != nil || !found {
		return failed("reset password", domain.ErrInvalidToken)
	}
	if err := r.passwords.ValidatePasswordStrength(newPassword); err != nil {
		return failed("reset password", err)
	}
	hash, err := r.passwords.HashPassword(newPassword)
	if err != nil {
		return failed("hash password", err)
	}
	// The flow runs before authentication, so it holds the administrative
	// transaction: the session revocation touches RLS-guarded auth.tokens.
	err = r.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		if _, _, setErr := r.store.SetAccountPassword(txCtx, link.AccountID, hash); setErr != nil {
			return setErr
		}
		redeemed, _, redeemErr := r.store.RedeemPasswordResetToken(txCtx, link.ID)
		if redeemErr != nil {
			return redeemErr
		}
		if !redeemed {
			return domain.ErrInvalidToken
		}
		if _, revokeErr := r.auth.DeleteAccountSessionsWithAudit(txCtx, link.AccountID, "password_reset", "", ""); revokeErr != nil {
			return fmt.Errorf("revoke tokens during password reset: %w", revokeErr)
		}
		return nil
	})
	if err != nil {
		return failed("reset password transaction", err)
	}
	return nil
}

// RecordPasswordResetDelivery stores the outcome of mailing a link.
func (r *PasswordReset) RecordPasswordResetDelivery(ctx context.Context, id int64, delivery domain.TokenDelivery) error {
	_, err := r.store.RecordPasswordResetDelivery(ctx, id, delivery.Bounded())
	return err
}

// DeleteSpentPasswordResetTokens removes expired and used links.
func (r *PasswordReset) DeleteSpentPasswordResetTokens(ctx context.Context) (int, error) {
	deleted, _, err := r.store.DeleteSpentPasswordResetTokens(ctx, r.now())
	if err != nil {
		return 0, failed("cleanup expired password reset tokens", err)
	}
	return deleted, nil
}

// DeleteStalePasswordResetWindows removes rate-limit windows older than a day.
func (r *PasswordReset) DeleteStalePasswordResetWindows(ctx context.Context) (int, error) {
	deleted, _, err := r.store.DeleteStalePasswordResetWindows(ctx, r.now().Add(-staleResetWindowAge))
	if err != nil {
		return 0, failed("cleanup password reset rate limits", err)
	}
	r.logger.Info("password reset rate limit cleanup completed",
		slog.Int("records_deleted", deleted))
	return deleted, nil
}
