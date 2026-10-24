package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/delivery/subscriptions"
)

// PushSubscriptionRepository defines operations for Web Push subscriptions.
// All reads/writes are tenant-scoped (RLS); callers outside tenant middleware
// must wrap calls in tenant.WithTenantTx.
type PushSubscriptionRepository interface {

	// Upsert inserts or refreshes a subscription keyed by (tenant_id, portal,
	// endpoint).
	RegisterSubscription(ctx context.Context, sub *subscriptions.PushSubscription) error
	// DeleteByEndpoint removes the caller's staff-portal subscription for the
	// current tenant.
	DeleteByEndpoint(ctx context.Context, accountID int64, endpoint string) error
	// DeleteParentByAccountEndpoint removes the caller's parent-portal
	// subscription for the current tenant.
	DeleteParentByAccountEndpoint(ctx context.Context, accountID int64, endpoint string) error
	// DeleteSchoolByEndpoint removes only a school-portal subscription for the
	// caller in the current tenant.
	DeleteSchoolByEndpoint(ctx context.Context, accountID int64, endpoint string) error
	// DeleteSchoolByEndpointAcrossTenants removes every school-portal binding
	// for an endpoint across tenants (#2208). Callers must use an admin
	// transaction; a browser holds at most one school registration.
	DeleteSchoolByEndpointAcrossTenants(ctx context.Context, endpoint string) error
	// DeleteExpiredIfUnchanged removes a subscription only if its persisted
	// ownership, key material, and update timestamp still match the sent snapshot.
	DeleteExpiredIfUnchanged(ctx context.Context, sub *subscriptions.PushSubscription) (bool, error)
	// DeleteParentByEndpoint removes every parent-portal binding for an endpoint
	// across tenants. Callers must use an admin transaction.
	DeleteParentByEndpoint(ctx context.Context, endpoint string) error
	// DeleteStaffByAccountID removes every staff-portal subscription for an
	// account across tenants. Callers must use an admin transaction and
	// reserve this for account-wide session revocation, not a single-device
	// logout.
	DeleteStaffByAccountID(ctx context.Context, accountID int64) error
	// DeleteParentByAccountID removes every parent-portal subscription for an
	// account across tenants. Callers must use an admin transaction and
	// reserve this for account-wide session revocation, not a single-device
	// logout.
	DeleteParentByAccountID(ctx context.Context, accountID int64) error
	// DeleteSchoolByAccountID removes every school-portal subscription for an
	// account across tenants (#2208). Admin transaction required.
	DeleteSchoolByAccountID(ctx context.Context, accountID int64) error
	// DeleteByTokenFamilyID removes subscriptions registered by one
	// refresh-token family, any portal, across tenants. Callers must use an
	// admin transaction. An empty family ID is a no-op so pre-binding rows
	// are not swept by an unbound logout.
	DeleteByTokenFamilyID(ctx context.Context, accountID int64, familyID string) error
	// DeleteStaffUnboundByAccount removes staff-portal subscriptions that were
	// never bound to a token family. A positive tenantID selects the school. Callers must use an admin transaction.
	DeleteStaffUnboundByAccount(ctx context.Context, accountID, tenantID int64) error
	// DeleteParentUnboundByAccount removes parent-portal subscriptions that
	// were never bound to a token family. A positive tenantID selects the school. Callers must use an admin transaction.
	DeleteParentUnboundByAccount(ctx context.Context, accountID, tenantID int64) error
	// DeleteSchoolUnboundByAccount is DeleteStaffUnboundByAccount for
	// school-portal subscriptions (#2208).
	DeleteSchoolUnboundByAccount(ctx context.Context, accountID, tenantID int64) error
	// DeleteUnboundByAccountForPortal is the unbound delete of the given
	// portal; a portal other than parent or school is the staff portal.
	DeleteUnboundByAccountForPortal(ctx context.Context, accountID, tenantID int64, portal string) error
	// DeleteOrphanedSubscriptions removes push rows whose token family is gone
	// or whose account has no live session left for that portal. Unbound
	// parent rows stay if any parent session exists on the account.
	DeleteOrphanedSubscriptions(ctx context.Context) error
	// FindForTenantStaff returns all staff-portal subscriptions of the current tenant.
	FindForTenantStaff(ctx context.Context) ([]*subscriptions.PushSubscription, error)

	// FindForStaffAccounts returns the staff-portal subscriptions of the given
	// accounts in the current tenant. Same eligibility rules as
	// FindForTenantStaff, narrowed to named recipients — the addressing
	// primitive for personal notifications.
	FindForStaffAccounts(ctx context.Context, accountIDs []int64) ([]*subscriptions.PushSubscription, error)
	// FindForSchoolAccounts returns school-portal subscriptions for named
	// accounts in the current tenant.
	FindForSchoolAccounts(ctx context.Context, accountIDs []int64) ([]*subscriptions.PushSubscription, error)
	// FindForTenantAdmins returns staff-portal subscriptions of accounts with
	// effective admin scope in the current tenant.
	FindForTenantAdmins(ctx context.Context) ([]*subscriptions.PushSubscription, error)
	// FindForGuardians returns parent-portal subscriptions of guardian accounts
	// with active guardian access in the current tenant. Pending-enrollment-only
	// recipients are deliberately excluded from Web Push.
	//
	// A non-empty studentIDs additionally requires parent_portal.access for at
	// least one of those children on the caller's student-guardian relationship,
	// so a notification about a child reaches only the accounts that may still
	// hear about it at delivery time. Pass nil for events about no child.
	FindForGuardians(ctx context.Context, guardianAccountIDs []int64, studentIDs []int64) ([]*subscriptions.PushSubscription, error)
}
