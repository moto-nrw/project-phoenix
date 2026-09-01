package auth

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// CleanupDependencies is the complete dependency set used by token and
// password-reset rate-limit maintenance. It intentionally excludes the login,
// MFA, mail, role, and invitation graph required by NewService.
type CleanupDependencies struct {
	Account                authModels.AccountRepository
	Token                  authModels.TokenRepository
	PasswordResetRateLimit authModels.PasswordResetRateLimitRepository
	AuthEvent              auditModels.AuthEventRepository
	Audit                  auditModels.Command
	PushSubscription       iotModels.PushSubscriptionRepository
	DB                     *bun.DB
	Logger                 *slog.Logger
	TenantRuntime          tenant.UnitOfWork
}

func NewCleanupService(deps CleanupDependencies) *Service {
	service := &Service{
		repos: &repositories.Factory{
			Account:                deps.Account,
			Token:                  deps.Token,
			PasswordResetRateLimit: deps.PasswordResetRateLimit,
			AuthEvent:              deps.AuthEvent,
			PushSubscription:       deps.PushSubscription,
		},
		db:     deps.DB,
		logger: deps.Logger,
		audit:  deps.Audit,
	}
	service.SetTenantRuntime(deps.TenantRuntime)
	return service
}
