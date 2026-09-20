package authmodels

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// AccountRepository defines operations for managing accounts
type AccountRepository interface {
	base.CRUDRepository[*Account]
	FindManageableByID(ctx context.Context, id int64) (*Account, error)
	FindByIDForUpdate(ctx context.Context, id int64) (*Account, error)
	FindByEmail(ctx context.Context, email string) (*Account, error)
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
	// ListEffectiveAdminAccountIDs returns the IDs of active accounts with
	// effective admin scope in the current tenant: the literal admin role, or
	// an admin:* / *:* permission from a role or granted directly.
	ListEffectiveAdminAccountIDs(ctx context.Context) ([]int64, error)
	// AnonymizeForDeletion overwrites the email with an anonymized
	// placeholder and clears the username (GDPR person deletion).
	AnonymizeForDeletion(ctx context.Context, accountID int64, anonymizedEmail string) error
}

// InvitationTokenRepository defines operations for managing invitation tokens.
type InvitationTokenRepository interface {
	Create(ctx context.Context, token *InvitationToken) error
	Update(ctx context.Context, token *InvitationToken) error
	FindByID(ctx context.Context, id interface{}) (*InvitationToken, error)
	FindByEmail(ctx context.Context, email string) ([]*InvitationToken, error)
	DeleteExpired(ctx context.Context, now time.Time) (int, error)
	List(ctx context.Context, filters map[string]interface{}) ([]*InvitationToken, error)
}

// AccountTenantAccessInfo describes one school an account has (or had) access
// to. Unlike the account listings above it is keyed by account, not by tenant,
// and therefore spans every school in the platform — it backs the operator-only
// "Schulzugänge" surface and must never be exposed to a tenant-scoped caller.
//
// Roles are NOT part of this row: they live in auth.account_roles and are
// resolved separately by the identity capability; this row is a plain
// per-mapping lookup.
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
	FindActiveByAccountID(ctx context.Context, accountID int64) ([]AccountTenant, error)
	ExistsByAccountAndTenant(ctx context.Context, accountID, tenantID int64) (bool, error)
	// ExistsActiveByAccountAndTenantForShare is ExistsByAccountAndTenant with a
	// FOR SHARE row lock. Transaction-only: it blocks a concurrent membership
	// revocation until the caller's transaction commits, which is what makes a
	// membership check and a token write in that transaction atomic.
	ExistsActiveByAccountAndTenantForShare(ctx context.Context, accountID, tenantID int64) (bool, error)
}

type StaffCalendarFeedOwner struct {
	AccountID int64 `bun:"account_id"`
	TenantID  int64 `bun:"tenant_id"`
}

type StaffCalendarFeedTokenRepository interface {
	// FindOwnerByTokenHash resolves a cross-tenant capability token; generic
	// tenant-scoped filters cannot perform this lookup before the tenant is known.
	FindOwnerByTokenHash(ctx context.Context, tokenHash string) (*StaffCalendarFeedOwner, error)
	// EnsureToken atomically creates the token once and returns the winning value
	// when concurrent requests race; a generic update cannot express that contract.
	EnsureToken(ctx context.Context, accountID, tenantID int64, tokenHash string) (string, error)
	// RotateToken replaces the active mapping's capability token atomically; this
	// domain operation is intentionally narrower than a generic per-field update.
	RotateToken(ctx context.Context, accountID, tenantID int64, tokenHash string) (bool, error)
}
