package delivery

import (
	"context"

	"database/sql"
	"errors"
	"fmt"
	"time"

	deliveryModels "github.com/moto-nrw/project-phoenix/models/delivery"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

const (
	tablePushSubscriptions = "iot.push_subscriptions"
	viewPushSubscriptions  = "platform.delivery_push_subscriptions"
	pushPortalFilter       = "portal = ?"
)

// PushSubscriptionRepository implements deliveryModels.PushSubscriptionRepository.
type PushSubscriptionRepository struct {
	DB *bun.DB
}

// NewPushSubscriptionRepository creates a new PushSubscriptionRepository.
func NewPushSubscriptionRepository(db *bun.DB) deliveryModels.PushSubscriptionRepository {
	return &PushSubscriptionRepository{DB: db}
}

// Upsert inserts or refreshes a subscription keyed by (tenant_id, portal,
// endpoint), so a browser can be registered independently in both staff
// portals.
// A re-subscribe from the same browser rotates keys and may switch accounts
// (different user logs in on the same device) — both are overwritten.
func (r *PushSubscriptionRepository) RegisterSubscription(ctx context.Context, sub *deliveryModels.PushSubscription) error {
	if sub.TenantID == 0 {
		sub.TenantID = tenant.FromContext(ctx)
	}
	_, err := r.database(ctx).NewInsert().
		Model((*pushSubscription)(sub)).
		ModelTableExpr(tablePushSubscriptions).
		On("CONFLICT (tenant_id, portal, endpoint) DO UPDATE").
		Set("account_id = EXCLUDED.account_id").
		Set("p256dh = EXCLUDED.p256dh").
		Set("auth = EXCLUDED.auth").
		Set("user_agent = EXCLUDED.user_agent").
		Set("token_family_id = EXCLUDED.token_family_id").
		Set("updated_at = NOW()").
		Exec(ctx)
	if err != nil {
		return &databaseError{Op: "upsert push subscription", Err: translateNotFound(err)}
	}
	return nil
}

// DeleteByEndpoint removes the caller's staff-portal subscription for the
// current tenant. Scoped to the account so one user cannot unsubscribe
// another's device.
func (r *PushSubscriptionRepository) DeleteByEndpoint(ctx context.Context, accountID int64, endpoint string) error {
	return r.deleteByEndpointPortal(ctx, accountID, endpoint, deliveryModels.PushPortalStaff, "delete staff push subscription")
}

// DeleteParentByAccountEndpoint removes the caller's parent-portal
// subscription for the current tenant without affecting another portal that
// uses the same browser endpoint.
func (r *PushSubscriptionRepository) DeleteParentByAccountEndpoint(ctx context.Context, accountID int64, endpoint string) error {
	return r.deleteByEndpointPortal(ctx, accountID, endpoint, deliveryModels.PushPortalParent, "delete parent push subscription")
}

func (r *PushSubscriptionRepository) deleteByEndpointPortal(ctx context.Context, accountID int64, endpoint, portal, op string) error {
	query := r.database(ctx).NewDelete().
		Model((*pushSubscription)(nil)).
		ModelTableExpr(tablePushSubscriptions).
		Where("account_id = ?", accountID).
		Where("endpoint = ?", endpoint).
		Where("portal = ?", portal)
	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}
	if _, err := query.Exec(ctx); err != nil {
		return &databaseError{Op: op, Err: translateNotFound(err)}
	}
	return nil
}

// DeleteSchoolByEndpoint removes the caller's school-portal subscription for
// the current tenant without affecting a tenant-portal registration that uses
// the same browser endpoint.
func (r *PushSubscriptionRepository) DeleteSchoolByEndpoint(ctx context.Context, accountID int64, endpoint string) error {
	return r.deleteByEndpointPortal(ctx, accountID, endpoint, deliveryModels.PushPortalSchool, "delete school push subscription")
}

