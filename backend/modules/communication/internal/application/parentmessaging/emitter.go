package parentmessaging

import (
	"context"
	"errors"
	"fmt"

	eventContract "github.com/moto-nrw/project-phoenix/modules/communication/eventcontract"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

// eventContract.ChildEvent describes one notification pill appended to a child's parent-OGS
// thread: a request lifecycle notice (created / decided / withdrawn) or a
// self-service action mirror (sick note, one-day pickup change). Pills are
// pure timeline entries — never interactive; the referenced record (RefTable /
// RefID) is the source of truth.

// IsTerminalRequestEvent reports whether ev CLOSES a change request that
// already carries a request_created pill — a request_status event referencing
// the request row (ref_table + ref_id). These are the only events allowed
// through the emitter while messaging is disabled (or while its submitting
// guardian has lost access): without the closing pill the staff timeline keeps
// showing a request the review queue has already resolved. request_created
// pills and self-service mirrors (sick_note, care_exception*) carry no such
// reconcile obligation, so they stay dropped.
func IsTerminalRequestEvent(ev eventContract.ChildEvent) bool {
	return ev.EventType == eventContract.EventRequestStatus && ev.RefTable != "" && ev.RefID != nil
}

// eventContract.PortalGuardian is one guardian of a child who holds parent_portal.access
// and a linked account, with the locale their notifications are rendered in.

// eventContract.Events is the Communication-owned side of the emitter: the thread and pill
// writes with their detached tenant transaction, the guardian list behind the
// access checks, the settings gate, and the realtime fan-out. It is a
// consumer-owned port; Communication supplies the implementation at the
// composition seam.

// Emitter coordinates parent request notifications for services outside the
// messaging domain. EnqueueRequestDecision writes the durable device intent in
// the caller's transaction. EmitChildEvent appends the best-effort chat pill in
// a detached transaction after commit. Every method is nil-safe, so a
// partially-wired test service can hold a nil emitter.
type Emitter struct {
	events eventContract.Events

	// notifier enqueues a staff decision for the submitting
	// guardian's devices in the decision transaction (#1671).
	notifier eventContract.DecisionNotifier
}

// NewEmitterWithDecisions wraps Communication's event implementation with the
// durable guardian delivery of decided requests at construction (ADR 0059). A
// nil events port turns every operation into a no-op, so partially-wired test
// factories stay safe; a nil notifier delivers no decisions.
func NewEmitterWithDecisions(events eventContract.Events, notifier eventContract.DecisionNotifier) *Emitter {
	return &Emitter{events: events, notifier: notifier}
}

// EmitChildEvent appends one pill to the (student, guardian) thread and fires
// the parent-message SSE wake-up. guardianAccountID selects the thread — for
// staff decisions it is the request's SUBMITTING guardian, not the acting
// staffer. The gating and reconcile rules are Communication's; see the owner
// implementation.
func (e *Emitter) EmitChildEvent(tenantID, studentID, guardianAccountID int64, ev eventContract.ChildEvent) {
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
func isStaffDecisionPill(ev eventContract.ChildEvent) bool {
	if ev.EventType != eventContract.EventRequestStatus {
		return false
	}
	if ev.ActorKind != eventContract.ActorStaff {
		return false
	}
	switch ev.RequestStatus {
	case eventContract.RequestStatusDone, eventContract.RequestStatusRejected:
		return true
	default:
		return false
	}
}

// eventContract.DecisionNotifier is the Delivery producer of a guardian's request decision.
// The emitter resolves current child access before calling this port; Delivery
// applies notification consent, locale, and the active delivery window.

// EnqueueRequestDecision records a staff decision delivery for the submitting
// guardian in the caller's domain transaction.
//
// Only staff-authored terminal request pills qualify: a guardian withdrawing
// their own request is not news to them, and an "offen" pill is the submission
// itself. The transaction requirement prevents a request decision from
// committing without its durable delivery intent.
func (e *Emitter) EnqueueRequestDecision(ctx context.Context, tenantID, studentID, guardianAccountID int64, ev eventContract.ChildEvent) error {
	if e == nil || e.notifier == nil || e.events == nil {
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
	return e.notifier.NotifyRequestDecision(ctx, tenantID, studentID, guardianAccountID, locale, ev.RefTable, refID, ev.RequestType, ev.RequestStatus)
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

// EmitParentChildEvent is the public parent-authored event capability. The
// retained emitter keeps the richer staff-decision vocabulary until #4074.
func (e *Emitter) EmitParentChildEvent(tenantID, studentID, guardianAccountID int64, event communication.ParentChildEvent) {
	e.EmitChildEvent(tenantID, studentID, guardianAccountID, eventContract.ChildEvent{
		EventType: event.EventType, ActorKind: event.ActorKind, ActorAccountID: event.ActorAccountID,
		Body: event.Body, RequestType: event.RequestType, RequestStatus: event.RequestStatus,
		RefTable: event.RefTable, RefID: event.RefID,
	})
}
