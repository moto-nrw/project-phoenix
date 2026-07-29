package users

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const (
	tableNotificationPreferences     = "users.notification_preferences"
	tableExprNotificationPreferences = `users.notification_preferences AS "notification_preference"`
	aliasNotificationPreference      = "notification_preference"
)

// NotificationPreferenceRepository implements users.NotificationPreferenceRepository.
type NotificationPreferenceRepository struct {
	*base.Repository[*users.NotificationPreference]
	db *bun.DB
}

// NewNotificationPreferenceRepository creates a NotificationPreferenceRepository.
func NewNotificationPreferenceRepository(db *bun.DB) users.NotificationPreferenceRepository {
	repo := base.NewRepository[*users.NotificationPreference](db, tableNotificationPreferences, "NotificationPreference")
	repo.TenantScoped = true
	return &NotificationPreferenceRepository{Repository: repo, db: db}
}

// Upsert records one decision, overwriting the previous one for the same
// (tenant, account, type).
func (r *NotificationPreferenceRepository) Upsert(ctx context.Context, pref *users.NotificationPreference) error {
	base.EnsureTenantID(ctx, pref)

	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(pref).
		ModelTableExpr(tableNotificationPreferences).
		On("CONFLICT (tenant_id, account_id, notification_type) DO UPDATE").
		Set("enabled = EXCLUDED.enabled").
		Set("updated_at = NOW()").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "upsert notification preference", Err: err}
	}
	return nil
}

// ListByAccount returns every stored decision of one account in the current tenant.
func (r *NotificationPreferenceRepository) ListByAccount(ctx context.Context, accountID int64) ([]*users.NotificationPreference, error) {
	var prefs []*users.NotificationPreference

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&prefs).
		ModelTableExpr(tableExprNotificationPreferences).
		Where(`"notification_preference".account_id = ?`, accountID)

	query = base.WithTenantFilter(ctx, query, aliasNotificationPreference)

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list notification preferences by account", Err: err}
	}
	return prefs, nil
}

// FilterOptedIn narrows candidate recipients to those who agreed to the type.
//
// The empty-input short circuit matters: a producer that resolved no candidates
// must end up with no recipients, never with "everyone". Enforcing that here
// rather than in each producer keeps one answer to "has this person agreed".
func (r *NotificationPreferenceRepository) FilterOptedIn(ctx context.Context, notificationType string, accountIDs []int64) ([]int64, error) {
	if notificationType == "" || len(accountIDs) == 0 {
		return nil, nil
	}

	var optedIn []int64

	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(tableExprNotificationPreferences).
		ColumnExpr(`"notification_preference".account_id`).
		Where(`"notification_preference".notification_type = ?`, notificationType).
		Where(`"notification_preference".enabled`).
		Where(`"notification_preference".account_id IN (?)`, bun.List(accountIDs))

	query = base.WithTenantFilter(ctx, query, aliasNotificationPreference)

	if err := query.Scan(ctx, &optedIn); err != nil {
		return nil, &modelBase.DatabaseError{Op: "filter opted-in accounts", Err: err}
	}
	return optedIn, nil
}

// DisableAllForAccount switches the named stored decisions of one account off.
func (r *NotificationPreferenceRepository) DisableAllForAccount(ctx context.Context, accountID int64, notificationTypes []string) error {
	if len(notificationTypes) == 0 {
		return nil
	}
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*users.NotificationPreference)(nil)).
		ModelTableExpr(tableNotificationPreferences).
		Set("enabled = FALSE").
		Set("updated_at = NOW()").
		Where("account_id = ?", accountID).
		Where("notification_type IN (?)", bun.List(notificationTypes)).
		Where("enabled")

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	if _, err := query.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "disable all notification preferences", Err: err}
	}
	return nil
}
