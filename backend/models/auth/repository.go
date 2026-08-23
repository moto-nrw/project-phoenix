package auth

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// AccountRepository defines operations for managing accounts
type AccountRepository interface {
	base.CRUDRepository[*Account]
	FindManageableByID(ctx context.Context, id int64) (*Account, error)
	ListManageable(ctx context.Context, filters map[string]interface{}) ([]*Account, error)
	UpdateManageable(ctx context.Context, account *Account) error
	FindByIDForUpdate(ctx context.Context, id int64) (*Account, error)
	FindByEmail(ctx context.Context, email string) (*Account, error)
	FindByUsername(ctx context.Context, username string) (*Account, error)
	// FindByCalendarFeedToken resolves the account owning an iCalendar
	// subscription token. Returns (nil, nil) when no account matches.
	FindByCalendarFeedToken(ctx context.Context, token string) (*Account, error)
	// SetCalendarFeedToken sets (or rotates) the account's calendar feed token.
	SetCalendarFeedToken(ctx context.Context, accountID int64, token string) error
	// EnsureCalendarFeedToken atomically claims newToken only if the account has
	// no token yet, then returns the persisted token. Concurrent first-time
	// callers therefore all receive the same stored value instead of a URL a
	// later write overwrote.
	EnsureCalendarFeedToken(ctx context.Context, accountID int64, newToken string) (string, error)
	UpdateLastLogin(ctx context.Context, id int64) error
	UpdatePassword(ctx context.Context, id int64, passwordHash string) error
	UpdateAvatar(ctx context.Context, id int64, avatar string) error
	SetActive(ctx context.Context, id int64, active bool) error
	// ListEffectiveAdminAccountIDs returns the IDs of active accounts with
	// effective admin scope in the current tenant: the literal admin role, or
	// an admin:* / *:* permission from a role or granted directly.
	ListEffectiveAdminAccountIDs(ctx context.Context) ([]int64, error)
	FindAccountsWithRolesAndPermissions(ctx context.Context, filters map[string]interface{}) ([]*Account, error)
	FindEmailsByAccountIDs(ctx context.Context, accountIDs []int64) (map[int64]string, error)
	FindAvatarsByAccountIDs(ctx context.Context, accountIDs []int64) (map[int64]string, error)
	// AnonymizeForDeletion overwrites the email with an anonymized
	// placeholder and clears the username (GDPR person deletion).
	AnonymizeForDeletion(ctx context.Context, accountID int64, anonymizedEmail string) error
	// IncrementMFAAttempts atomically bumps mfa_attempts by one and sets
	// mfa_locked_until = now() + lockoutDuration when the post-increment
	// value reaches threshold. The CAS-style UPDATE means N concurrent
	// failed verifies count as N, not 1, closing the lockout-bypass race
	// from #1430 review item #6. Returns the post-update counter so the
	// caller can detect "this attempt triggered the lockout" via
	// `result.Attempts == threshold`.
	IncrementMFAAttempts(ctx context.Context, id int64, threshold int, lockoutDuration time.Duration) (MFAAttemptResult, error)
	// ResetMFAAttempts atomically clears mfa_attempts + mfa_locked_until
	// after a successful verify so a single Account.Update can't
	// inadvertently overwrite a concurrent increment.
	ResetMFAAttempts(ctx context.Context, id int64) error
	// IncrementPINAttempts atomically bumps pin_attempts by one and sets
	// pin_locked_until = now() + lockoutDuration when the post-increment
	// value reaches threshold. Mirrors IncrementMFAAttempts: the CAS-style
	// UPDATE means N concurrent failed PIN entries count as N, not 1,
	// closing the read-modify-write lockout-bypass race that the previous
	// model-level Account.IncrementPINAttempts() suffered (issue #586).
	IncrementPINAttempts(ctx context.Context, id int64, threshold int, lockoutDuration time.Duration) (PINAttemptResult, error)
	// ResetPINAttempts atomically clears pin_attempts + pin_locked_until
	// after a successful PIN verify.
	ResetPINAttempts(ctx context.Context, id int64) error
}

// MFAAttemptResult is the post-update snapshot returned by
// AccountRepository.IncrementMFAAttempts. Used by callers to decide
// whether the increment triggered the lockout transition.
type MFAAttemptResult struct {
	Attempts    int
	LockedUntil *time.Time
}

