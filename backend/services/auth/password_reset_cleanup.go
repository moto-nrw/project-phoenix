package auth

import (
	"context"
	"log/slog"
)

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

	s.getLogger().Info("password reset rate limit cleanup completed",
		slog.Int("records_deleted", count))
	return count, nil
}
