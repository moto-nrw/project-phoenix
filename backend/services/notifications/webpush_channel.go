package notifications

import (
	"context"
	"log/slog"
)

// webPushChannel is the planned Web Push channel (#1624). It is a deliberate
// stub: per the issue, device/browser subscription persistence and VAPID
// delivery are only added when Web Push is actually activated. Registering
// the stub now fixes the extension point so producers already fan out to it
// and activating push later changes no caller.
//
// Activation plan (documented in docs/notifications.md):
//  1. Add an iot/notifications push_subscriptions table (account + tenant +
//     device endpoint, multiple devices per account) via migration.
//  2. Add a VAPID webpush dependency and keys via env/SOPS.
//  3. Replace Deliver below: load the audience's subscriptions and send ONLY
//     Title/Body/DeepLink — never Data — as the push payload (GDPR: sensitive
//     details are fetched authenticated after the app opens).
//  4. Add the frontend service worker + permission prompt.
type webPushChannel struct {
	logger *slog.Logger
}

// NewWebPushChannel returns the not-yet-active Web Push channel stub.
func NewWebPushChannel(logger *slog.Logger) Channel {
	return &webPushChannel{logger: logger}
}

func (c *webPushChannel) Name() string { return "web_push" }

func (c *webPushChannel) Deliver(_ context.Context, event Event) error {
	logger := c.logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Debug("web push channel not yet activated, skipping delivery",
		"notification_type", event.Type,
		"tenant_id", event.Audience.TenantID,
	)
	return nil
}
