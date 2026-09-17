package auth

import (
	"log/slog"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// CleanupDependencies is the complete dependency set used by the session
// token maintenance. It intentionally excludes the login, MFA and mail graph
// required by NewService; the expired session sweep and the revocation
// follow-ups run through the Identity & Access session port (#3251), and the
// password reset maintenance is called on the module directly (#3332).
type CleanupDependencies struct {
	Sessions      AccountSessions
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
	}
	service.SetTenantRuntime(deps.TenantRuntime)
	return service
}
