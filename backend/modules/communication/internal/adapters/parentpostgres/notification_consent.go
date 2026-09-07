package parentpostgres

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	notificationPreferenceTable     = "users.notification_preferences"
	notificationPreferenceTableExpr = `users.notification_preferences AS "notification_preference"`
	notificationPreferenceAlias     = "notification_preference"
)

// NotificationConsentStore persists one account's decision per notification
// type and school. Absence of a row means "not agreed"; a row with
// enabled = false is a deliberate opt-out that a later change of defaults must
// not overrule.
type NotificationConsentStore struct{ store }

// NewNotificationConsentStore builds the consent store over the ambient
// transaction runtime.
func NewNotificationConsentStore(database Database) *NotificationConsentStore {
	return &NotificationConsentStore{store: newStore(database)}
}

type notificationPreferenceRow struct {
	bun.BaseModel    `bun:"table:users.notification_preferences,alias:notification_preference"`
	ID               int64  `bun:"id,pk,autoincrement"`
	TenantID         int64  `bun:"tenant_id,notnull"`
	AccountID        int64  `bun:"account_id,notnull"`
	NotificationType string `bun:"notification_type,notnull"`
	Enabled          bool   `bun:"enabled,notnull"`
}

// StoredConsent returns every decision the account stored in the current
// tenant, keyed by notification type. Types the account never touched are
// simply absent.
func (s *NotificationConsentStore) StoredConsent(ctx context.Context, accountID int64) (map[string]bool, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []notificationPreferenceRow
	query := db.NewSelect().
		Model(&rows).
		ModelTableExpr(notificationPreferenceTableExpr).
		Where(`"notification_preference".account_id = ?`, accountID)
	query = withTenant(query, notificationPreferenceAlias, tenantID)
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("list notification preferences by account: %w", err)
	}
	stored := make(map[string]bool, len(rows))
	for _, row := range rows {
		stored[row.NotificationType] = row.Enabled
	}
	return stored, nil
}

