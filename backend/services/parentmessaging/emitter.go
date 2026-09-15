package parentmessaging

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
)

// ChildEvent describes one notification pill appended to a child's parent-OGS
// thread: a request lifecycle notice (created / decided / withdrawn) or a
// self-service action mirror (sick note, one-day pickup change). Pills are
// pure timeline entries — never interactive; the referenced record (RefTable /
// RefID) is the source of truth.
type ChildEvent struct {
	// EventType discriminates the pill for localized rendering:
	// "request_created" | "request_status" | "sick_note" | "care_exception" |
	// "care_exception_correction".
	EventType string
	// ActorKind is the side that TRIGGERED the event — ActorGuardian for parent
	// actions (submit, withdraw, self-service) or ActorStaff for staff
	// decisions. It stamps EventActorKind (so the other side's unread badge
	// fires even for dual-role accounts) and the thread's last-sender side.
	ActorKind string
	// ActorAccountID is the acting account (guardian or staff).
	ActorAccountID int64
	// Body is the German pill text (the staff portal renders it directly; the
	// parents portal localizes from the structured fields below).
	Body string
	// RequestType / RequestStatus / DecisionReason carry the structured request
	// outcome for localized clients. RequestStatus uses the parent_messages
	// vocabulary (offen/erledigt/abgelehnt/zurueckgezogen); callers map their
	// domain statuses at the emit boundary. Empty for non-request events.
	RequestType    string
	RequestStatus  string
	DecisionReason string
	// RefTable / RefID point at the underlying record (e.g. the change-request
	// row) so a future client can deep-link. Optional.
	RefTable string
	RefID    *int64
	// Payload carries the structured detail a localized client needs to render
	// the pill itself instead of the German Body — today the confirmed
	// effective date of a decided offering-change request (#2484). Optional.
	Payload map[string]any
}

// IsTerminalRequestEvent reports whether ev CLOSES a change request that
// already carries a request_created pill — a request_status event referencing
// the request row (ref_table + ref_id). These are the only events allowed
// through the emitter while messaging is disabled (or while its submitting
// guardian has lost access): without the closing pill the staff timeline keeps
// showing a request the review queue has already resolved. request_created
// pills and self-service mirrors (sick_note, care_exception*) carry no such
// reconcile obligation, so they stay dropped.
func IsTerminalRequestEvent(ev ChildEvent) bool {
	return ev.EventType == EventRequestStatus && ev.RefTable != "" && ev.RefID != nil
}

// PortalGuardian is one guardian of a child who holds parent_portal.access
// and a linked account, with the locale their notifications are rendered in.
type PortalGuardian struct {
	AccountID    int64
	PortalLocale string
}

// Events is the Communication-owned side of the emitter: the thread and pill
// writes with their detached tenant transaction, the guardian list behind the
// access checks, the settings gate, and the realtime fan-out. It is a
// consumer-owned port; Communication supplies the implementation at the
// composition seam.
type Events interface {
	// EmitChildEvent appends one pill to the (student, guardian) thread in a
	// detached tenant transaction and fires the parent-message wake-up.
	EmitChildEvent(tenantID, studentID, guardianAccountID int64, ev ChildEvent)
	// WakeChildGuardians wakes every portal guardian of the child with a
	// message-independent care-state invalidation.
	WakeChildGuardians(tenantID, studentID int64)
	// PortalGuardians lists the child's portal guardians on the caller's
	// context, so the caller's tenant transaction and RLS scope apply.
	PortalGuardians(ctx context.Context, studentID int64) ([]PortalGuardian, error)
	// InTransaction reports whether ctx carries an active tenant transaction.
	InTransaction(ctx context.Context) bool
	// MessagingEnabledForTenant applies the read-path fail-OPEN gate.
	MessagingEnabledForTenant(ctx context.Context, tenantID int64) bool
}

