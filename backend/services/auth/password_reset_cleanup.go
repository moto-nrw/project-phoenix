package auth

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
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

// CleanupDependencies is the complete dependency set used by token and
// password-reset rate-limit maintenance. It intentionally excludes the login,
// MFA, mail, role, and invitation graph required by NewService; the expired
// session sweep and the revocation follow-ups run through the Identity &
// Access port (#3251).
type CleanupDependencies struct {
	PasswordResetRateLimit authModels.PasswordResetRateLimitRepository
	Sessions               AccountSessions
	Audit                  auditModels.Command
	DB                     *bun.DB
	Logger                 *slog.Logger
	TenantRuntime          tenant.UnitOfWork
}

func NewCleanupService(deps CleanupDependencies) *Service {
	service := &Service{
		repos: &repositories.Factory{
			PasswordResetRateLimit: deps.PasswordResetRateLimit,
		},
		db:       deps.DB,
		logger:   deps.Logger,
		audit:    deps.Audit,
		sessions: deps.Sessions,
	}
	service.SetTenantRuntime(deps.TenantRuntime)
	return service
}