// RecordConsent stores one decision, overwriting any previous one for the same
// (tenant, account, type).
func (s *NotificationConsentStore) RecordConsent(ctx context.Context, accountID int64, notificationType string, enabled bool) error {
	if accountID <= 0 {
		return fmt.Errorf("record notification preference: account ID is required")
	}
	if notificationType == "" {
		return fmt.Errorf("record notification preference: notification type is required")
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	row := &notificationPreferenceRow{
		TenantID:         tenantID,
		AccountID:        accountID,
		NotificationType: notificationType,
		Enabled:          enabled,
	}
	if _, err := db.NewInsert().
		Model(row).
		ModelTableExpr(notificationPreferenceTable).
		ExcludeColumn("id").
		On("CONFLICT (tenant_id, account_id, notification_type) DO UPDATE").
		Set("enabled = EXCLUDED.enabled").
		Set("updated_at = NOW()").
		Exec(ctx); err != nil {
		return fmt.Errorf("upsert notification preference: %w", err)
	}
	return nil
}

// HasAnyOptedIn answers whether the current tenant holds at least one enabled
// row for any named type. It is a cheap scheduling gate only; it does not
// establish that the stored account is still an eligible recipient.
func (s *NotificationConsentStore) HasAnyOptedIn(ctx context.Context, notificationTypes []string) (bool, error) {
	if len(notificationTypes) == 0 {
		return false, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, err
	}
	query := db.NewSelect().
		TableExpr(notificationPreferenceTableExpr).
		Where(`"notification_preference".notification_type IN (?)`, bun.List(notificationTypes)).
		Where(`"notification_preference".enabled`)
	query = withTenant(query, notificationPreferenceAlias, tenantID)
	exists, err := query.Exists(ctx)
	if err != nil {
		return false, fmt.Errorf("check for opted-in accounts: %w", err)
	}
	return exists, nil
}

// FilterOptedIn narrows candidate recipients to those who agreed to the type.
//
// The empty-input short circuit matters: a producer that resolved no
// candidates must end up with no recipients, never with "everyone".
func (s *NotificationConsentStore) FilterOptedIn(ctx context.Context, notificationType string, accountIDs []int64) ([]int64, error) {
	if notificationType == "" || len(accountIDs) == 0 {
		return nil, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var optedIn []int64
	query := db.NewSelect().
		TableExpr(notificationPreferenceTableExpr).
		ColumnExpr(`"notification_preference".account_id`).
		Where(`"notification_preference".notification_type = ?`, notificationType).
		Where(`"notification_preference".enabled`).
		Where(`"notification_preference".account_id IN (?)`, bun.List(accountIDs))
	query = withTenant(query, notificationPreferenceAlias, tenantID)
	if err := query.Scan(ctx, &optedIn); err != nil {
		return nil, fmt.Errorf("filter opted-in accounts: %w", err)
	}
	return optedIn, nil
}

// FilterOptedInByType narrows one candidate set for multiple types in a single
// query, keyed by notification type.
func (s *NotificationConsentStore) FilterOptedInByType(
	ctx context.Context,
	notificationTypes []string,
	accountIDs []int64,
) (map[string][]int64, error) {
	optedInByType := make(map[string][]int64)
	if len(notificationTypes) == 0 || len(accountIDs) == 0 {
		return optedInByType, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		NotificationType string `bun:"notification_type"`
		AccountID        int64  `bun:"account_id"`
	}
	query := db.NewSelect().
		TableExpr(notificationPreferenceTableExpr).
		ColumnExpr(`"notification_preference".notification_type`).
		ColumnExpr(`"notification_preference".account_id`).
		Where(`"notification_preference".notification_type IN (?)`, bun.List(notificationTypes)).
		Where(`"notification_preference".enabled`).
		Where(`"notification_preference".account_id IN (?)`, bun.List(accountIDs)).
		OrderExpr(`"notification_preference".notification_type ASC`).
		OrderExpr(`"notification_preference".account_id ASC`)
	query = withTenant(query, notificationPreferenceAlias, tenantID)
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("filter opted-in accounts by type: %w", err)
	}
	for _, row := range rows {
		optedInByType[row.NotificationType] = append(optedInByType[row.NotificationType], row.AccountID)
	}
	return optedInByType, nil
}

// FilterNotOptedOut is the mirror of FilterOptedIn: it removes only the
// candidates who explicitly declined the type. It reads the opt-out rows and
// subtracts them instead of selecting survivors in SQL, so candidates without
// any row have nothing to join against and the caller's order is preserved.
func (s *NotificationConsentStore) FilterNotOptedOut(ctx context.Context, notificationType string, accountIDs []int64) ([]int64, error) {
	if notificationType == "" || len(accountIDs) == 0 {
		return nil, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var optedOut []int64
	query := db.NewSelect().
		TableExpr(notificationPreferenceTableExpr).
		ColumnExpr(`"notification_preference".account_id`).
		Where(`"notification_preference".notification_type = ?`, notificationType).
		Where(`NOT "notification_preference".enabled`).
		Where(`"notification_preference".account_id IN (?)`, bun.List(accountIDs))
	query = withTenant(query, notificationPreferenceAlias, tenantID)
	if err := query.Scan(ctx, &optedOut); err != nil {
		return nil, fmt.Errorf("filter opted-out accounts: %w", err)
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

// DisableConsent switches the named stored decisions of one account off. Types
// with no row stay absent — they are already off.
func (s *NotificationConsentStore) DisableConsent(ctx context.Context, accountID int64, notificationTypes []string) error {
	if len(notificationTypes) == 0 {
		return nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	query := db.NewUpdate().
		Model((*notificationPreferenceRow)(nil)).
		ModelTableExpr(notificationPreferenceTable).
		Set("enabled = FALSE").
		Set("updated_at = NOW()").
		Where("account_id = ?", accountID).
		Where("notification_type IN (?)", bun.List(notificationTypes)).
		Where("enabled")
	if tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}
	if _, err := query.Exec(ctx); err != nil {
		return fmt.Errorf("disable all notification preferences: %w", err)
	}
	return nil
}
