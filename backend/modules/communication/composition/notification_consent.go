package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/parentpostgres"
	"github.com/uptrace/bun"
)

// NotificationConsentConfig wires Communication's consent store. Observe is
// optional; when set, every operation reports its duration and error under a
// stable name.
type NotificationConsentConfig struct {
	DB      *bun.DB
	Observe func(Observation)
}

// NewNotificationConsent builds the owner capability over
// users.notification_preferences.
func NewNotificationConsent(cfg NotificationConsentConfig) (communication.NotificationConsentCapability, error) {
	if cfg.DB == nil {
		return nil, errors.New("communication compose: notification consent requires a database")
	}
	return &notificationConsent{
		store:   parentpostgres.NewNotificationConsentStore(parentDatabase(cfg.DB)),
		observe: cfg.Observe,
	}, nil
}

type notificationConsent struct {
	store   *parentpostgres.NotificationConsentStore
	observe func(Observation)
}

func (c *notificationConsent) StoredConsent(ctx context.Context, accountID int64) (map[string]bool, error) {
	return observeResult(ctx, c.observe, "notification_consent.stored", func(runCtx context.Context) (map[string]bool, error) {
		return c.store.StoredConsent(runCtx, accountID)
	})
}

func (c *notificationConsent) RecordConsent(ctx context.Context, accountID int64, notificationType string, enabled bool) error {
	_, err := observeResult(ctx, c.observe, "notification_consent.record", func(runCtx context.Context) (struct{}, error) {
		return struct{}{}, c.store.RecordConsent(runCtx, accountID, notificationType, enabled)
	})
	return err
}

func (c *notificationConsent) DisableConsent(ctx context.Context, accountID int64, notificationTypes []string) error {
	_, err := observeResult(ctx, c.observe, "notification_consent.disable", func(runCtx context.Context) (struct{}, error) {
		return struct{}{}, c.store.DisableConsent(runCtx, accountID, notificationTypes)
	})
	return err
}

func (c *notificationConsent) HasAnyOptedIn(ctx context.Context, notificationTypes []string) (bool, error) {
	return observeResult(ctx, c.observe, "notification_consent.has_any_opted_in", func(runCtx context.Context) (bool, error) {
		return c.store.HasAnyOptedIn(runCtx, notificationTypes)
	})
}

func (c *notificationConsent) FilterOptedIn(ctx context.Context, notificationType string, accountIDs []int64) ([]int64, error) {
	return observeResult(ctx, c.observe, "notification_consent.filter_opted_in", func(runCtx context.Context) ([]int64, error) {
		return c.store.FilterOptedIn(runCtx, notificationType, accountIDs)
	})
}

func (c *notificationConsent) FilterOptedInByType(ctx context.Context, notificationTypes []string, accountIDs []int64) (map[string][]int64, error) {
	return observeResult(ctx, c.observe, "notification_consent.filter_opted_in_by_type", func(runCtx context.Context) (map[string][]int64, error) {
		return c.store.FilterOptedInByType(runCtx, notificationTypes, accountIDs)
	})
}

func (c *notificationConsent) FilterNotOptedOut(ctx context.Context, notificationType string, accountIDs []int64) ([]int64, error) {
	return observeResult(ctx, c.observe, "notification_consent.filter_not_opted_out", func(runCtx context.Context) ([]int64, error) {
		return c.store.FilterNotOptedOut(runCtx, notificationType, accountIDs)
	})
}

var _ communication.NotificationConsentCapability = (*notificationConsent)(nil)