// DeleteSchoolByEndpointAcrossTenants serializes rebinds, then removes every
// school-portal binding for an endpoint across tenants and accounts. A school
// session is pinned to exactly one school, so a browser holds at most one
// school registration; without this, the row of the school a person left keeps
// receiving her notifications. The caller must supply an admin transaction.
func (r *PushSubscriptionRepository) DeleteSchoolByEndpointAcrossTenants(ctx context.Context, endpoint string) error {
	db := r.database(ctx)
	if _, err := db.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", endpoint); err != nil {
		return &databaseError{Op: "lock school push subscription endpoint", Err: translateNotFound(err)}
	}
	_, err := db.NewDelete().
		Model((*pushSubscription)(nil)).
		ModelTableExpr(tablePushSubscriptions).
		Where("endpoint = ?", endpoint).
		Where(pushPortalFilter, deliveryModels.PushPortalSchool).
		Exec(ctx)
	if err != nil {
		return &databaseError{Op: "delete school push subscriptions by endpoint", Err: translateNotFound(err)}
	}
	return nil
}

// DeleteExpiredIfUnchanged removes an expired subscription only while it still
// matches the snapshot sent to the push service. A concurrent refresh or
// account rebind changes at least one predicate and preserves the current row.
func (r *PushSubscriptionRepository) DeleteExpiredIfUnchanged(ctx context.Context, sub *deliveryModels.PushSubscription) (bool, error) {
	query := r.database(ctx).NewDelete().
		Model((*pushSubscription)(nil)).
		ModelTableExpr(tablePushSubscriptions).
		Where("id = ?", sub.ID).
		Where("account_id = ?", sub.AccountID).
		Where("portal = ?", sub.Portal).
		Where("endpoint = ?", sub.Endpoint).
		Where("p256dh = ?", sub.P256dh).
		Where("auth = ?", sub.Auth).
		Where("updated_at = ?", sub.UpdatedAt)
	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}
	result, err := query.Exec(ctx)
	if err != nil {
		return false, &databaseError{Op: "delete unchanged expired push subscription", Err: translateNotFound(err)}
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, &databaseError{Op: "count deleted expired push subscriptions", Err: translateNotFound(err)}
	}
	return rows > 0, nil
}

// DeleteParentByEndpoint serializes rebinds, then removes every parent-portal
// binding for an endpoint across tenants. The caller must supply an admin
// transaction because a parent device can be linked to several schools.
func (r *PushSubscriptionRepository) DeleteParentByEndpoint(ctx context.Context, endpoint string) error {
	db := r.database(ctx)
	if _, err := db.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", endpoint); err != nil {
		return &databaseError{Op: "lock parent push subscription endpoint", Err: translateNotFound(err)}
	}
	_, err := db.NewDelete().
		Model((*pushSubscription)(nil)).
		ModelTableExpr(tablePushSubscriptions).
		Where("endpoint = ?", endpoint).
		Where(pushPortalFilter, deliveryModels.PushPortalParent).
		Exec(ctx)
	if err != nil {
		return &databaseError{Op: "delete parent push subscriptions by endpoint", Err: translateNotFound(err)}
	}
	return nil
}

// DeleteStaffByAccountID removes every staff-portal subscription for an
// account across tenants. The caller must supply an admin transaction because
// this intentionally crosses tenant boundaries during account-wide session
// revocation.
func (r *PushSubscriptionRepository) DeleteStaffByAccountID(ctx context.Context, accountID int64) error {
	return r.deleteByAccountPortal(ctx, accountID, deliveryModels.PushPortalStaff, "delete staff push subscriptions")
}

// DeleteSchoolByAccountID removes every school-portal subscription for an
// account across tenants (#2208). Same admin-transaction contract as the
// staff variant.
func (r *PushSubscriptionRepository) DeleteSchoolByAccountID(ctx context.Context, accountID int64) error {
	return r.deleteByAccountPortal(ctx, accountID, deliveryModels.PushPortalSchool, "delete school push subscriptions")
}

// DeleteParentByAccountID removes every parent-portal subscription for an
// account across tenants. The caller must supply an admin transaction.
func (r *PushSubscriptionRepository) DeleteParentByAccountID(ctx context.Context, accountID int64) error {
	return r.deleteByAccountPortal(ctx, accountID, deliveryModels.PushPortalParent, "delete parent push subscriptions")
}

