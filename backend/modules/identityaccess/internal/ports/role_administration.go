package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// RoleStore is the persistence port over the identity-owned role and
// permission rows the role administration (#3314) reads and writes:
// auth.roles, auth.permissions, auth.role_permissions, auth.account_roles,
// auth.account_permissions and the account and school mapping locks the
// mutations serialize on. Every statement runs on the connection the
// caller's context carries and applies that context's tenant filter.
//
// Lookups report found=false only for a missing row; every other failure is
// an error the flows keep in their chain.
type RoleStore interface {
	PermissionStore
	CreateRole(ctx context.Context, role domain.ManagedRole) (domain.ManagedRole, error)
	FindRole(ctx context.Context, id int64) (domain.ManagedRole, bool, error)
	// FindRoleIgnoringTenant resolves the role without the tenant filter, so a
	// system role (tenant_id NULL) stays resolvable inside a tenant
	// transaction.
	FindRoleIgnoringTenant(ctx context.Context, id int64) (domain.ManagedRole, bool, error)
	// FindSystemRoleByName resolves the platform system role with that name,
	// matched case-insensitively; a school's own role never matches.
	FindSystemRoleByName(ctx context.Context, name string) (domain.ManagedRole, bool, error)
	// FindRoleForUpdate locks the role row with the visibility FindRole has.
	FindRoleForUpdate(ctx context.Context, id int64) (domain.ManagedRole, bool, error)
	UpdateRole(ctx context.Context, role domain.ManagedRole) error
	DeleteRole(ctx context.Context, id int64) error
	ListRoles(ctx context.Context, filter domain.RoleFilter) ([]domain.ManagedRole, error)
	// ListAccountRoles returns the roles the account holds at the tenant in
	// context.
	ListAccountRoles(ctx context.Context, accountID int64) ([]domain.ManagedRole, error)
	ListAccountRoleNames(ctx context.Context, accountIDs []int64) (map[int64]string, error)

	// AccountHoldsRole reports whether the account-role mapping exists.
	AccountHoldsRole(ctx context.Context, accountID, roleID int64) (bool, error)
	CreateAccountRole(ctx context.Context, accountID, roleID, tenantID int64) error
	DeleteAccountRole(ctx context.Context, accountID, roleID int64) error
	DeleteRoleAssignments(ctx context.Context, roleID int64) error

	// LockAccount locks the account row for the caller's transaction.
	LockAccount(ctx context.Context, accountID int64) (bool, error)
	// HasTenantMembership reports whether the account is mapped to the
	// tenant. With share it only counts an active mapping and locks it FOR
	// SHARE, so a concurrent revocation waits for the caller.
	HasTenantMembership(ctx context.Context, accountID, tenantID int64, share bool) (bool, error)
	ListAccountEmails(ctx context.Context, accountIDs []int64) (map[int64]string, error)
	ListAccountAvatars(ctx context.Context, accountIDs []int64) (map[int64]string, error)
}

// RoleAssignmentPolicy classifies roles for the assignment rules; the public
// package owns the decisions, the composition binds them.
type RoleAssignmentPolicy interface {
	IsLehrkraftSystemRole(role *domain.RoleFacts) bool
	IsGuardianTierRole(role *domain.RoleFacts) bool
	// IsPlatformCaregiverRole reports whether the role already is the
	// platform caregiver role, so the caregiver upgrade adds nothing.
	IsPlatformCaregiverRole(role *domain.RoleFacts) bool
	// ValidateAssignableSchoolRole returns the public policy sentinel that
	// refuses the role for tenantID, or nil.
	ValidateAssignableSchoolRole(role *domain.RoleFacts, tenantID int64) error
}

// SchoolIdentity completes the school identity chain a role requires on the
// caller's transaction.
type SchoolIdentity interface {
	EnsureSchoolIdentity(ctx context.Context, input domain.SchoolIdentityInput) (*domain.SchoolIdentity, error)
}

// SessionRevocation revokes an account's sessions at the school on the
// caller's transaction and queues the push cleanup once it committed.
type SessionRevocation interface {
	DeleteAccountSessionsWithAudit(ctx context.Context, accountID int64, reason, ipAddress, userAgent string) ([]domain.AccountSession, error)
	QueuePushCleanup(ctx context.Context, accountID int64, sessions []domain.AccountSession, reason string)
}

// CaregiverProfiles answers whether the account's identity at the tenant in
// context carries a live caregiver profile (users.teachers), the fact every
// path that guards the Lehrkraft role reads (#1772).
type CaregiverProfiles interface {
	HasLiveCaregiverProfile(ctx context.Context, accountID int64) (bool, error)
}

// PermissionStore persists the permission catalog, role selections and direct account grants.
// Statements use the caller's connection and scope; application flows own locking and transactions.
type PermissionStore interface {
	DeleteRolePermissions(ctx context.Context, roleID int64) error
	CreatePermission(ctx context.Context, permission domain.ManagedPermission) (domain.ManagedPermission, error)
	FindPermission(ctx context.Context, id int64) (domain.ManagedPermission, bool, error)
	FindPermissionByName(ctx context.Context, name string) (domain.ManagedPermission, error)
	UpdatePermission(ctx context.Context, permission domain.ManagedPermission) error
	DeletePermission(ctx context.Context, id int64) error
	ListPermissions(ctx context.Context, filter domain.PermissionFilter) ([]domain.ManagedPermission, error)
	ListRolePermissions(ctx context.Context, roleID int64) ([]domain.ManagedPermission, error)
	// ListAccountPermissions returns the direct and the role-based
	// permissions of the account.
	ListAccountPermissions(ctx context.Context, accountID int64) ([]domain.ManagedPermission, error)
	ListAccountDirectPermissions(ctx context.Context, accountID int64) ([]domain.ManagedPermission, error)
	AssignRolePermission(ctx context.Context, roleID, permissionID int64) error
	RemoveRolePermission(ctx context.Context, roleID, permissionID int64) error
	DeletePermissionAssignments(ctx context.Context, permissionID int64) error
	DeletePermissionGrants(ctx context.Context, permissionID int64) error
	GrantAccountPermission(ctx context.Context, accountID, permissionID int64) error
	DenyAccountPermission(ctx context.Context, accountID, permissionID int64) error
	RemoveAccountPermission(ctx context.Context, accountID, permissionID int64) error
}

// ManageableAccounts applies the account administration's visibility rules.
// Missing and out-of-scope accounts return ErrAccountNotFound.
type ManageableAccounts interface {
	FindManageableAccount(ctx context.Context, accountID int64) (domain.ManagedAccountRecord, error)
}
