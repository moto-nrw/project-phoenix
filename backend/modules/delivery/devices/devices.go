// Package devices exposes Delivery's parent device and notification consent capabilities.
package devices

import (
	"context"
	"errors"
)

// PushSubscriptions manages only the authenticated parent's devices.
// The root preserves Delivery's atomic fan-out across active school mappings.
type PushSubscriptions interface {
	PublicKey() (string, error)
	SubscribeParent(context.Context, int64, PushSubscriptionInput) error
	UnsubscribeParent(context.Context, int64, string) error
}

// PushSubscriptionInput is the device information this adapter accepts.
type PushSubscriptionInput struct {
	Endpoint      string
	P256dh        string
	Auth          string
	UserAgent     string
	TokenFamilyID string
}

// NotificationPreferences reads and writes the parent's cross-school decisions.
type NotificationPreferences interface {
	SupportedLocale(string) bool
	GetForParent(context.Context, int64) (*PreferenceOverview, error)
	SetForParent(context.Context, int64, string, bool) error
	DisableAllForParent(context.Context, int64) error
}

// PreferenceOverview preserves the parent's notification preference response.
type PreferenceOverview struct {
	TenantEnabled bool              `json:"tenant_enabled"`
	Types         []PreferenceState `json:"types"`
}

// PreferenceState is one notification choice displayed by the portal.
type PreferenceState struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Group       string `json:"group"`
	Enabled     bool   `json:"enabled"`
	Available   bool   `json:"available"`
}

// These error identities are translated by the root without changing diagnostics.
var (
	ErrWebPushNotConfigured    = errors.New("web push is not configured on this server")
	ErrInvalidPushSubscription = errors.New("invalid push subscription")
	ErrUnknownNotificationType = errors.New("unknown notification type")
)