func (r *PushSubscriptionRepository) deleteByAccountPortal(ctx context.Context, accountID int64, portal, op string) error {
	_, err := r.database(ctx).NewDelete().
		TableExpr(tablePushSubscriptions).
		Where("account_id = ?", accountID).
		Where(pushPortalFilter, portal).
		Exec(ctx)
	if err != nil {
		return &databaseError{Op: op, Err: translateNotFound(err)}
	}
	return nil
}

// DeleteByTokenFamilyID removes subscriptions registered by one refresh-token
// family, any portal, across tenants. The caller must supply an admin
// transaction. An empty family ID is a no-op so unbound legacy rows survive.
func (r *PushSubscriptionRepository) DeleteByTokenFamilyID(ctx context.Context, accountID int64, familyID string) error {
	if familyID == "" {
		return nil
	}
	_, err := r.database(ctx).NewDelete().
		TableExpr(tablePushSubscriptions).
		Where("account_id = ?", accountID).
		Where("token_family_id = ?", familyID).
		Exec(ctx)
	if err != nil {
		return &databaseError{Op: "delete push subscriptions by token family", Err: translateNotFound(err)}
	}
	return nil
}

// DeleteStaffUnboundByAccount removes staff-portal subscriptions that have no
// token family. The caller must supply an admin transaction. A positive
// tenantID limits the delete to that school.
func (r *PushSubscriptionRepository) DeleteStaffUnboundByAccount(ctx context.Context, accountID, tenantID int64) error {
	return r.deleteUnboundByAccount(ctx, accountID, tenantID, deliveryModels.PushPortalStaff, "delete unbound staff push subscriptions")
}

// DeleteSchoolUnboundByAccount removes school-portal subscriptions that have
// no token family (#2208). Admin transaction required.
func (r *PushSubscriptionRepository) DeleteSchoolUnboundByAccount(ctx context.Context, accountID, tenantID int64) error {
	return r.deleteUnboundByAccount(ctx, accountID, tenantID, deliveryModels.PushPortalSchool, "delete unbound school push subscriptions")
}

// DeleteParentUnboundByAccount removes parent-portal subscriptions that have
// no token family. The caller must supply an admin transaction. A positive
// tenantID limits the delete to that school.
func (r *PushSubscriptionRepository) DeleteParentUnboundByAccount(ctx context.Context, accountID, tenantID int64) error {
	return r.deleteUnboundByAccount(ctx, accountID, tenantID, deliveryModels.PushPortalParent, "delete unbound parent push subscriptions")
}

// DeleteUnboundByAccountForPortal removes the account's subscriptions
// without a token family on the given portal. A portal other than parent or
// school is the staff portal.
func (r *PushSubscriptionRepository) DeleteUnboundByAccountForPortal(ctx context.Context, accountID, tenantID int64, portal string) error {
	switch portal {
	case deliveryModels.PushPortalParent:
		return r.DeleteParentUnboundByAccount(ctx, accountID, tenantID)
	case deliveryModels.PushPortalSchool:
		return r.DeleteSchoolUnboundByAccount(ctx, accountID, tenantID)
	default:
		return r.DeleteStaffUnboundByAccount(ctx, accountID, tenantID)
	}
}

func (r *PushSubscriptionRepository) DeleteOrphanedSubscriptions(ctx context.Context) error {
	_, err := r.database(ctx).NewDelete().
		TableExpr(tablePushSubscriptions + ` AS "push_subscription"`).
		Where(`"push_subscription".id IN (
			SELECT "delivery_subscription".id
			FROM platform.delivery_push_subscriptions AS "delivery_subscription"
			WHERE NOT "delivery_subscription".has_live_token
		)`).
		Exec(ctx)
	if err != nil {
		return &databaseError{Op: "delete orphaned push subscriptions", Err: translateNotFound(err)}
	}
	return nil
}

func (r *PushSubscriptionRepository) deleteUnboundByAccount(ctx context.Context, accountID, tenantID int64, portal, op string) error {
	query := r.database(ctx).NewDelete().
		TableExpr(tablePushSubscriptions).
		Where("account_id = ?", accountID).
		Where("token_family_id = ?", "").
		Where(pushPortalFilter, portal)
	if tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}
	if _, err := query.Exec(ctx); err != nil {
		return &databaseError{Op: op, Err: translateNotFound(err)}
	}
	return nil
}

