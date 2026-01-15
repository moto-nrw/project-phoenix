package auth

import (
	auditModels "github.com/moto-nrw/project-phoenix/internal/core/domain/audit"
	authModels "github.com/moto-nrw/project-phoenix/internal/core/domain/auth"
	userModels "github.com/moto-nrw/project-phoenix/internal/core/domain/users"
)

// Repositories groups the interfaces needed by auth services.
type Repositories struct {
	Account                authModels.AccountRepository
	AccountParent          authModels.AccountParentRepository
	Role                   authModels.RoleRepository
	Permission             authModels.PermissionRepository
	RolePermission         authModels.RolePermissionRepository
	AccountRole            authModels.AccountRoleRepository
	AccountPermission      authModels.AccountPermissionRepository
	Token                  authModels.TokenRepository
	PasswordResetToken     authModels.PasswordResetTokenRepository
	PasswordResetRateLimit authModels.PasswordResetRateLimitRepository
	InvitationToken        authModels.InvitationTokenRepository
	GuardianInvitation     authModels.GuardianInvitationRepository
	Person                 userModels.PersonRepository
	AuthEvent              auditModels.AuthEventRepository
}
