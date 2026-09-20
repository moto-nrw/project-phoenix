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

// AccountTenantRepository defines operations for querying account-tenant mappings.
type AccountTenantRepository interface {
	Create(ctx context.Context, mapping *AccountTenant) error
	EnsureActive(ctx context.Context, mapping *AccountTenant) error
	ExistsByAccountAndTenant(ctx context.Context, accountID, tenantID int64) (bool, error)
}
