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
	"log/slog"
	"maps"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery/devices"
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

	// StudentIDs marks a guardian-scoped event as being ABOUT these children, and
	// is an authorization instruction rather than payload: the delivery
	// transaction re-reads the guardian relationships and keeps only recipients who
	// still hold parent_portal.access for at least one of them.
	//
	// Producers decide their audience in the transaction that produced the event;
	// devices are resolved later, in a transaction of their own. Between the two
	// a school can revoke a guardian's access to a child, and a push is rendered
	// on a lock screen — so the question is answered again where the sending
	// happens. It can only narrow the audience the producer chose, never widen
	// it: every listed child was already part of what the producer resolved, so
	// "at least one" cannot admit a recipient the producer excluded.
	//
	// One child for a message or a decided request, the children an appointment
	// was addressed to for the calendar producers. Leave it empty only for events
	// that are about no child at all (broadcast announcements); those are gated
	// by their own producer.
	StudentIDs []int64
}

// Event is a channel-agnostic notification. Title/Body/DeepLink are
// display-safe by contract (see the package comment); Data carries additional
// non-sensitive values (opaque IDs) for client-side routing.
type Event struct {
	Type           string // e.g. "test", later "pickup_upcoming" (#669)
	IdempotencyKey string
	RelatedType    string
	RelatedID      int64
	Audience       Audience
	Priority       string // PriorityLow/Normal/High; empty defaults to normal
	Title          string
	Body           string
	DeepLink       string // app-relative path ("/reminders"); never an absolute URL
	// SchoolDeepLink is the same destination on the school portal (#2208),
	// as an app-relative /school/... path. Empty means "this notification has
	// no place in moto schule"; a school device then opens the portal root.
	SchoolDeepLink string
	Data           map[string]string

	// Portal names the portal that asked for the event. It is only consulted
	// for TypeTest (#2208): the test notification proves the setup of the
	// portal the person is standing in, so it must not fan out into the other
	// staff portal. Empty means PortalStaff. Catalogue types ignore it — where
	// they are delivered is decided by the catalogue, not by the producer.
	Portal string
}

// ErrDisabled is returned by Notify when notifications.dispatch_enabled is
// off for the event's tenant. Callers that fire notifications as a side
// effect should treat it as a silent no-op; the test endpoint surfaces it.
var ErrDisabled = devices.ErrNotificationsDisabled

// ErrOutsideActiveWindow is returned when a tenant's delivery window has
// closed for the day. Producers treat it like ErrDisabled: a silent no-op.
var ErrOutsideActiveWindow = devices.ErrOutsideNotificationWindow

// Channel delivers an event over one transport. Implementations must be
// fire-and-forget-safe: a returned error is logged by the router and never
// propagated to the feature that triggered the event.
type Channel interface {
	Name() string
	Deliver(ctx context.Context, event Event) error
}

type durableChannel interface {
	Channel
	durableChannel()
}

// durableReceiptChannel reports how many durable intents it recorded. It is
// used by producers that hold a claim and must not mark it complete when a
// notification was only accepted by the router, but no delivery intent exists.
type durableReceiptChannel interface {
	durableChannel
	DeliverDurably(context.Context, Event) (int, error)
}

