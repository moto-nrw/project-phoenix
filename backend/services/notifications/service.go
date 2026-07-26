// Package notifications is the notification abstraction for issue #1624: a
// single entry point — Notify(ctx, event) — through which features trigger
// user-facing notifications without knowing the delivery channels.
//
// Design:
//
//   - Features build an Event (type, audience, priority, display-safe title/
//     body, deep link) and call Service.Notify. They never talk to a channel.
//   - The service checks the per-tenant feature flag
//     notifications.dispatch_enabled (default OFF) and, when enabled, fans the
//     event out to every registered Channel after the surrounding tenant
//     transaction commits. Channel failures are logged and never block the
//     caller (fire-and-forget, mirroring SSE broadcasting).
//   - Channels are pluggable: today an SSE/in-app channel (wrapping the
//     existing realtime.Broadcaster — the existing SSE cache-invalidation
//     events are untouched) and a web-push stub. E-mail and real Web Push are
//     future channels; see docs/notifications.md.
//
// GDPR contract: Title, Body and DeepLink are the ONLY user-visible fields and
// must be display-safe — no student names or other sensitive child data. A
// deep link points into the authenticated app, where sensitive details are
// loaded after login. Data carries opaque IDs only.
package notifications

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"strings"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Priority levels for a notification event.
const (
	PriorityLow    = "low"
	PriorityNormal = "normal"
	PriorityHigh   = "high"
)

// AudienceScope selects who receives a notification.
type AudienceScope string

const (
	// ScopeTenant delivers to every connected client of the tenant (staff app).
	ScopeTenant AudienceScope = "tenant"
	// ScopeGuardian delivers only to one guardian account's own clients.
	ScopeGuardian AudienceScope = "guardian"
	// ScopeGroup delivers to clients subscribed to one active group.
	ScopeGroup AudienceScope = "group"
)

// Audience describes the recipients of an event. TenantID is always required;
// the other fields depend on Scope.
type Audience struct {
	TenantID          int64
	Scope             AudienceScope
	GuardianAccountID int64  // required for ScopeGuardian
	ActiveGroupID     string // required for ScopeGroup
}

// Event is a channel-agnostic notification. Title/Body/DeepLink are
// display-safe by contract (see the package comment); Data carries additional
// non-sensitive values (opaque IDs) for client-side routing.
type Event struct {
	Type     string // e.g. "test", later "pickup_upcoming" (#669)
	Audience Audience
	Priority string // PriorityLow/Normal/High; empty defaults to normal
	Title    string
	Body     string
	DeepLink string // app-relative path ("/reminders"); never an absolute URL
	Data     map[string]string
}

// ErrDisabled is returned by Notify when notifications.dispatch_enabled is
// off for the event's tenant. Callers that fire notifications as a side
// effect should treat it as a silent no-op; the test endpoint surfaces it.
var ErrDisabled = errors.New("notifications are disabled for this tenant")

// Channel delivers an event over one transport. Implementations must be
// fire-and-forget-safe: a returned error is logged by the router and never
// propagated to the feature that triggered the event.
type Channel interface {
	Name() string
	Deliver(ctx context.Context, event Event) error
}

// Service is the entry point features use to trigger notifications.
type Service interface {
	// Notify validates the event, checks the tenant feature flag, and fans
	// the event out to all registered channels after the surrounding tenant
	// transaction commits. Returns ErrDisabled when the flag is off and a
	// validation error for malformed events; channel delivery failures are
	// logged, not returned.
	Notify(ctx context.Context, event Event) error
}

type router struct {
	settings configService.SettingsService
	channels []Channel
	logger   *slog.Logger
}

// NewService builds the channel router. Channels are fixed at wiring time.
func NewService(settings configService.SettingsService, logger *slog.Logger, channels ...Channel) Service {
	return &router{settings: settings, channels: channels, logger: logger}
}

func (r *router) getLogger() *slog.Logger {
	if r.logger == nil {
		return slog.Default()
	}
	return r.logger
}

// validate rejects malformed events before any channel sees them. It also
// enforces the shape of the GDPR contract that CAN be checked mechanically:
// a deep link must be app-relative so a payload can never smuggle users to an
// external site.
func validate(event Event) error {
	if event.Type == "" {
		return errors.New("notification event requires a type")
	}
	if event.Audience.TenantID <= 0 {
		return errors.New("notification event requires a tenant id")
	}
	if event.Title == "" {
		return errors.New("notification event requires a title")
	}
	switch event.Audience.Scope {
	case ScopeTenant:
	case ScopeGuardian:
		if event.Audience.GuardianAccountID <= 0 {
			return errors.New("guardian-scoped notification requires a guardian account id")
		}
	case ScopeGroup:
		if event.Audience.ActiveGroupID == "" {
			return errors.New("group-scoped notification requires an active group id")
		}
	default:
		return fmt.Errorf("unknown audience scope %q", event.Audience.Scope)
	}
	if event.DeepLink != "" && (!strings.HasPrefix(event.DeepLink, "/") ||
		strings.HasPrefix(event.DeepLink, "//") ||
		strings.Contains(event.DeepLink, `\`)) {
		return errors.New("deep link must be an app-relative path")
	}
	switch event.Priority {
	case "", PriorityLow, PriorityNormal, PriorityHigh:
	default:
		return fmt.Errorf("unknown priority %q", event.Priority)
	}
	return nil
}

func (r *router) Notify(ctx context.Context, event Event) error {
	if err := validate(event); err != nil {
		return err
	}
	if event.Priority == "" {
		event.Priority = PriorityNormal
	}

	if r.settings == nil {
		return errors.New("notifications service has no settings service configured")
	}
	// ResolveBoolForTenant works with and without tenant middleware, so
	// scheduler-style producers can notify outside a request context.
	enabled, err := r.settings.ResolveBoolForTenant(ctx, event.Audience.TenantID, configModel.KeyNotificationsDispatchEnabled)
	if err != nil {
		return fmt.Errorf("resolving notification feature flag: %w", err)
	}
	if !enabled {
		return ErrDisabled
	}

	// Channels run after commit, so they must not inherit the closed transaction.
	// Snapshot the mutable payload before the callback outlives this call.
	dispatchCtx := modelBase.ContextWithoutTx(ctx)
	event.Data = maps.Clone(event.Data)
	tenant.RegisterAfterCommit(ctx, func() {
		r.deliver(dispatchCtx, event)
	})
	return nil
}

func (r *router) deliver(ctx context.Context, event Event) {
	for _, ch := range r.channels {
		if err := ch.Deliver(ctx, event); err != nil {
			// Fire-and-forget per channel: one failing channel must neither
			// block the caller nor the remaining channels.
			r.getLogger().Error("notification channel delivery failed",
				"channel", ch.Name(),
				"notification_type", event.Type,
				"tenant_id", event.Audience.TenantID,
				"error", err.Error(),
			)
		}
	}
}