// PINAttemptResult is the post-update snapshot returned by
// AccountRepository.IncrementPINAttempts. Mirrors MFAAttemptResult so the
// caller can tell whether this attempt crossed the lockout threshold.
type PINAttemptResult struct {
	Attempts    int
	LockedUntil *time.Time
}

// RoleRepository defines operations for managing roles
type RoleRepository interface {
	base.CRUDRepository[*Role]
	FindByName(ctx context.Context, name string) (*Role, error)
	FindByAccountID(ctx context.Context, accountID int64) ([]*Role, error)
	FindRoleNamesByAccountIDs(ctx context.Context, accountIDs []int64) (map[int64]string, error)
	AssignRoleToAccount(ctx context.Context, accountID int64, roleID int64) error
	RemoveRoleFromAccount(ctx context.Context, accountID int64, roleID int64) error
}

// PermissionRepository defines operations for managing permissions
type PermissionRepository interface {
	base.CRUDRepository[*Permission]
	FindByName(ctx context.Context, name string) (*Permission, error)
	FindByAccountID(ctx context.Context, accountID int64) ([]*Permission, error)
	FindByAccountIDForTenant(ctx context.Context, accountID int64, tenantID int64) ([]*Permission, error)
	// LockAccountPermissionSourcesForTenant takes FOR SHARE locks on the
	// direct grants and role-permission rows that make up the account's
	// effective permissions at a tenant. Transaction-only: it lets a caller
	// read the permission set and write a token derived from it without a
	// concurrent revocation committing in between.
	LockAccountPermissionSourcesForTenant(ctx context.Context, accountID int64, tenantID int64) error
	FindDirectByAccountID(ctx context.Context, accountID int64) ([]*Permission, error)
	FindByRoleID(ctx context.Context, roleID int64) ([]*Permission, error)
	AssignPermissionToRole(ctx context.Context, roleID int64, permissionID int64) error
	RemovePermissionFromRole(ctx context.Context, roleID int64, permissionID int64) error
}

// AccountParentRepository defines operations for managing parent accounts
type AccountParentRepository interface {
	base.CRUDRepository[*AccountParent]
	FindByEmail(ctx context.Context, email string) (*AccountParent, error)
	FindByUsername(ctx context.Context, username string) (*AccountParent, error)
	UpdateLastLogin(ctx context.Context, id int64) error
	UpdatePassword(ctx context.Context, id int64, passwordHash string) error
}

// RolePermissionRepository defines operations for managing role-permission mappings
type RolePermissionRepository interface {
	base.CRUDRepository[*RolePermission]
	FindByRoleID(ctx context.Context, roleID int64) ([]*RolePermission, error)
	DeleteByRoleID(ctx context.Context, roleID int64) error
	DeleteByPermissionID(ctx context.Context, permissionID int64) error
}

// AccountRoleRepository defines operations for managing account-role mappings
type AccountRoleRepository interface {
	base.CRUDRepository[*AccountRole]
	FindByAccountID(ctx context.Context, accountID int64) ([]*AccountRole, error)
	FindByAccountIDForTenant(ctx context.Context, accountID int64, tenantID int64) ([]*AccountRole, error)
	// FindByAccountIDForTenantForShare is FindByAccountIDForTenant with a FOR
	// SHARE row lock. Transaction-only: it lets a caller re-check a role and
	// write in the same transaction without a concurrent revocation slipping
	// in between.
	FindByAccountIDForTenantForShare(ctx context.Context, accountID int64, tenantID int64) ([]*AccountRole, error)
	FindByRoleID(ctx context.Context, roleID int64) ([]*AccountRole, error)
	FindByAccountAndRole(ctx context.Context, accountID, roleID int64) (*AccountRole, error)
	DeleteByAccountAndRole(ctx context.Context, accountID, roleID int64) error
	// DeleteByAccountRoleAndTenant removes a single role assignment scoped to
	// one school. Unlike DeleteByAccountAndRole it never touches the account's
	// assignments at other schools, which is what cross-tenant access
	// management requires.
	DeleteByAccountRoleAndTenant(ctx context.Context, accountID, roleID, tenantID int64) error
	DeleteByAccountID(ctx context.Context, accountID int64) error
	DeleteByRoleID(ctx context.Context, roleID int64) error
}

