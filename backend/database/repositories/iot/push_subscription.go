package iot

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const (
	tablePushSubscriptions = "iot.push_subscriptions"
	pushPortalFilter       = "portal = ?"
	activeAccountJoin      = `INNER JOIN auth.accounts AS "account"
		ON "account".id = "push_subscription".account_id`
	activeAccountTenantJoin = `INNER JOIN auth.account_tenants AS "account_tenant"
		ON "account_tenant".account_id = "push_subscription".account_id
		AND "account_tenant".tenant_id = "push_subscription".tenant_id`
	nonGuardianRoleFilter = `EXISTS (
		SELECT 1
		FROM auth.account_roles AS "staff_account_role"
		INNER JOIN auth.roles AS "staff_role" ON "staff_role".id = "staff_account_role".role_id
		WHERE "staff_account_role".account_id = "push_subscription".account_id
			AND "staff_account_role".tenant_id = "push_subscription".tenant_id
			AND LOWER("staff_role".name) <> ?
	)`
	guardianRoleFilter = `EXISTS (
		SELECT 1
		FROM auth.account_roles AS "guardian_account_role"
		INNER JOIN auth.roles AS "guardian_role" ON "guardian_role".id = "guardian_account_role".role_id
		WHERE "guardian_account_role".account_id = "push_subscription".account_id
			AND "guardian_account_role".tenant_id = "push_subscription".tenant_id
			AND LOWER("guardian_role".name) = ?
	)`
	// childAccessFilter keeps a notification about one or more children to the
	// accounts that still hold parent_portal.access for at least one of them. The
	// relationship row, not the account, carries that permission: the same person
	// can be a full guardian for one child and hidden from another.
	childAccessFilter = `EXISTS (
		SELECT 1
		FROM users.students_guardians AS "child_link"
		INNER JOIN users.guardian_profiles AS "child_guardian"
			ON "child_guardian".id = "child_link".guardian_profile_id
		WHERE "child_link".student_id IN (?)
			AND "child_link".tenant_id = "push_subscription".tenant_id
			AND "child_guardian".account_id = "push_subscription".account_id
			AND "child_link".permissions @> ?::jsonb
	)`
	// guardianPortalAccessPermission is the containment probe for
	// childAccessFilter, spelled exactly as the parent read paths spell it.
	guardianPortalAccessPermission = `{"parent_portal.access": true}`
)

// PushSubscriptionRepository implements iot.PushSubscriptionRepository.
type PushSubscriptionRepository struct {
	*base.Repository[*iot.PushSubscription]
}

// NewPushSubscriptionRepository creates a new PushSubscriptionRepository.
func NewPushSubscriptionRepository(db *bun.DB) iot.PushSubscriptionRepository {
	repo := base.NewRepository[*iot.PushSubscription](db, tablePushSubscriptions, "PushSubscription")
	repo.TenantScoped = true
	return &PushSubscriptionRepository{Repository: repo}
}

// Upsert inserts or refreshes a subscription keyed by (tenant_id, endpoint).
// A re-subscribe from the same browser rotates keys and may switch accounts
// (different user logs in on the same device) — both are overwritten.
func (r *PushSubscriptionRepository) Upsert(ctx context.Context, sub *iot.PushSubscription) error {
	base.EnsureTenantID(ctx, sub)
	_, err := base.GetDB(ctx, r.DB).NewInsert().
		Model(sub).
		ModelTableExpr(tablePushSubscriptions).
		On("CONFLICT (tenant_id, endpoint) DO UPDATE").
		Set("account_id = EXCLUDED.account_id").
		Set("portal = EXCLUDED.portal").
		Set("p256dh = EXCLUDED.p256dh").
		Set("auth = EXCLUDED.auth").
		Set("user_agent = EXCLUDED.user_agent").
		Set("token_family_id = EXCLUDED.token_family_id").
		Set("updated_at = NOW()").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "upsert push subscription", Err: err}
	}
	return nil
}

// DeleteByEndpoint removes the caller's subscription for the current tenant.
// Scoped to the account so one user cannot unsubscribe another's device.
func (r *PushSubscriptionRepository) DeleteByEndpoint(ctx context.Context, accountID int64, endpoint string) error {
	query := base.GetDB(ctx, r.DB).NewDelete().
		Model((*iot.PushSubscription)(nil)).
		ModelTableExpr(tablePushSubscriptions).
		Where("account_id = ?", accountID).
		Where("endpoint = ?", endpoint)
	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}
	if _, err := query.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "delete push subscription", Err: err}
	}
	return nil
}

