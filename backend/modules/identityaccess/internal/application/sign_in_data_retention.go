package application

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// Fixed retention windows of short-lived sign-in data (#3954), the same for
// every school; docs/reference/loeschfristen.md lists them.
const (
	// GuardianInvitationRetention keeps an invitation for parents 30 days
	// after its redemption, or after its expiry when nobody redeemed it.
	GuardianInvitationRetention = 30 * 24 * time.Hour
	// SignInChallengeRetention keeps an e-mail code, with the IP address it
	// was requested from, and a passkey ceremony 7 days after their expiry.
	SignInChallengeRetention = 7 * 24 * time.Hour
)

// SignInDataRetention deletes the short-lived sign-in data of accounts and
// operators once its fixed window has passed. The hourly token cleanup runs
// it; it needs no flow dependencies, so every composition serves it.
type SignInDataRetention struct {
	run    func(context.Context, func(context.Context) error) error
	store  ports.SignInDataRetentionStore
	now    func() time.Time
	logger *slog.Logger
}

func NewSignInDataRetention(store ports.SignInDataRetentionStore, logger *slog.Logger, run func(context.Context, func(context.Context) error) error) *SignInDataRetention {
	if store == nil {
		panic("identity access application: sign-in data retention requires its store")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &SignInDataRetention{store: store, now: time.Now, logger: logger, run: run}
}

func (r *SignInDataRetention) DeleteExpiredGuardianInvitations(ctx context.Context) (value0 int, err error) {
	err = r.run(ctx, func(txCtx context.Context) error {
		value0, err = r.inUnitDeleteExpiredGuardianInvitations(txCtx)
		return err
	})
	return
}

func (r *SignInDataRetention) inUnitDeleteExpiredGuardianInvitations(ctx context.Context) (int, error) {
	deleted, _, err := r.store.DeleteExpiredGuardianInvitations(ctx, r.now().Add(-GuardianInvitationRetention))
	if err != nil {
		return 0, failed("delete expired guardian invitations", err)
	}
	if deleted > 0 {
		r.logger.Info("expired guardian invitations deleted",
			slog.Int("records_deleted", deleted),
			slog.Int("retention_days", int(GuardianInvitationRetention.Hours()/24)))
	}
	return deleted, nil
}

// DeleteExpiredCatererInvitations keeps a school's invitation to its caterer
// as long as one for parents (#3873): it holds the caterer's e-mail address.
func (r *SignInDataRetention) DeleteExpiredCatererInvitations(ctx context.Context) (value0 int, err error) {
	err = r.run(ctx, func(txCtx context.Context) error {
		value0, err = r.inUnitDeleteExpiredCatererInvitations(txCtx)
		return err
	})
	return
}

func (r *SignInDataRetention) inUnitDeleteExpiredCatererInvitations(ctx context.Context) (int, error) {
	deleted, _, err := r.store.DeleteExpiredCatererInvitations(ctx, r.now().Add(-GuardianInvitationRetention))
	if err != nil {
		return 0, failed("delete expired caterer invitations", err)
	}
	r.logDeleted("expired caterer invitations deleted", deleted, GuardianInvitationRetention)
	return deleted, nil
}

func (r *SignInDataRetention) DeleteExpiredMFAEmailChallenges(ctx context.Context) (value0 int, err error) {
	err = r.run(ctx, func(txCtx context.Context) error {
		value0, err = r.inUnitDeleteExpiredMFAEmailChallenges(txCtx)
		return err
	})
	return
}

func (r *SignInDataRetention) inUnitDeleteExpiredMFAEmailChallenges(ctx context.Context) (int, error) {
	before := r.now().Add(-SignInChallengeRetention)
	accounts, _, err := r.store.DeleteExpiredAccountMFAEmailChallenges(ctx, before)
	if err != nil {
		return 0, failed("delete expired mfa email challenges", err)
	}
	operators, _, err := r.store.DeleteExpiredOperatorMFAEmailChallenges(ctx, before)
	if err != nil {
		return 0, failed("delete expired operator mfa email challenges", err)
	}
	r.logDeleted("expired mfa email challenges deleted", accounts+operators, SignInChallengeRetention)
	return accounts + operators, nil
}

func (r *SignInDataRetention) DeleteExpiredPasskeySessions(ctx context.Context) (value0 int, err error) {
	err = r.run(ctx, func(txCtx context.Context) error {
		value0, err = r.inUnitDeleteExpiredPasskeySessions(txCtx)
		return err
	})
	return
}

func (r *SignInDataRetention) inUnitDeleteExpiredPasskeySessions(ctx context.Context) (int, error) {
	before := r.now().Add(-SignInChallengeRetention)
	accounts, _, err := r.store.DeleteExpiredAccountPasskeySessions(ctx, before)
	if err != nil {
		return 0, failed("delete expired passkey sessions", err)
	}
	operators, _, err := r.store.DeleteExpiredOperatorPasskeySessions(ctx, before)
	if err != nil {
		return 0, failed("delete expired operator passkey sessions", err)
	}
	r.logDeleted("expired passkey sessions deleted", accounts+operators, SignInChallengeRetention)
	return accounts + operators, nil
}

func (r *SignInDataRetention) logDeleted(message string, deleted int, window time.Duration) {
	if deleted > 0 {
		r.logger.Info(message,
			slog.Int("records_deleted", deleted),
			slog.Int("retention_days", int(window.Hours()/24)))
	}
}