// AccountPermissionRepository defines operations for managing account-permission mappings
type AccountPermissionRepository interface {
	base.CRUDRepository[*AccountPermission]
	FindByAccountID(ctx context.Context, accountID int64) ([]*AccountPermission, error)
	GrantPermission(ctx context.Context, accountID, permissionID int64) error
	DenyPermission(ctx context.Context, accountID, permissionID int64) error
	RemovePermission(ctx context.Context, accountID, permissionID int64) error
	DeleteByPermissionID(ctx context.Context, permissionID int64) error
	DeleteByAccountID(ctx context.Context, accountID int64) (int64, error)

	FindByPermissionID(ctx context.Context, permissionID int64) ([]*AccountPermission, error)

	FindByAccountAndPermission(ctx context.Context, accountID, permissionID int64) (*AccountPermission, error)
}

// TokenRepository defines operations for managing authentication tokens
type TokenRepository interface {
	base.CRUDRepository[*Token]
	FindByToken(ctx context.Context, token string) (*Token, error)
	FindByTokenForUpdate(ctx context.Context, token string) (*Token, error)
	MarkRotated(ctx context.Context, id int64, replacementToken string, recoveryProofHash []byte, rotatedAt time.Time) error
	DeleteExpiredRotatedForAccount(ctx context.Context, accountID int64, now time.Time) error
	FindByAccountID(ctx context.Context, accountID int64) ([]*Token, error)
	DeleteExpiredTokens(ctx context.Context) (int, error)
	ListInactiveAccountIDsWithLiveTokens(ctx context.Context) ([]int64, error)
	HasLiveTokensCreatedAfter(ctx context.Context, accountID int64, since time.Time) (bool, error)
	DeleteByAccountIDReturning(ctx context.Context, accountID int64) ([]*Token, error)
	DeleteAllByAccountIDReturning(ctx context.Context, accountID int64) ([]*Token, error)
	DeleteByAccountIDCreatedAtOrBeforeReturning(ctx context.Context, accountID int64, cutoff time.Time) ([]*Token, error)
	CleanupOldTokensForAccountReturning(ctx context.Context, accountID int64, portalScope string, keepCount int) ([]*Token, error)

	// Bulk deletion
	DeleteByTenantIDReturning(ctx context.Context, tenantID int64) ([]*Token, error)

	DeleteByFamilyIDReturning(ctx context.Context, familyID string) ([]*Token, error)
	GetLatestTokenInFamily(ctx context.Context, familyID string) (*Token, error)

	// Token family tracking methods
	FindByFamilyID(ctx context.Context, familyID string) ([]*Token, error)
}

// PasswordResetTokenRepository defines operations for managing password reset tokens
type PasswordResetTokenRepository interface {
	base.CRUDRepository[*PasswordResetToken]
	UpdateDeliveryResult(ctx context.Context, tokenID int64, sentAt *time.Time, emailError *string, retryCount int) error
	FindByToken(ctx context.Context, token string) (*PasswordResetToken, error)
	FindByAccountID(ctx context.Context, accountID int64) ([]*PasswordResetToken, error)
	FindValidByToken(ctx context.Context, token string) (*PasswordResetToken, error)
	MarkAsUsed(ctx context.Context, tokenID int64) error
	DeleteExpiredTokens(ctx context.Context) (int, error)
	InvalidateTokensByAccountID(ctx context.Context, accountID int64) error
}

// PasswordResetRateLimitRepository defines operations for managing password reset rate limiting.
type PasswordResetRateLimitRepository interface {
	CheckRateLimit(ctx context.Context, email string) (*RateLimitState, error)
	IncrementAttempts(ctx context.Context, email string) (*RateLimitState, error)
	CleanupExpired(ctx context.Context) (int, error)
}

// InvitationTokenRepository defines operations for managing invitation tokens.
type InvitationTokenRepository interface {
	Create(ctx context.Context, token *InvitationToken) error
	Update(ctx context.Context, token *InvitationToken) error
	FindByID(ctx context.Context, id interface{}) (*InvitationToken, error)
	FindByToken(ctx context.Context, token string) (*InvitationToken, error)
	UpdateDeliveryResult(ctx context.Context, id int64, sentAt *time.Time, emailError *string, retryCount int) error
	FindValidByToken(ctx context.Context, token string, now time.Time) (*InvitationToken, error)
	FindByEmail(ctx context.Context, email string) ([]*InvitationToken, error)
	MarkAsUsed(ctx context.Context, id int64) error
	InvalidateByEmail(ctx context.Context, email string) (int, error)
	InvalidateByTenantID(ctx context.Context, tenantID int64) (int, error)
	DeleteExpired(ctx context.Context, now time.Time) (int, error)
	List(ctx context.Context, filters map[string]interface{}) ([]*InvitationToken, error)
}