// Emitter coordinates parent request notifications for services outside the
// messaging domain. EnqueueRequestDecision writes the durable device intent in
// the caller's transaction. EmitChildEvent appends the best-effort chat pill in
// a detached transaction after commit. Every method is nil-safe, so a
// partially-wired test service can hold a nil emitter.
type Emitter struct {
	events Events

	// notifier and preferences enqueue a staff decision for the submitting
	// guardian's devices in the decision transaction (#1671).
	notifier    notifications.Service
	preferences notifications.PreferenceService
}

// NewEmitter wraps Communication's event implementation. A nil events port
// turns every operation into a no-op, so partially-wired test factories stay
// safe.
func NewEmitter(events Events) *Emitter {
	return &Emitter{events: events}
}

// WithDecisionNotifications adds durable guardian delivery for decided requests.
func (e *Emitter) WithDecisionNotifications(notifier notifications.Service, preferences notifications.PreferenceService) *Emitter {
	if e == nil {
		return nil
	}
	e.notifier = notifier
	e.preferences = preferences
	return e
}

// EmitChildEvent appends one pill to the (student, guardian) thread and fires
// the parent-message SSE wake-up. guardianAccountID selects the thread — for
// staff decisions it is the request's SUBMITTING guardian, not the acting
// staffer. The gating and reconcile rules are Communication's; see the owner
// implementation.
func (e *Emitter) EmitChildEvent(tenantID, studentID, guardianAccountID int64, ev ChildEvent) {
	if e == nil || e.events == nil {
		return
	}
	if tenantID <= 0 || studentID <= 0 || guardianAccountID <= 0 {
		return
	}
	e.events.EmitChildEvent(tenantID, studentID, guardianAccountID, ev)
}

// isStaffDecisionPill reports whether a pill is the OGS deciding a parent's
// request — the only pill worth interrupting a parent's day for.
//
// It deliberately excludes the two neighbouring shapes: a guardian withdrawing
// their own request (they just did it) and an "offen" status (that is the
// submission itself, which the parent also just made).
func isStaffDecisionPill(ev ChildEvent) bool {
	if ev.EventType != EventRequestStatus {
		return false
	}
	if ev.ActorKind != ActorStaff {
		return false
	}
	switch ev.RequestStatus {
	case RequestStatusDone, RequestStatusRejected:
		return true
	default:
		return false
	}
}

// requestDecisionCopy returns the display-safe localized title and body for a
// decided request. It names the subject area but never the child: the payload
// is rendered on a lock screen, and which child it concerns is behind the
// authenticated deep link.
func requestDecisionCopy(locale, requestType, requestStatus string) (title, body string) {
	return notifications.ParentRequestDecisionCopy(locale, requestType, requestStatus)
}