// staffSubscriptionQuery builds the base query every staff-portal finder shares:
// staff portal, active account, active tenant mapping, non-guardian role, and
// the tenant predicate.
//
// Extracted so the eligibility rules cannot drift between finders. They are a
// delivery-time authorization check, not a convenience filter: a subscription
// must stop receiving pushes the moment the account is deactivated or its
// mapping to this school ends, and that has to hold for every finder equally.
func (r *PushSubscriptionRepository) staffSubscriptionQuery(ctx context.Context, subs *[]*pushSubscription, portals ...string) *bun.SelectQuery {
	if len(portals) == 0 {
		portals = []string{deliveryModels.PushPortalStaff}
	}
	query := r.database(ctx).NewSelect().
		Model(subs).
		ModelTableExpr(viewPushSubscriptions+` AS "push_subscription"`).
		Where(`"push_subscription".portal IN (?)`, bun.List(portals)).
		Where(`"push_subscription".account_active`).
		Where(`"push_subscription".tenant_active`).
		Where(`"push_subscription".has_staff_role`)

	return withTenantFilter(ctx, query, "push_subscription")
}

// FindForTenantStaff returns all staff-portal subscriptions of the current tenant.
func (r *PushSubscriptionRepository) FindForTenantStaff(ctx context.Context) ([]*deliveryModels.PushSubscription, error) {
	var subs []*pushSubscription
	if err := r.staffSubscriptionQuery(ctx, &subs).Scan(ctx); err != nil {
		return nil, &databaseError{Op: "find staff push subscriptions", Err: translateNotFound(err)}
	}
	return subscriptionValues(subs), nil
}

// FindForStaffAccounts returns staff-portal subscriptions of the given
// accounts in the current tenant.
//
// The eligibility rules are re-checked here, at delivery time, rather than
// inherited from whoever assembled the recipient list: that list is built in an
// earlier transaction, and an account can be deactivated or unmapped from the
// school in between.
func (r *PushSubscriptionRepository) FindForStaffAccounts(ctx context.Context, accountIDs []int64) ([]*deliveryModels.PushSubscription, error) {
	if len(accountIDs) == 0 {
		return nil, nil
	}
	var subs []*pushSubscription
	query := r.staffSubscriptionQuery(ctx, &subs, deliveryModels.PushPortalStaff).
		Where(`"push_subscription".account_id IN (?)`, bun.List(accountIDs))
	if err := query.Scan(ctx); err != nil {
		return nil, &databaseError{Op: "find staff account push subscriptions", Err: translateNotFound(err)}
	}
	return subscriptionValues(subs), nil
}

// FindForSchoolAccounts returns school-portal subscriptions of the named
// accounts in the current tenant. Notification delivery invokes it only for
// types explicitly offered in moto schule.
//
// Beyond the shared staff eligibility rules it requires the lehrkraft system
// role at this school, so revoking that role stops school-portal pushes even
// when the account keeps another staff role.
func (r *PushSubscriptionRepository) FindForSchoolAccounts(ctx context.Context, accountIDs []int64) ([]*deliveryModels.PushSubscription, error) {
	if len(accountIDs) == 0 {
		return nil, nil
	}
	var subs []*pushSubscription
	query := r.staffSubscriptionQuery(ctx, &subs, deliveryModels.PushPortalSchool).
		Where(`"push_subscription".account_id IN (?)`, bun.List(accountIDs)).
		Where(`"push_subscription".has_school_role`)
	if err := query.Scan(ctx); err != nil {
		return nil, &databaseError{Op: "find school push subscriptions", Err: translateNotFound(err)}
	}
	return subscriptionValues(subs), nil
}