// MFACredentialRepository persists per-account MFA enrollment records.
type MFACredentialRepository interface {
	base.CRUDRepository[*MFACredential]
	FindByAccountID(ctx context.Context, accountID int64) (*MFACredential, error)
	UpdateLastUsedAt(ctx context.Context, id int64, when time.Time) error
	DeleteByAccountID(ctx context.Context, accountID int64) error
}

// MFAEmailChallengeRepository persists time-limited 6-digit email codes.
type MFAEmailChallengeRepository interface {
	Create(ctx context.Context, challenge *MFAEmailChallenge) error
	FindByID(ctx context.Context, id interface{}) (*MFAEmailChallenge, error)
	// FindActiveByAccountIDInScope returns the most recent unconsumed,
	// unexpired challenge an account holds FOR ONE PORTAL. The scope (and,
	// when known, the school) is part of the key on purpose: the enrollment
	// confirm paths have no challenge id to look up by, and an account-wide
	// "newest active code" would let a school-portal code be redeemed at the
	// tenant surface whenever both are in flight.
	FindActiveByAccountIDInScope(ctx context.Context, accountID, tenantID int64, scope string) (*MFAEmailChallenge, error)
	// FindActiveByIDForAccount returns one specific unconsumed, unexpired challenge
	// owned by the account — the lookup the challenge-token verify path uses so a
	// code is only ever redeemed against the challenge it was minted for.
	FindActiveByIDForAccount(ctx context.Context, id, accountID int64) (*MFAEmailChallenge, error)
	MarkConsumed(ctx context.Context, id int64, consumedAt time.Time) error
	// CountRecentByAccountID counts challenges issued at or after `since` (used for rate-limit checks).
	CountRecentByAccountID(ctx context.Context, accountID int64, since time.Time) (int, error)
	DeleteExpired(ctx context.Context) (int, error)
}

// MFATrustedDeviceRepository persists HMAC-signed trusted-device records.
// Every read/list/revoke that touches per-cookie state is scoped by
// (account_id, tenant_id) so a cookie issued in one tenant can never
// bypass MFA in another — see #1430 review item #9.
type MFATrustedDeviceRepository interface {
	Create(ctx context.Context, device *MFATrustedDevice) error
	// FindActiveByAccountTenantAndTokenHash is the lookup behind
	// VerifyTrustedDevice. The tenant_id filter is the security boundary:
	// the same raw cookie hash MUST NOT match across tenants.
	FindActiveByAccountTenantAndTokenHash(ctx context.Context, accountID, tenantID int64, tokenHash string) (*MFATrustedDevice, error)
	// ListActiveByAccountTenant returns the devices the user can see in
	// the currently-active tenant's settings page. Settings are per-
	// tenant so the list is too; users with cross-tenant access see
	// their devices for the tenant they're in.
	ListActiveByAccountTenant(ctx context.Context, accountID, tenantID int64) ([]*MFATrustedDevice, error)
	UpdateLastUsedAt(ctx context.Context, id int64, when time.Time) error
	Revoke(ctx context.Context, id int64, revokedAt time.Time) error
	// RevokeAllByAccountID stays account-scoped (no tenant filter) because
	// it is only used by the Disable cascade — disabling MFA on one tenant
	// removes the account-wide credential, so every cross-tenant trusted
	// device must go with it.
	RevokeAllByAccountID(ctx context.Context, accountID int64, revokedAt time.Time) error
	// RevokeAllByAccountTenant revokes every active device for the
	// (account, tenant) pair. Used by tenant-scoped admin override
	// writes that flip MFA to force_off — the operation must NOT touch
	// devices the same account trusted in other tenants.
	RevokeAllByAccountTenant(ctx context.Context, accountID, tenantID int64, revokedAt time.Time) error
	DeleteExpired(ctx context.Context) (int, error)
}