// DeleteExpiredIfUnchanged removes an expired subscription only while it still
// matches the snapshot sent to the push service. A concurrent refresh or
// account rebind changes at least one predicate and preserves the current row.
func (r *PushSubscriptionRepository) DeleteExpiredIfUnchanged(ctx context.Context, sub *iot.PushSubscription) (bool, error) {
	query := base.GetDB(ctx, r.DB).NewDelete().
		Model((*iot.PushSubscription)(nil)).
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
		return false, &modelBase.DatabaseError{Op: "delete unchanged expired push subscription", Err: err}
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, &modelBase.DatabaseError{Op: "count deleted expired push subscriptions", Err: err}
	}
	return rows > 0, nil
}

// DeleteParentByEndpoint serializes rebinds, then removes every parent-portal
// binding for an endpoint across tenants. The caller must supply an admin
// transaction because a parent device can be linked to several schools.
func (r *PushSubscriptionRepository) DeleteParentByEndpoint(ctx context.Context, endpoint string) error {
	db := base.GetDB(ctx, r.DB)
	if _, err := db.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", endpoint); err != nil {
		return &modelBase.DatabaseError{Op: "lock parent push subscription endpoint", Err: err}
	}
	_, err := db.NewDelete().
		Model((*iot.PushSubscription)(nil)).
		ModelTableExpr(tablePushSubscriptions).
		Where("endpoint = ?", endpoint).
		Where(pushPortalFilter, iot.PushPortalParent).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "delete parent push subscriptions by endpoint", Err: err}
	}
	return nil
}

// DeleteStaffByAccountID removes every staff-portal subscription for an
// account across tenants. The caller must supply an admin transaction because
// this intentionally crosses tenant boundaries during account-wide session
// revocation.
func (r *PushSubscriptionRepository) DeleteStaffByAccountID(ctx context.Context, accountID int64) error {
	_, err := base.GetDB(ctx, r.DB).NewDelete().
		TableExpr(tablePushSubscriptions).
		Where("account_id = ?", accountID).
		Where(pushPortalFilter, iot.PushPortalStaff).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "delete staff push subscriptions", Err: err}
	}
	return nil
}

// DeleteStaffByTokenFamilyID removes staff-portal subscriptions registered by
// one refresh-token family, across tenants. The caller must supply an admin
// transaction. An empty family ID is a no-op so unbound legacy rows survive.
func (r *PushSubscriptionRepository) DeleteStaffByTokenFamilyID(ctx context.Context, accountID int64, familyID string) error {
	if familyID == "" {
		return nil
	}
	_, err := base.GetDB(ctx, r.DB).NewDelete().
		TableExpr(tablePushSubscriptions).
		Where("account_id = ?", accountID).
		Where("token_family_id = ?", familyID).
		Where(pushPortalFilter, iot.PushPortalStaff).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "delete staff push subscriptions by token family", Err: err}
	}
	return nil
}

// DeleteStaffUnboundByAccount removes staff-portal subscriptions that have no
// token family. The caller must supply an admin transaction. A positive
// tenantID limits the delete to that school.
func (r *PushSubscriptionRepository) DeleteStaffUnboundByAccount(ctx context.Context, accountID, tenantID int64) error {
	query := base.GetDB(ctx, r.DB).NewDelete().
		TableExpr(tablePushSubscriptions).
		Where("account_id = ?", accountID).
		Where("token_family_id = ?", "").
		Where(pushPortalFilter, iot.PushPortalStaff)
	if tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}
	if _, err := query.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "delete unbound staff push subscriptions", Err: err}
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
func (r *PushSubscriptionRepository) staffSubscriptionQuery(ctx context.Context, subs *[]*iot.PushSubscription) *bun.SelectQuery {
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(subs).
		ModelTableExpr(tablePushSubscriptions+` AS "push_subscription"`).
		Join(activeAccountJoin).
		Join(activeAccountTenantJoin).
		Where(pushPortalFilter, iot.PushPortalStaff).
		Where(`"account".active = ?`, true).
		Where(`"account_tenant".status = ?`, authModels.AccountTenantStatusActive).
		Where(nonGuardianRoleFilter, authModels.BaseRoleGuardian)

	return base.WithTenantFilter(ctx, query, "push_subscription")
}

