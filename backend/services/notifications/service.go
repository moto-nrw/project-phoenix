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
//     events are untouched) and the Web Push channel (#2003, VAPID-signed
//     pushes to registered devices). E-mail is a future channel; see
//     docs/notifications.md.
//
// GDPR contract: Title, Body and DeepLink are the ONLY user-visible fields and
// must be display-safe — no student names or other sensitive child data. The
// audience must also match the visibility of the source data; display safety
// does not authorize broader delivery. A deep link points into the
// authenticated app, where sensitive details are loaded after login. Data
// carries opaque IDs only.
package notifications

import (
	"context"
	"errors"

	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"

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
	// ScopeAdmin delivers only to connected staff clients with effective admin
	// scope in the tenant.
	ScopeAdmin AudienceScope = "admin"
	// ScopeGuardian delivers only to one or more guardian accounts' own clients.
	ScopeGuardian AudienceScope = "guardian"
	// ScopeGroup delivers to clients subscribed to one active group.
	ScopeGroup AudienceScope = "group"
	// ScopeStaff delivers to one or more named staff accounts' own clients and
	// devices. This is the scope for personal notifications, where each
	// recipient gets their own payload.
	ScopeStaff AudienceScope = "staff"
)

// Audience describes the recipients of an event. TenantID is always required;
// the other fields depend on Scope.
type Audience struct {
	TenantID           int64
	Scope              AudienceScope
	GuardianAccountID  int64   // one recipient for ScopeGuardian
	GuardianAccountIDs []int64 // batched recipients for ScopeGuardian
	ActiveGroupID      string  // required for ScopeGroup
	StaffAccountIDs    []int64 // recipients for ScopeStaff (auth.accounts.id)
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

// ErrOutsideActiveWindow is returned when a tenant's delivery window has
// closed for the day. Producers treat it like ErrDisabled: a silent no-op.
var ErrOutsideActiveWindow = errors.New("outside the tenant's notification window")

// Channel delivers an event over one transport. Implementations must be
// fire-and-forget-safe: a returned error is logged by the router and never
// propagated to the feature that triggered the event.
type Channel interface {
	Name() string
	Deliver(ctx context.Context, event Event) error
}

// BatchChannel is implemented by channels that can serve many events at once
// more cheaply than one at a time. The router uses it when available and loops
// Deliver otherwise, so implementing it is always optional.
type BatchChannel interface {
	Channel
	DeliverBatch(ctx context.Context, events []Event) error
}

// BatchNotifier dispatches many events sharing one tenant in one pass.
//
// Kept separate from Service because several test doubles implement Service and
// have no business growing a batch method.
type BatchNotifier interface {
	NotifyBatch(ctx context.Context, events []Event) error
}

// Notifier is the full surface the concrete service provides. It is assignable
// to Service, so consumers that only notify one audience keep their narrower
// type.
type Notifier interface {
	Service
	BatchNotifier
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

// TypeTest is the notification an admin can fire to verify the setup. It is
// the one type exempt from the delivery window.
const TypeTest = "test"

// NewService builds the channel router. Channels are fixed at wiring time.
// It returns Notifier so wiring can reach NotifyBatch; the value stays
// assignable to Service for consumers that only notify one audience.
func NewService(settings configService.SettingsService, logger *slog.Logger, channels ...Channel) Notifier {
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
	case ScopeAdmin:
	case ScopeGuardian:
		if event.Audience.GuardianAccountID > 0 && len(event.Audience.GuardianAccountIDs) > 0 {
			return errors.New("guardian-scoped notification cannot mix singular and batched account ids")
		}
		if event.Audience.GuardianAccountID <= 0 && len(event.Audience.GuardianAccountIDs) == 0 {
			return errors.New("guardian-scoped notification requires a guardian account id")
		}
		for _, accountID := range event.Audience.GuardianAccountIDs {
			if accountID <= 0 {
				return errors.New("guardian-scoped notification requires positive guardian account ids")
			}
		}
	case ScopeGroup:
		if event.Audience.ActiveGroupID == "" {
			return errors.New("group-scoped notification requires an active group id")
		}
	case ScopeStaff:
		if len(event.Audience.StaffAccountIDs) == 0 {
			return errors.New("staff-scoped notification requires at least one staff account id")
		}
		for _, accountID := range event.Audience.StaffAccountIDs {
			if accountID <= 0 {
				return errors.New("staff-scoped notification requires positive staff account ids")
			}
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

	// The test notification is exempt from the delivery window on purpose: an
	// admin verifying the setup in the evening must get an answer, not silence
	// that looks like a broken configuration.
	if event.Type != TypeTest {
		within, werr := r.withinActiveWindow(ctx, event.Audience.TenantID)
		if werr != nil {
			return werr
		}
		if !within {
			return ErrOutsideActiveWindow
		}
	}

	// Channels run after commit, so they must not inherit the closed transaction.
	// Snapshot the mutable payload before the callback outlives this call.
	dispatchCtx := modelBase.ContextWithoutTx(ctx)
	event.Data = maps.Clone(event.Data)
	event.Audience.GuardianAccountIDs = slices.Clone(event.Audience.GuardianAccountIDs)
	event.Audience.StaffAccountIDs = slices.Clone(event.Audience.StaffAccountIDs)
	tenant.RegisterAfterCommit(ctx, func() {
		r.deliver(dispatchCtx, event)
	})
	return nil
}

// NotifyBatch dispatches many events that share one tenant in a single
// after-commit hook.
//
// It exists because the per-recipient shape of personal notifications would
// otherwise multiply the fixed cost of Notify: one feature-flag read and one
// delivery pass per event. Here the flag is resolved once, and a channel that
// can group its own work gets the whole batch through DeliverBatch.
//
// Validation is all-or-nothing: one malformed event fails the batch rather than
// silently dropping a recipient.
func (r *router) NotifyBatch(ctx context.Context, events []Event) error {
	if len(events) == 0 {
		return nil
	}

	tenantID := events[0].Audience.TenantID
	prepared := make([]Event, 0, len(events))
	for _, event := range events {
		if err := validate(event); err != nil {
			return err
		}
		if event.Audience.TenantID != tenantID {
			return errors.New("notification batch must not mix tenants")
		}
		if event.Priority == "" {
			event.Priority = PriorityNormal
		}
		event.Data = maps.Clone(event.Data)
		event.Audience.GuardianAccountIDs = slices.Clone(event.Audience.GuardianAccountIDs)
		event.Audience.StaffAccountIDs = slices.Clone(event.Audience.StaffAccountIDs)
		prepared = append(prepared, event)
	}

	if r.settings == nil {
		return errors.New("notifications service has no settings service configured")
	}
	enabled, err := r.settings.ResolveBoolForTenant(ctx, tenantID, configModel.KeyNotificationsDispatchEnabled)
	if err != nil {
		return fmt.Errorf("resolving notification feature flag: %w", err)
	}
	if !enabled {
		return ErrDisabled
	}
	within, err := r.withinActiveWindow(ctx, tenantID)
	if err != nil {
		return err
	}
	if !within {
		return ErrOutsideActiveWindow
	}

	dispatchCtx := modelBase.ContextWithoutTx(ctx)
	tenant.RegisterAfterCommit(ctx, func() {
		r.deliverBatch(dispatchCtx, prepared)
	})
	return nil
}

func guardianAccountIDs(audience Audience) []int64 {
	if len(audience.GuardianAccountIDs) > 0 {
		return audience.GuardianAccountIDs
	}
	if audience.GuardianAccountID > 0 {
		return []int64{audience.GuardianAccountID}
	}
	return nil
}

// staffAccountIDs returns the recipients of a staff-scoped event.
func staffAccountIDs(audience Audience) []int64 {
	return audience.StaffAccountIDs
}

// withinActiveWindow reports whether the tenant currently accepts delivery.
//
// The check lives here rather than in each producer so quiet hours hold for
// everything by construction. An unparseable or empty setting means "no
// restriction": a broken time value must not silence a school outright.
func (r *router) withinActiveWindow(ctx context.Context, tenantID int64) (bool, error) {
	start, err := r.resolveWindowBound(ctx, tenantID, configModel.KeyNotificationsActiveWindowStart)
	if err != nil {
		return false, err
	}
	end, err := r.resolveWindowBound(ctx, tenantID, configModel.KeyNotificationsActiveWindowEnd)
	if err != nil {
		return false, err
	}
	if start < 0 || end < 0 || start >= end {
		return true, nil
	}

	now := timezone.Now()
	nowMin := now.Hour()*60 + now.Minute()
	return nowMin >= start && nowMin < end, nil
}

// resolveWindowBound reads one "HH:MM" setting as a minute of day, or -1 when
// it is unset or malformed.
func (r *router) resolveWindowBound(ctx context.Context, tenantID int64, key string) (int, error) {
	raw, err := r.settings.ResolveStringForTenant(ctx, tenantID, key)
	if err != nil {
		return 0, fmt.Errorf("resolving %s: %w", key, err)
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return -1, nil
	}
	parsed, err := time.Parse("15:04", raw)
	if err != nil {
		r.getLogger().Warn("ignoring malformed notification window bound",
			"key", key,
			"tenant_id", tenantID,
			"value", raw,
		)
		return -1, nil
	}
	return parsed.Hour()*60 + parsed.Minute(), nil
}

// deliverBatch hands the whole batch to channels that can group their work and
// falls back to one Deliver per event for the others.
func (r *router) deliverBatch(ctx context.Context, events []Event) {
	for _, ch := range r.channels {
		if batch, ok := ch.(BatchChannel); ok {
			if err := batch.DeliverBatch(ctx, events); err != nil {
				r.getLogger().Error("notification channel batch delivery failed",
					"channel", ch.Name(),
					"event_count", len(events),
					"error", err.Error(),
				)
			}
			continue
		}
		for _, event := range events {
			if err := ch.Deliver(ctx, event); err != nil {
				r.getLogger().Error("notification channel delivery failed",
					"channel", ch.Name(),
					"notification_type", event.Type,
					"tenant_id", event.Audience.TenantID,
					"error", err.Error(),
				)
			}
		}
	}
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