// PasskeyCredentialRepository persists WebAuthn credentials for tenant-portal accounts.
type PasskeyCredentialRepository interface {
	Create(ctx context.Context, credential *PasskeyCredential) error
	FindActiveByAccountID(ctx context.Context, accountID int64) ([]*PasskeyCredential, error)
	FindActiveByCredentialIDAndUserHandle(ctx context.Context, credentialID, userHandle []byte) (*PasskeyCredential, error)
	UpdateAfterUse(ctx context.Context, id int64, credentialJSON []byte, usedAt time.Time) error
	Revoke(ctx context.Context, accountID, id int64, revokedAt time.Time) error
}

// PasskeySessionRepository persists server-side WebAuthn ceremony state.
type PasskeySessionRepository interface {
	Create(ctx context.Context, session *PasskeySession) error
	Consume(ctx context.Context, id, purpose string, now time.Time) (*PasskeySession, error)
	DeleteExpired(ctx context.Context, now time.Time) (int, error)
}

// MFAOverrideRepository persists the per-(account, tenant) admin
// overrides plus the optional platform-wide ("operator account-wide")
// row keyed on TenantID IS NULL. Reads return nil instead of an error
// on a missing row — "no override" is a valid state for the resolver.
//
// Schema invariants enforced by partial unique indexes (migration
// 1.15.72):
//   - At most one row with tenant_id IS NULL per account.
//   - At most one row with tenant_id = X per (account, X).
type MFAOverrideRepository interface {
	// FindGlobal returns the platform-wide row (tenant_id IS NULL) for
	// the given account, or (nil, nil) when no operator override has
	// been set. Used by IsRequired as the first step of the resolution
	// chain.
	FindGlobal(ctx context.Context, accountID int64) (*MFAOverride, error)
	// FindByAccountAndTenant returns the tenant-scoped row, or (nil,
	// nil) when none exists. Step 2 of the resolver.
	FindByAccountAndTenant(ctx context.Context, accountID, tenantID int64) (*MFAOverride, error)
	// UpsertGlobal writes or replaces the platform-wide row for an
	// account. Used only by the operator account-wide endpoint.
	UpsertGlobal(ctx context.Context, override *MFAOverride) error
	// UpsertTenant writes or replaces the tenant-scoped row.
	UpsertTenant(ctx context.Context, override *MFAOverride) error
	// DeleteGlobal removes the platform-wide row if it exists.
	// Operator-only.
	DeleteGlobal(ctx context.Context, accountID int64) error
	// DeleteTenant removes the (account, tenant) row if it exists.
	DeleteTenant(ctx context.Context, accountID, tenantID int64) error
	// ListByAccount returns every override row that targets the given
	// account. Used by the operator UI and by the audit/diagnostic
	// surface so a single account's override picture across tenants is
	// inspectable.
	ListByAccount(ctx context.Context, accountID int64) ([]*MFAOverride, error)

	// DeleteAllByAccount wipes every override row when the account is
	// reset or destroyed. Returns the number of rows removed for the
	// audit log.
	DeleteAllByAccount(ctx context.Context, accountID int64) (int64, error)
}

// TenantAccountInfo holds flattened account data for a given tenant, used by operator dashboard.
type TenantAccountInfo struct {
	AccountID           int64  `bun:"account_id" json:"account_id"`
	Email               string `bun:"email" json:"email"`
	Active              bool   `bun:"active" json:"active"`
	FirstName           string `bun:"first_name" json:"first_name"`
	LastName            string `bun:"last_name" json:"last_name"`
	RoleName            string `bun:"role_name" json:"role_name"`
	PedagogicRole       string `bun:"pedagogic_role" json:"pedagogic_role"`
	Status              string `bun:"status" json:"status"`
	HasAdminRole        bool   `bun:"has_admin_role" json:"has_admin_role"`
	HasUserRole         bool   `bun:"has_user_role" json:"has_user_role"`
	HasCaregiverProfile bool   `bun:"has_caregiver_profile" json:"has_caregiver_profile"`
	IsActiveCaregiver   bool   `bun:"is_active_caregiver" json:"is_active_caregiver"`
}

// OrgAccountInfo extends TenantAccountInfo with school context for org-level listings.
type OrgAccountInfo struct {
	TenantAccountInfo
	SchoolID   int64  `bun:"school_id" json:"school_id"`
	SchoolName string `bun:"school_name" json:"school_name"`
}

