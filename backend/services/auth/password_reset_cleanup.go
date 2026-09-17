package auth

import (
	"log/slog"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// CleanupDependencies is the complete dependency set used by token and
// password-reset rate-limit maintenance. It intentionally excludes the login,
// MFA, mail, role, and invitation graph required by NewService; the expired
// session sweep, the revocation follow-ups (#3251) and the password reset
// cleanup (#2722) run through the Identity & Access ports.
type CleanupDependencies struct {
	Sessions      AccountSessions
	Resets        PasswordResets
	Audit         auditModels.Command
	DB            *bun.DB
	Logger        *slog.Logger
	TenantRuntime tenant.UnitOfWork
}

func NewCleanupService(deps CleanupDependencies) *Service {
	service := &Service{
		db:       deps.DB,
		logger:   deps.Logger,
		audit:    deps.Audit,
		sessions: deps.Sessions,
		resets:   deps.Resets,
	}
	service.SetTenantRuntime(deps.TenantRuntime)
	return service
}