// FindForTenantStaff returns all staff-portal subscriptions of the current tenant.
func (r *PushSubscriptionRepository) FindForTenantStaff(ctx context.Context) ([]*iot.PushSubscription, error) {
	var subs []*iot.PushSubscription
	if err := r.staffSubscriptionQuery(ctx, &subs).Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find staff push subscriptions", Err: err}
	}
	return subs, nil
}

// FindForStaffAccounts returns the staff-portal subscriptions of the given
// accounts in the current tenant. It is the addressing primitive for personal
// notifications, where each recipient gets their own payload.
//
// The eligibility rules are re-checked here, at delivery time, rather than
// inherited from whoever assembled the recipient list: that list is built in an
// earlier transaction, and an account can be deactivated or unmapped from the
// school in between.
func (r *PushSubscriptionRepository) FindForStaffAccounts(ctx context.Context, accountIDs []int64) ([]*iot.PushSubscription, error) {
	if len(accountIDs) == 0 {
		return nil, nil
	}
	var subs []*iot.PushSubscription
	query := r.staffSubscriptionQuery(ctx, &subs).
		Where(`"push_subscription".account_id IN (?)`, bun.List(accountIDs))
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find staff account push subscriptions", Err: err}
	}
	return subs, nil
}

// FindForTenantAdmins returns staff-portal subscriptions of effective admins:
// accounts with the literal admin role or an admin:* / *:* permission granted
// directly or through a tenant role. This mirrors the SSE authorization scope.
func (r *PushSubscriptionRepository) FindForTenantAdmins(ctx context.Context) ([]*iot.PushSubscription, error) {
	var subs []*iot.PushSubscription
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&subs).
		ModelTableExpr(tablePushSubscriptions+` AS "push_subscription"`).
		Join(activeAccountJoin).
		Join(activeAccountTenantJoin).
		Where(pushPortalFilter, iot.PushPortalStaff).
		Where(`"account".active = ?`, true).
		Where(`"account_tenant".status = ?`, authModels.AccountTenantStatusActive).
		Where(nonGuardianRoleFilter, authModels.BaseRoleGuardian).
		Where(`EXISTS (
			SELECT 1
			FROM auth.account_roles AS "ar"
			INNER JOIN auth.roles AS "r" ON "r".id = "ar".role_id
			LEFT JOIN auth.role_permissions AS "rp" ON "rp".role_id = "ar".role_id
			LEFT JOIN auth.permissions AS "p" ON "p".id = "rp".permission_id
			WHERE "ar".account_id = "push_subscription".account_id
			  AND "ar".tenant_id = "push_subscription".tenant_id
			  AND (
			    LOWER("r".name) = 'admin'
			    OR ("p".resource = 'admin' AND "p".action = '*')
			    OR ("p".resource = '*' AND "p".action = '*')
			  )
		) OR EXISTS (
			SELECT 1
			FROM auth.account_permissions AS "ap"
			INNER JOIN auth.permissions AS "p" ON "p".id = "ap".permission_id
			WHERE "ap".account_id = "push_subscription".account_id
			  AND "ap".tenant_id = "push_subscription".tenant_id
			  AND "ap".granted = TRUE
			  AND (
			    ("p".resource = 'admin' AND "p".action = '*')
			    OR ("p".resource = '*' AND "p".action = '*')
			  )
		)`)
	query = base.WithTenantFilter(ctx, query, "push_subscription")
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find admin push subscriptions", Err: err}
	}
	return subs, nil
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
func (r *PushSubscriptionRepository) FindForGuardians(ctx context.Context, guardianAccountIDs []int64, studentIDs []int64) ([]*iot.PushSubscription, error) {
	if len(guardianAccountIDs) == 0 {
		return nil, nil
	}
	var subs []*iot.PushSubscription
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&subs).
		ModelTableExpr(tablePushSubscriptions+` AS "push_subscription"`).
		Join(activeAccountJoin).
		Join(activeAccountTenantJoin).
		Where(pushPortalFilter, iot.PushPortalParent).
		Where(`"account".active = ?`, true).
		Where(`"account_tenant".status = ?`, authModels.AccountTenantStatusActive).
		Where(guardianRoleFilter, authModels.BaseRoleGuardian).
		Where(`"push_subscription".account_id IN (?)`, bun.List(guardianAccountIDs))
	if len(studentIDs) > 0 {
		query = query.Where(childAccessFilter, bun.List(studentIDs), guardianPortalAccessPermission)
	}
	query = base.WithTenantFilter(ctx, query, "push_subscription")
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find guardian push subscriptions", Err: err}
	}
	return subs, nil
}