// AccountTenantAccessInfo describes one school an account has (or had) access
// to. Unlike the account listings above it is keyed by account, not by tenant,
// and therefore spans every school in the platform — it backs the operator-only
// "Schulzugänge" surface and must never be exposed to a tenant-scoped caller.
//
// Roles are NOT part of this row: they live in auth.account_roles and are
// merged in by the service from AccountRoleRepository.FindByAccountID so the
// query stays a plain per-mapping lookup.
type AccountTenantAccessInfo struct {
	TenantID         int64      `bun:"tenant_id" json:"tenant_id"`
	SchoolName       string     `bun:"school_name" json:"school_name"`
	SchoolSlug       string     `bun:"school_slug" json:"school_slug"`
	SchoolActive     bool       `bun:"school_active" json:"school_active"`
	OrganizationID   int64      `bun:"organization_id" json:"organization_id"`
	OrganizationName string     `bun:"organization_name" json:"organization_name"`
	Status           string     `bun:"status" json:"status"`
	ActivatedAt      *time.Time `bun:"activated_at" json:"activated_at,omitempty"`
	DeactivatedAt    *time.Time `bun:"deactivated_at" json:"deactivated_at,omitempty"`
	HasPerson        bool       `bun:"has_person" json:"has_person"`
	HasStaff         bool       `bun:"has_staff" json:"has_staff"`
}

// AccountTenantRepository defines operations for querying account-tenant mappings.
type AccountTenantRepository interface {
	Create(ctx context.Context, mapping *AccountTenant) error
	EnsureActive(ctx context.Context, mapping *AccountTenant) error
	Deactivate(ctx context.Context, accountID, tenantID int64) error
	FindActiveByAccountID(ctx context.Context, accountID int64) ([]AccountTenant, error)
	FindActiveGuardianByAccountID(ctx context.Context, accountID int64) ([]AccountTenant, error)
	ExistsByAccountAndTenant(ctx context.Context, accountID, tenantID int64) (bool, error)
	// ExistsActiveByAccountAndTenantForShare is ExistsByAccountAndTenant with a
	// FOR SHARE row lock. Transaction-only: it blocks a concurrent Deactivate
	// until the caller's transaction commits, which is what makes a
	// membership check and a token write in that transaction atomic.
	ExistsActiveByAccountAndTenantForShare(ctx context.Context, accountID, tenantID int64) (bool, error)
	ListAccountsByTenantID(ctx context.Context, tenantID int64) ([]TenantAccountInfo, error)
	ListAccountsByOrganizationID(ctx context.Context, organizationID int64) ([]OrgAccountInfo, error)
	ListAllAccounts(ctx context.Context) ([]OrgAccountInfo, error)
	ListTenantAccessByAccountID(ctx context.Context, accountID int64) ([]AccountTenantAccessInfo, error)
}

// GuardianInvitationRepository defines operations for managing guardian invitations.
type GuardianInvitationRepository interface {
	// Create inserts a new guardian invitation
	Create(ctx context.Context, invitation *GuardianInvitation) error

	// Update updates an existing guardian invitation
	Update(ctx context.Context, invitation *GuardianInvitation) error

	// FindByID retrieves a guardian invitation by ID
	FindByID(ctx context.Context, id int64) (*GuardianInvitation, error)

	// FindByToken retrieves a guardian invitation by token
	FindByToken(ctx context.Context, token string) (*GuardianInvitation, error)

	// FindByGuardianProfileID retrieves invitations for a guardian profile
	FindByGuardianProfileID(ctx context.Context, guardianProfileID int64) ([]*GuardianInvitation, error)

	// FindPending retrieves all pending (not accepted, not expired) invitations
	FindPending(ctx context.Context) ([]*GuardianInvitation, error)

	// FindPendingApproval retrieves parent-initiated invitations awaiting
	// staff approval (approval_status = 'pending'), newest first.
	FindPendingApproval(ctx context.Context) ([]*GuardianInvitation, error)

	// MarkAsAccepted marks an invitation as accepted
	MarkAsAccepted(ctx context.Context, id int64) error

	// UpdateEmailStatus updates the email delivery status
	UpdateEmailStatus(ctx context.Context, id int64, sentAt *time.Time, emailError *string, retryCount int) error

	// DeleteExpired deletes expired invitations
	DeleteExpired(ctx context.Context) (int, error)
}
