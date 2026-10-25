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
//     docs/reference/notifications.md.
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
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

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
	if err := validateAudience(event); err != nil {
		return err
	}
	if event.DeepLink != "" && (!strings.HasPrefix(event.DeepLink, "/") ||
		strings.HasPrefix(event.DeepLink, "//") ||
		strings.Contains(event.DeepLink, `\`)) {
		return errors.New("deep link must be an app-relative path")
	}
	if event.SchoolDeepLink != "" && (!strings.HasPrefix(event.SchoolDeepLink, "/") ||
		strings.HasPrefix(event.SchoolDeepLink, "//") ||
		strings.Contains(event.SchoolDeepLink, `\`)) {
		return errors.New("school deep link must be an app-relative path")
	}
	switch event.Priority {
	case "", PriorityLow, PriorityNormal, PriorityHigh:
	default:
		return fmt.Errorf("unknown priority %q", event.Priority)
	}
	return nil
}

func (r *router) NotifySynchronously(ctx context.Context, event Event) error {
	if err := validate(event); err != nil {
		return err
	}
	if event.Priority == "" {
		event.Priority = PriorityNormal
	}
	if err := r.requireDispatch(ctx, event.Audience.TenantID, event.Type != TypeTest); err != nil {
		return err
	}
	if err := r.requireEventModules(ctx, &event); err != nil {
		return err
	}
	dispatchCtx := tenant.ContextWithoutAfterCommitHooks(tenant.ContextWithoutTransaction(ctx))
	// The durable producer waits for one channel, not for all of them. Every
	// other channel stays fire-and-forget exactly as in Notify — a parent with
	// the portal open must see the in-app notification even though only Web
	// Push acceptance decides whether the producer may mark the delivery done.
	// They run first so a push failure cannot skip them.
	r.deliverNonSynchronousChannels(dispatchCtx, event)
	for _, ch := range r.channels {
		if synchronous, ok := ch.(synchronousChannel); ok {
			started := time.Now()
			deliveryErr := synchronous.DeliverSynchronously(dispatchCtx, event)
			if r.observe != nil {
				r.observe(ch.Name(), event.Type, event.Type, time.Since(started), deliveryErr)
			}
			if deliveryErr != nil {
				return fmt.Errorf("synchronous notification channel %s: %w", ch.Name(), deliveryErr)
			}
		}
	}
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
// everything by construction. An unparseable or empty setting closes the
// window: corrupt configuration must never widen delivery into quiet hours.
func (r *router) withinActiveWindow(ctx context.Context, tenantID int64) (bool, error) {
	start, err := r.resolveWindowBound(ctx, tenantID, settingActiveWindowStart)
	if err != nil {
		return false, err
	}
	end, err := r.resolveWindowBound(ctx, tenantID, settingActiveWindowEnd)
	if err != nil {
		return false, err
	}
	if start < 0 || end < 0 {
		return false, nil
	}
	// Equal valid bounds explicitly represent an unrestricted 24-hour window.
	if start == end {
		return true, nil
	}

	now := calendar.Now()
	nowMin := now.Hour()*60 + now.Minute()
	return withinWindow(nowMin, start, end), nil
}

func withinWindow(nowMin, start, end int) bool {
	if start == end {
		return true
	}
	if start > end {
		return nowMin >= start || nowMin < end
	}
	return nowMin >= start && nowMin < end
}

// resolveWindowBound reads one "HH:MM" setting as a minute of day. Invalid
// values return -1 so the shared window check fails closed.
func (r *router) resolveWindowBound(ctx context.Context, tenantID int64, key string) (int, error) {
	raw, err := r.settings.ResolveStringForTenant(ctx, tenantID, key)
	if err != nil {
		return 0, fmt.Errorf("resolving %s: %w", key, err)
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		r.getLogger().Warn("blank notification window bound; treating window as closed",
			"key", key,
			"tenant_id", tenantID,
		)
		return -1, nil
	}
	parsed, err := time.Parse("15:04", raw)
	if err != nil {
		r.getLogger().Warn("malformed notification window bound; treating window as closed",
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
		if _, durable := ch.(durableChannel); durable {
			continue
		}
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
		if _, durable := ch.(durableChannel); durable {
			continue
		}
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

func (r *router) requireDispatch(ctx context.Context, tenantID int64, realDelivery bool) error {
	if r.settings == nil {
		return errors.New("notifications service has no settings service configured")
	}
	enabled, err := r.settings.ResolveBoolForTenant(ctx, tenantID, settingDispatchEnabled)
	if err != nil {
		return fmt.Errorf("resolving notification feature flag: %w", err)
	}
	if !enabled {
		return ErrDisabled
	}
	if realDelivery {
		within, err := r.withinActiveWindow(ctx, tenantID)
		if err != nil {
			return err
		}
		if !within {
			return ErrOutsideActiveWindow
		}
	}
	return nil
}

func prepareBatch(events []Event, tenantID int64) ([]Event, error) {
	prepared := make([]Event, 0, len(events))
	for _, event := range events {
		if err := validate(event); err != nil {
			return nil, err
		}
		if event.Audience.TenantID != tenantID {
			return nil, errors.New("notification batch must not mix tenants")
		}
		if event.Priority == "" {
			event.Priority = PriorityNormal
		}
		event.Data = maps.Clone(event.Data)
		event.Audience.GuardianAccountIDs = slices.Clone(event.Audience.GuardianAccountIDs)
		event.Audience.StaffAccountIDs = slices.Clone(event.Audience.StaffAccountIDs)
		event.Audience.StudentIDs = slices.Clone(event.Audience.StudentIDs)
		prepared = append(prepared, event)
	}

	return prepared, nil
}

func validateAudience(event Event) error {
	// A child scope only means something where it is enforced, which is the
	// guardian device lookup. Silently ignoring it elsewhere would let a producer
	// believe it had narrowed an audience it did not narrow.
	if len(event.Audience.StudentIDs) > 0 && event.Audience.Scope != ScopeGuardian {
		return fmt.Errorf("student-scoped audience requires the guardian scope, got %q", event.Audience.Scope)
	}
	for _, studentID := range event.Audience.StudentIDs {
		if studentID <= 0 {
			return errors.New("student-scoped audience requires positive student ids")
		}
	}
	switch event.Audience.Scope {
	case ScopeTenant:
	case ScopeAdmin:
	case ScopeGuardian:
		return validateGuardianAudience(event)
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
	return nil
}

func validateGuardianAudience(event Event) error {
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
	return nil
}

func (r *router) deliverNonSynchronousChannels(dispatchCtx context.Context, event Event) {
	for _, ch := range r.channels {
		if _, durable := ch.(durableChannel); durable {
			continue
		}
		if _, ok := ch.(synchronousChannel); ok {
			continue
		}
		if err := ch.Deliver(dispatchCtx, event); err != nil {
			r.getLogger().Error("notification channel delivery failed",
				"channel", ch.Name(),
				"notification_type", event.Type,
				"tenant_id", event.Audience.TenantID,
				"error", err.Error(),
			)
		}
	}
}
