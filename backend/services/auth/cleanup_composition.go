package auth

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

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
