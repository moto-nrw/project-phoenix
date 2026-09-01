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
		return &modelBase.DatabaseError{Op: "upsert notification preference", Err: base.TranslateNotFound(err)}
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
		return nil, &modelBase.DatabaseError{Op: "list notification preferences by account", Err: base.TranslateNotFound(err)}
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
		return nil, &modelBase.DatabaseError{Op: "filter opted-in accounts", Err: base.TranslateNotFound(err)}
	}
	return optedIn, nil
}

// HasAnyOptedIn answers whether the current tenant has at least one enabled
// decision for any named type. The partial notification-preference index makes
// this the cheap gate used before a scheduler resolves its full audience.
func (r *NotificationPreferenceRepository) HasAnyOptedIn(ctx context.Context, notificationTypes []string) (bool, error) {
	if len(notificationTypes) == 0 {
		return false, nil
	}

	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(tableExprNotificationPreferences).
		Where(`"notification_preference".notification_type IN (?)`, bun.List(notificationTypes)).
		Where(`"notification_preference".enabled`)
	query = base.WithTenantFilter(ctx, query, aliasNotificationPreference)

	exists, err := query.Exists(ctx)
	if err != nil {
		return false, &modelBase.DatabaseError{Op: "check for opted-in accounts", Err: base.TranslateNotFound(err)}
	}
	return exists, nil
}

// FilterOptedInByType narrows one candidate set for multiple notification types
// in a single query.
func (r *NotificationPreferenceRepository) FilterOptedInByType(
	ctx context.Context,
	notificationTypes []string,
	accountIDs []int64,
) (map[string][]int64, error) {
	optedInByType := make(map[string][]int64)
	if len(notificationTypes) == 0 || len(accountIDs) == 0 {
		return optedInByType, nil
	}

	var rows []struct {
		NotificationType string `bun:"notification_type"`
		AccountID        int64  `bun:"account_id"`
	}
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(tableExprNotificationPreferences).
		ColumnExpr(`"notification_preference".notification_type`).
		ColumnExpr(`"notification_preference".account_id`).
		Where(`"notification_preference".notification_type IN (?)`, bun.List(notificationTypes)).
		Where(`"notification_preference".enabled`).
		Where(`"notification_preference".account_id IN (?)`, bun.List(accountIDs)).
		OrderExpr(`"notification_preference".notification_type ASC`).
		OrderExpr(`"notification_preference".account_id ASC`)
	query = base.WithTenantFilter(ctx, query, aliasNotificationPreference)

	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "filter opted-in accounts by type", Err: base.TranslateNotFound(err)}
	}
	for _, row := range rows {
		optedInByType[row.NotificationType] = append(optedInByType[row.NotificationType], row.AccountID)
	}
	return optedInByType, nil
}

// FilterNotOptedOut removes only the candidates who explicitly declined the type.
//
// It reads the opt-out rows and subtracts them instead of selecting survivors in
// SQL: candidates without any row have nothing to join against, and the input
// order is preserved so callers keep a stable recipient list.
func (r *NotificationPreferenceRepository) FilterNotOptedOut(ctx context.Context, notificationType string, accountIDs []int64) ([]int64, error) {
	if notificationType == "" || len(accountIDs) == 0 {
		return nil, nil
	}

	var optedOut []int64

	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(tableExprNotificationPreferences).
		ColumnExpr(`"notification_preference".account_id`).
		Where(`"notification_preference".notification_type = ?`, notificationType).
		Where(`NOT "notification_preference".enabled`).
		Where(`"notification_preference".account_id IN (?)`, bun.List(accountIDs))

	query = base.WithTenantFilter(ctx, query, aliasNotificationPreference)

	if err := query.Scan(ctx, &optedOut); err != nil {
		return nil, &modelBase.DatabaseError{Op: "filter opted-out accounts", Err: base.TranslateNotFound(err)}
	}

	declined := make(map[int64]struct{}, len(optedOut))
	for _, accountID := range optedOut {
		declined[accountID] = struct{}{}
	}

	remaining := make([]int64, 0, len(accountIDs))
	for _, accountID := range accountIDs {
		if _, ok := declined[accountID]; ok {
			continue
		}
		remaining = append(remaining, accountID)
	}
	return remaining, nil
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
		return &modelBase.DatabaseError{Op: "disable all notification preferences", Err: base.TranslateNotFound(err)}
	}
	return nil
}
