package iot

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const tablePushSubscriptions = "iot.push_subscriptions"

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

// FindForTenantStaff returns all staff-portal subscriptions of the current tenant.
func (r *PushSubscriptionRepository) FindForTenantStaff(ctx context.Context) ([]*iot.PushSubscription, error) {
	var subs []*iot.PushSubscription
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&subs).
		ModelTableExpr(tablePushSubscriptions+` AS "push_subscription"`).
		Where("portal = ?", iot.PushPortalStaff)
	query = base.WithTenantFilter(ctx, query, "push_subscription")
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find staff push subscriptions", Err: err}
	}
	return subs, nil
}

// FindForTenantAdmins returns staff-portal subscriptions of accounts holding
// the admin role in the current tenant. Mirrors the effective-admin scope the
// SSE hub uses (literal admin role; wildcard-permission accounts are admins
// by role in practice).
func (r *PushSubscriptionRepository) FindForTenantAdmins(ctx context.Context) ([]*iot.PushSubscription, error) {
	var subs []*iot.PushSubscription
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&subs).
		ModelTableExpr(tablePushSubscriptions+` AS "push_subscription"`).
		Where("portal = ?", iot.PushPortalStaff).
		Where(`EXISTS (
			SELECT 1
			FROM auth.account_roles AS "ar"
			INNER JOIN auth.roles AS "r" ON "r".id = "ar".role_id
			WHERE "ar".account_id = "push_subscription".account_id
			  AND "ar".tenant_id = "push_subscription".tenant_id
			  AND LOWER("r".name) = 'admin'
		)`)
	query = base.WithTenantFilter(ctx, query, "push_subscription")
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find admin push subscriptions", Err: err}
	}
	return subs, nil
}

// FindForGuardian returns parent-portal subscriptions of one guardian account
// in the current tenant.
func (r *PushSubscriptionRepository) FindForGuardian(ctx context.Context, guardianAccountID int64) ([]*iot.PushSubscription, error) {
	var subs []*iot.PushSubscription
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&subs).
		ModelTableExpr(tablePushSubscriptions+` AS "push_subscription"`).
		Where("portal = ?", iot.PushPortalParent).
		Where("account_id = ?", guardianAccountID)
	query = base.WithTenantFilter(ctx, query, "push_subscription")
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find guardian push subscriptions", Err: err}
	}
	return subs, nil
}