// synchronousChannel reports Web Push acceptance to a producer that must not
// mark a durable delivery complete until the push service has responded.
type synchronousChannel interface {
	Channel
	DeliverSynchronously(ctx context.Context, event Event) error
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

// SynchronousService is for durable producers such as appointment reminders.
// Unlike Notify, it waits for Web Push acceptance and returns delivery errors.
type SynchronousService interface {
	NotifySynchronously(ctx context.Context, event Event) error
}

// DurableReceiptService writes durable notification intents and reports how
// many of them were recorded. It keeps the write in the caller's transaction,
// unlike SynchronousService, which contacts the push provider directly.
type DurableReceiptService interface {
	NotifyDurably(ctx context.Context, event Event) (int, error)
}

// Service is the entry point features use to trigger notifications.
type Service interface {
	// Notify validates the event, checks the tenant feature flag, and writes
	// durable channel intents in the caller's transaction. Enqueue failures are
	// returned. Ephemeral channels run best-effort after commit.
	Notify(ctx context.Context, event Event) error
}

// RouterSettings resolves the dispatch flag and delivery window of an event's
// tenant.
type RouterSettings interface {
	ResolveBoolForTenant(ctx context.Context, tenantID int64, key string) (bool, error)
	ResolveStringForTenant(ctx context.Context, tenantID int64, key string) (string, error)
}

type router struct {
	settings RouterSettings
	channels []Channel
	logger   *slog.Logger
	observe  DeliveryObserver
}

// DeliveryObserver records one synchronous transport outcome. All arguments
// are stable catalogue labels and must never contain recipient data.
type DeliveryObserver func(transport, template, caller string, duration time.Duration, err error)

// TypeTest is the notification an admin can fire to verify the setup. It is
// the one type exempt from the delivery window.
const TypeTest = "test"

// NewService builds the channel router. Channels are fixed at wiring time.
// It returns Notifier so wiring can reach NotifyBatch; the value stays
// assignable to Service for consumers that only notify one audience.
func NewService(settings RouterSettings, logger *slog.Logger, channels ...Channel) Notifier {
	return &router{settings: settings, channels: channels, logger: logger}
}

// NewServiceWithDeliveryObserver builds the production router with runtime
// evidence for fail-closed sends. NewService remains the small test and local
// adapter constructor.
func NewServiceWithDeliveryObserver(settings RouterSettings, logger *slog.Logger, observe DeliveryObserver, channels ...Channel) Notifier {
	service := NewService(settings, logger, channels...).(*router)
	service.observe = observe
	return service
}

func (r *router) getLogger() *slog.Logger {
	if r.logger == nil {
		return slog.Default()
	}
	return r.logger
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
	enabled, err := r.settings.ResolveBoolForTenant(ctx, event.Audience.TenantID, settingDispatchEnabled)
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

	for _, channel := range r.channels {
		if _, durable := channel.(durableChannel); !durable {
			continue
		}
		if err := channel.Deliver(ctx, event); err != nil {
			return fmt.Errorf("enqueue durable notification channel %s: %w", channel.Name(), err)
		}
	}

	// Ephemeral channels run after commit, so they must not inherit the closed transaction.
	// Snapshot the mutable payload before the callback outlives this call.
	dispatchCtx := tenant.ContextWithoutAfterCommitHooks(tenant.ContextWithoutTransaction(ctx))
	event.Data = maps.Clone(event.Data)
	event.Audience.GuardianAccountIDs = slices.Clone(event.Audience.GuardianAccountIDs)
	event.Audience.StaffAccountIDs = slices.Clone(event.Audience.StaffAccountIDs)
	event.Audience.StudentIDs = slices.Clone(event.Audience.StudentIDs)
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
	prepared, err := prepareBatch(events, tenantID)
	if err != nil {
		return err
	}
	if err := r.requireDispatch(ctx, tenantID, slices.ContainsFunc(prepared, func(event Event) bool { return event.Type != TypeTest })); err != nil {
		return err
	}
	for _, channel := range r.channels {
		if _, durable := channel.(durableChannel); !durable {
			continue
		}
		for _, event := range prepared {
			if err := channel.Deliver(ctx, event); err != nil {
				return fmt.Errorf("enqueue durable notification channel %s: %w", channel.Name(), err)
			}
		}
	}

	dispatchCtx := tenant.ContextWithoutAfterCommitHooks(tenant.ContextWithoutTransaction(ctx))
	tenant.RegisterAfterCommit(ctx, func() {
		r.deliverBatch(dispatchCtx, prepared)
	})
	return nil
}

// NotifyDurably has the same validation, tenant gates, and after-commit
// delivery as Notify. Its receipt is limited to durable channel intents, so a
// producer with no other delivery channel can roll back its claim when no push
// subscription accepted the event.
func (r *router) NotifyDurably(ctx context.Context, event Event) (int, error) {
	if err := validate(event); err != nil {
		return 0, err
	}
	if event.Priority == "" {
		event.Priority = PriorityNormal
	}
	if err := r.requireDispatch(ctx, event.Audience.TenantID, event.Type != TypeTest); err != nil {
		return 0, err
	}

	accepted := 0
	for _, channel := range r.channels {
		if receipt, ok := channel.(durableReceiptChannel); ok {
			count, err := receipt.DeliverDurably(ctx, event)
			if err != nil {
				return accepted, fmt.Errorf("enqueue durable notification channel %s: %w", channel.Name(), err)
			}
			accepted += count
			continue
		}
		if _, durable := channel.(durableChannel); !durable {
			continue
		}
		if err := channel.Deliver(ctx, event); err != nil {
			return accepted, fmt.Errorf("enqueue durable notification channel %s: %w", channel.Name(), err)
		}
	}

	dispatchCtx := tenant.ContextWithoutAfterCommitHooks(tenant.ContextWithoutTransaction(ctx))
	event.Data = maps.Clone(event.Data)
	event.Audience.GuardianAccountIDs = slices.Clone(event.Audience.GuardianAccountIDs)
	event.Audience.StaffAccountIDs = slices.Clone(event.Audience.StaffAccountIDs)
	event.Audience.StudentIDs = slices.Clone(event.Audience.StudentIDs)
	tenant.RegisterAfterCommit(ctx, func() {
		r.deliver(dispatchCtx, event)
	})
	return accepted, nil
}
