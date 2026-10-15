package postgres

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/uptrace/bun"
)

// The fixed retention of short-lived sign-in data (#3954). Each statement
// deletes the rows whose window ended before the instant the application
// computed; the token cleanup runs them administratively for every school.

func (s *Store) DeleteExpiredGuardianInvitations(ctx context.Context, before time.Time) (int, domain.OperationStats, error) {
	rows, stats, err := s.exec(ctx, "delete expired guardian invitations", func(db bun.IDB) executor {
		return db.NewDelete().Model((*guardianInvitationRow)(nil)).
			Where(`COALESCE("guardian_invitation".accepted_at, "guardian_invitation".expires_at) < ?`, before)
	})
	return int(rows), stats, err
}

func (s *Store) DeleteExpiredAccountMFAEmailChallenges(ctx context.Context, before time.Time) (int, domain.OperationStats, error) {
	rows, stats, err := s.exec(ctx, "delete expired account mfa email challenges", func(db bun.IDB) executor {
		return db.NewDelete().TableExpr("auth.mfa_email_challenges").Where("expires_at < ?", before)
	})
	return int(rows), stats, err
}

func (s *Store) DeleteExpiredOperatorMFAEmailChallenges(ctx context.Context, before time.Time) (int, domain.OperationStats, error) {
	rows, stats, err := s.exec(ctx, "delete expired operator mfa email challenges", func(db bun.IDB) executor {
		return db.NewDelete().TableExpr("platform.operator_mfa_email_challenges").Where("expires_at < ?", before)
	})
	return int(rows), stats, err
}

func (s *Store) DeleteExpiredAccountPasskeySessions(ctx context.Context, before time.Time) (int, domain.OperationStats, error) {
	rows, stats, err := s.exec(ctx, "delete expired account passkey sessions", func(db bun.IDB) executor {
		return db.NewDelete().TableExpr("auth.passkey_sessions").Where("expires_at < ?", before)
	})
	return int(rows), stats, err
}

func (s *Store) DeleteExpiredOperatorPasskeySessions(ctx context.Context, before time.Time) (int, domain.OperationStats, error) {
	rows, stats, err := s.exec(ctx, "delete expired operator passkey sessions", func(db bun.IDB) executor {
		return db.NewDelete().TableExpr("platform.operator_passkey_sessions").Where("expires_at < ?", before)
	})
	return int(rows), stats, err
}