// EnqueueRequestDecision records a staff decision delivery for the submitting
// guardian in the caller's domain transaction.
//
// Only staff-authored terminal request pills qualify: a guardian withdrawing
// their own request is not news to them, and an "offen" pill is the submission
// itself. The transaction requirement prevents a request decision from
// committing without its durable delivery intent.
func (e *Emitter) EnqueueRequestDecision(ctx context.Context, tenantID, studentID, guardianAccountID int64, ev ChildEvent) error {
	if e == nil || e.notifier == nil || e.preferences == nil || e.events == nil {
		return nil
	}
	if !isStaffDecisionPill(ev) {
		return nil
	}
	if ev.RefID == nil {
		return nil
	}
	if !e.events.InTransaction(ctx) {
		return errors.New("parentmessaging: request decision enqueue requires an active tenant transaction")
	}
	refID := *ev.RefID

	// Authorization first: a push payload is rendered on a lock screen, so
	// consent is never a substitute for current child access.
	guardians, err := e.events.PortalGuardians(ctx, studentID)
	if err != nil {
		return err
	}
	locale := ""
	hasAccess := false
	for _, guardian := range guardians {
		if guardian.AccountID == guardianAccountID {
			hasAccess = true
			locale = guardian.PortalLocale
			break
		}
	}
	if !hasAccess {
		return nil
	}
	optedIn, err := e.preferences.FilterOptedIn(ctx, notifications.TypeParentRequestDecided, []int64{guardianAccountID})
	if err != nil {
		return err
	}
	if len(optedIn) == 0 {
		return nil
	}
	title, body := requestDecisionCopy(locale, ev.RequestType, ev.RequestStatus)
	err = e.notifier.Notify(ctx, notifications.Event{
		Type:           notifications.TypeParentRequestDecided,
		IdempotencyKey: fmt.Sprintf("parent-request-decision:%s:%d:%s", ev.RefTable, refID, ev.RequestStatus),
		RelatedType:    ev.RefTable, RelatedID: refID,
		Title:    title,
		Body:     body,
		DeepLink: fmt.Sprintf("/children/%d", studentID),
		Priority: notifications.PriorityNormal,
		Audience: notifications.Audience{
			TenantID:           tenantID,
			Scope:              notifications.ScopeGuardian,
			GuardianAccountIDs: optedIn,
			StudentIDs:         []int64{studentID},
		},
	})
	if errors.Is(err, notifications.ErrDisabled) || errors.Is(err, notifications.ErrOutsideActiveWindow) {
		return nil
	}
	return err
}

// BroadcastChildUpdateToGuardians wakes EVERY guardian of the child with a
// message-INDEPENDENT SSE invalidation (EventParentChildUpdated) so an already-
// open parents-app tab refetches the child's care state in real time — no matter
// which guardian acted and no matter whether parent messaging is enabled. Unlike
// EmitChildEvent it creates NO thread and NO pill and never touches the messaging
// setting. Fire-and-forget: schedule it AFTER the originating transaction
// commits (from a RegisterAfterCommit callback) so a woken client never reads
// the pre-commit snapshot. A nil dependency is a no-op.
func (e *Emitter) BroadcastChildUpdateToGuardians(tenantID, studentID int64) {
	if e == nil || e.events == nil {
		return
	}
	if tenantID <= 0 || studentID <= 0 {
		return
	}
	e.events.WakeChildGuardians(tenantID, studentID)
}

// MessagingEnabledForTenant reports whether parent-OGS messaging
// (operations.parent_notes_enabled) is ON for the tenant, so a sibling service
// can refuse to APPLY a change request whose only notification channel — the
// decision pill this emitter drops while messaging is off — would silently go
// nowhere. It mirrors the read-path fail-OPEN direction (a transient
// settings-resolve blip counts as enabled). A nil emitter (partially-wired test
// emitter) also counts as enabled, so the gate never blocks a flow whose
// emitter carries no settings dependency.
func (e *Emitter) MessagingEnabledForTenant(ctx context.Context, tenantID int64) bool {
	if e == nil || e.events == nil {
		return true
	}
	return e.events.MessagingEnabledForTenant(ctx, tenantID)
}

// GuardianHasChildAccess exposes the linked-guardian / parent_portal.access
// check to sibling services that must decide whether to APPLY (and notify a
// parent about) a change request — the care-schedule request flow refuses to
// approve a request whose submitting guardian has since lost access. It runs on
// the CALLER's context, so the caller's ambient tenant transaction and RLS
// scope apply; call it from inside a tenant transaction. Returns an error when
// the emitter is not wired with its Communication side.
func (e *Emitter) GuardianHasChildAccess(ctx context.Context, studentID, guardianAccountID int64) (bool, error) {
	if e == nil || e.events == nil {
		return false, fmt.Errorf("%w: guardian access check", ErrEmitterNotConfigured)
	}
	guardians, err := e.events.PortalGuardians(ctx, studentID)
	if err != nil {
		return false, err
	}
	for _, guardian := range guardians {
		if guardian.AccountID == guardianAccountID {
			return true, nil
		}
	}
	return false, nil
}
