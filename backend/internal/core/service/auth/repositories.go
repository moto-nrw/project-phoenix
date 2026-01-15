package auth

import (
	auditPort "github.com/moto-nrw/project-phoenix/internal/core/port/audit"
	authPort "github.com/moto-nrw/project-phoenix/internal/core/port/auth"
	userPort "github.com/moto-nrw/project-phoenix/internal/core/port/users"
)

// Repositories groups the interfaces needed by auth services.
type Repositories struct {
	Account                authPort.AccountRepository
	AccountParent          authPort.AccountParentRepository
	Role                   authPort.RoleRepository
	Permission             authPort.PermissionRepository
	RolePermission         authPort.RolePermissionRepository
	AccountRole            authPort.AccountRoleRepository
	AccountPermission      authPort.AccountPermissionRepository
	Token                  authPort.TokenRepository
	PasswordResetToken     authPort.PasswordResetTokenRepository
	PasswordResetRateLimit authPort.PasswordResetRateLimitRepository
	InvitationToken        authPort.InvitationTokenRepository
	GuardianInvitation     authPort.GuardianInvitationRepository
	Person                 userPort.PersonRepository
	AuthEvent              auditPort.AuthEventRepository
}