// FindForTenantAdmins returns staff-portal subscriptions of effective admins:
// accounts with the literal admin role or an admin:* / *:* permission granted
// directly or through a tenant role. This mirrors the SSE authorization scope.
func (r *PushSubscriptionRepository) FindForTenantAdmins(ctx context.Context) ([]*deliveryModels.PushSubscription, error) {
	var subs []*pushSubscription
	query := r.database(ctx).NewSelect().
		Model(&subs).
		ModelTableExpr(viewPushSubscriptions+` AS "push_subscription"`).
		Where(pushPortalFilter, deliveryModels.PushPortalStaff).
		Where(`"push_subscription".account_active`).
		Where(`"push_subscription".tenant_active`).
		Where(`"push_subscription".has_staff_role`).
		Where(`"push_subscription".effective_admin`)
	query = withTenantFilter(ctx, query, "push_subscription")
	if err := query.Scan(ctx); err != nil {
		return nil, &databaseError{Op: "find admin push subscriptions", Err: translateNotFound(err)}
	}
	return subscriptionValues(subs), nil
}

// FindForGuardians returns parent-portal subscriptions of guardian accounts
// with an active mapping and guardian role in the current tenant. This keeps
// pending-enrollment-only recipients out of Web Push until access is active.
//
// A non-empty studentIDs narrows the result to accounts that still hold
// parent_portal.access for at least one of those children. The producer decided
// its audience in an earlier transaction; this one sends. Asking again here is
// what keeps a notification about a child from reaching an account whose access
// was revoked in between — the same containment predicate the parent read paths
// and the messaging fan-out use, so the three cannot drift apart.
func (r *PushSubscriptionRepository) FindForGuardians(ctx context.Context, guardianAccountIDs []int64, studentIDs []int64) ([]*deliveryModels.PushSubscription, error) {
	if len(guardianAccountIDs) == 0 {
		return nil, nil
	}
	var subs []*pushSubscription
	query := r.database(ctx).NewSelect().
		Model(&subs).
		ModelTableExpr(viewPushSubscriptions+` AS "push_subscription"`).
		Where(pushPortalFilter, deliveryModels.PushPortalParent).
		Where(`"push_subscription".account_active`).
		Where(`"push_subscription".tenant_active`).
		Where(`"push_subscription".has_guardian_role`).
		Where(`"push_subscription".account_id IN (?)`, bun.List(guardianAccountIDs))
	if len(studentIDs) > 0 {
		query = query.Where(`"push_subscription".guardian_student_ids && ?::BIGINT[]`, pgdialect.Array(studentIDs))
	}
	query = withTenantFilter(ctx, query, "push_subscription")
	if err := query.Scan(ctx); err != nil {
		return nil, &databaseError{Op: "find guardian push subscriptions", Err: translateNotFound(err)}
	}
	return subscriptionValues(subs), nil
}

type pushSubscription struct {
	ID            int64     `bun:"id,pk,autoincrement"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	AccountID     int64     `bun:"account_id,notnull"`
	Portal        string    `bun:"portal,notnull"`
	Endpoint      string    `bun:"endpoint,notnull"`
	P256dh        string    `bun:"p256dh,notnull"`
	Auth          string    `bun:"auth,notnull"`
	UserAgent     string    `bun:"user_agent,notnull,default:''"`
	TokenFamilyID string    `bun:"token_family_id,notnull,default:''"`
}

func subscriptionValues(rows []*pushSubscription) []*deliveryModels.PushSubscription {
	if rows == nil {
		return nil
	}
	values := make([]*deliveryModels.PushSubscription, len(rows))
	for i, row := range rows {
		values[i] = (*deliveryModels.PushSubscription)(row)
	}
	return values
}
func (r *PushSubscriptionRepository) database(ctx context.Context) bun.IDB {
	raw, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return r.DB
	}
	switch tx := raw.(type) {
	case bun.Tx:
		return tx
	case *bun.Tx:
		if tx != nil {
			return tx
		}
	}
	panic(fmt.Sprintf("base repository: unsupported transaction type %T", raw))
}
func withTenantFilter(ctx context.Context, query *bun.SelectQuery, alias string) *bun.SelectQuery {
	if id := tenant.FromContext(ctx); id > 0 {
		return query.Where("?.tenant_id = ?", bun.Ident(alias), id)
	}
	return query
}
func translateNotFound(err error) error {
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return errors.Join(subscriptionNotFound{}, err)
}

type subscriptionNotFound struct{}

func (subscriptionNotFound) Error() string       { return "repository: not found" }
func (subscriptionNotFound) RepositoryNotFound() {}
