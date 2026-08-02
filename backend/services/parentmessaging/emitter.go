package parentmessaging

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/notifications"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
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
	// ActorKind is the side that TRIGGERED the event —
	// usersModels.ParentMessageSenderGuardian for parent actions (submit,
	// withdraw, self-service) or usersModels.ParentMessageSenderStaff for staff
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
}

// isTerminalRequestEvent reports whether ev CLOSES a change request that already
// carries a request_created pill — a request_status event referencing the request
// row (ref_table + ref_id). These are the only events allowed through the emitter
// while messaging is disabled (or while its submitting guardian has lost access):
// without the closing pill the staff timeline keeps showing a request the review
// queue has already resolved (see EmitChildEvent's reconcile path). request_created
// pills and self-service mirrors (sick_note, care_exception*) carry no such
// reconcile obligation, so they stay dropped.
func isTerminalRequestEvent(ev ChildEvent) bool {
	return ev.EventType == usersModels.ParentMessageEventRequestStatus && ev.RefTable != "" && ev.RefID != nil
}

// Emitter appends notification pills to parent-OGS threads on behalf of
// OUTSIDE services (schedule change requests, master-data review, parent
// self-service writes). It is deliberately best-effort and transactionally
// detached: EmitChildEvent must be called AFTER the originating action has
// committed (from a tenant.RegisterAfterCommit callback) and opens its OWN
// tenant transaction on a background context, so a pill failure can never
// roll back — or be rolled back by — the action it mirrors.
type Emitter struct {
	db          *bun.DB
	threadRepo  usersModels.ParentMessageThreadRepository
	messageRepo usersModels.ParentMessageRepository
	settings    TenantSettingsResolver
	broadcaster realtime.Broadcaster
	logger      *slog.Logger

	// notifier and preferences push a staff DECISION to the submitting
	// guardian's devices (#1671). Optional and set through
	// WithDecisionNotifications, so every existing construction site keeps
	// working and a partially-wired test emitter simply does not push.
	notifier    notifications.Service
	preferences notifications.PreferenceService
}

// WithDecisionNotifications adds the guardian push for decided requests. It is a
// separate setter rather than two more NewEmitter parameters because the emitter
// is constructed in several places (including tests) that have no business
// knowing about notifications.
func (e *Emitter) WithDecisionNotifications(notifier notifications.Service, preferences notifications.PreferenceService) *Emitter {
	if e == nil {
		return nil
	}
	e.notifier = notifier
	e.preferences = preferences
	return e
}

// NewEmitter wires an emitter. Any nil dependency (except broadcaster/logger)
// turns EmitChildEvent into a no-op, so partially-wired test factories stay
// safe.
func NewEmitter(
	db *bun.DB,
	threadRepo usersModels.ParentMessageThreadRepository,
	messageRepo usersModels.ParentMessageRepository,
	settings TenantSettingsResolver,
	broadcaster realtime.Broadcaster,
	logger *slog.Logger,
) *Emitter {
	return &Emitter{
		db:          db,
		threadRepo:  threadRepo,
		messageRepo: messageRepo,
		settings:    settings,
		broadcaster: broadcaster,
		logger:      logger,
	}
}

// EmitChildEvent appends one pill to the (student, guardian) thread and fires
// the parent-message SSE wake-up. guardianAccountID selects the thread — for
// staff decisions it is the request's SUBMITTING guardian, not the acting
// staffer.
//
// Gating fails CLOSED on a settings-resolve error, unlike the read-path
// MessagingEnabled helpers: this is a WRITE that creates thread rows via
// GetOrCreate, and a school that disabled messaging must not accumulate
// threads because of a transient settings blip. A skipped pill costs one
// notification; a wrongly-created thread is permanent. The lone exception is a
// terminal request event that CLOSES an already-emitted request_created pill —
// see the reconcile path below, which appends the closing pill to the existing
// thread without ever creating one (and, when the submitting guardian has since
// lost access, without waking that guardian).
func (e *Emitter) EmitChildEvent(tenantID, studentID, guardianAccountID int64, ev ChildEvent) {
	if e == nil || e.db == nil || e.threadRepo == nil || e.messageRepo == nil || e.settings == nil {
		return
	}
	if tenantID <= 0 || studentID <= 0 || guardianAccountID <= 0 {
		return
	}
	// Detached background context: the pill outlives the (already committed)
	// request transaction and must not inherit its cancellation.
	bgCtx := context.Background()
	enabled, err := e.settings.ResolveBoolForTenant(bgCtx, tenantID, configModels.KeyParentNotesEnabled)
	// Messaging OFF (or a settings-resolve blip, which fails CLOSED for this
	// write path) normally drops the pill entirely so a disabled school never
	// accumulates threads via GetOrCreate. The ONE exception is a TERMINAL
	// request event (request_status) that CLOSES a request whose request_created
	// pill was already emitted while messaging was on: the review queue clears on
	// the backend, but the staff thread timeline decides "is this request still
	// actionable?" by looking for a later status pill with the same ref_id — so
	// without the closing pill it shows a request the queue has already resolved
	// forever. The reconcile path below writes that closing pill against the
	// EXISTING thread + created pill only (never GetOrCreate, never resurrect), so
	// the no-orphan-thread invariant still holds. That same reconcile path also
	// covers a terminal event whose submitting guardian LOST access afterwards (see
	// the revoked branch inside the tx): the close is still written, but the
	// guardian SSE fan-out is dropped.
	messagingOff := err != nil || !enabled
	isTerminal := isTerminalRequestEvent(ev)
	if messagingOff && !isTerminal {
		return
	}

	var threadID int64
	var suppressReason string
	skipGuardianBroadcast := false
	err = tenant.WithTenantTx(bgCtx, e.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		// Staff-triggered pills (every staff decision, and any close we reconcile
		// onto an existing thread) must never resurrect a thread for — or wake — a
		// guardian whose child link (or parent_portal.access) was revoked after the
		// request was submitted: GetOrCreate would leave a permanent orphan thread
		// and the broadcast would push parent-message activity to an account the
		// parent APIs now hide. Guardian-triggered pills (submit / withdraw) are the
		// guardian acting live under their own permission, so they are not gated.
		revoked := false
		if ev.ActorKind == usersModels.ParentMessageSenderStaff {
			hasAccess, herr := e.guardianHasChildAccess(txCtx, studentID, guardianAccountID)
			if herr != nil {
				return herr
			}
			revoked = !hasAccess
		}
		// Reconcile (append to an EXISTING thread only, never GetOrCreate) whenever a
		// thread must not be born: messaging is disabled (no threads for a disabled
		// school) OR the guardian lost access (no orphan thread). A terminal request
		// event still needs its closing pill in either case so the staff timeline
		// stops showing a resolved request as actionable; a revoked guardian just
		// won't be woken for it (skipGuardianBroadcast below).
		reconcileOnly := messagingOff || revoked

		var thread *usersModels.ParentMessageThread
		if reconcileOnly {
			if !isTerminal {
				// Non-terminal pills (request_created, self-service mirrors) carry no
				// reconcile obligation: with messaging off they were already dropped
				// pre-tx, and for a revoked guardian there is no open request to close.
				suppressReason = "non-terminal pill dropped (messaging disabled or guardian access revoked)"
				return nil
			}
			// Only CLOSE an already-emitted request. Resolve the existing thread and
			// its request_created pill; if either is absent there is no stale "open"
			// notice to reconcile, so drop without creating a thread or leaving a
			// dangling status pill.
			existing, ferr := e.threadRepo.FindByStudentGuardian(txCtx, studentID, guardianAccountID)
			if ferr != nil {
				return ferr
			}
			if existing == nil || ev.RefID == nil {
				suppressReason = "no existing thread to reconcile"
				return nil
			}
			created, cerr := e.messageRepo.FindEventByRef(txCtx, existing.ID, usersModels.ParentMessageEventRequestCreated, ev.RefTable, *ev.RefID)
			if cerr != nil {
				return cerr
			}
			if created == nil {
				suppressReason = "no request_created pill to reconcile"
				return nil
			}
			thread = existing
			// A revoked guardian must not receive the parent-message SSE for a child
			// the parent APIs now hide; the tenant-wide staff wake still fires so the
			// staff inbox drops the stale "Anfrage bearbeiten" CTA.
			skipGuardianBroadcast = revoked
		} else {
			created, cerr := e.threadRepo.GetOrCreate(txCtx, tenantID, studentID, guardianAccountID)
			if cerr != nil {
				return cerr
			}
			thread = created
		}
		threadID = thread.ID
		msg := &usersModels.ParentMessage{
			ThreadID:        thread.ID,
			StudentID:       thread.StudentID,
			SenderAccountID: ev.ActorAccountID,
			SenderKind:      usersModels.ParentMessageSenderSystem,
			SenderName:      "System",
			Body:            ev.Body,
			Kind:            usersModels.ParentMessageKindEvent,
			EventType:       ev.EventType,
			EventActorKind:  ev.ActorKind,
			RequestType:     ev.RequestType,
			RequestStatus:   ev.RequestStatus,
			DecisionReason:  ev.DecisionReason,
			RefTable:        ev.RefTable,
			RefID:           ev.RefID,
		}
		msg.SetTenantID(thread.TenantID)
		if err := e.messageRepo.Create(txCtx, msg); err != nil {
			return err
		}
		// request_created pills are a QUEUE signal, not a chat message (#1803): keep
		// them out of the thread's denormalized last-activity preview/ordering
		// (last_message_*) exactly as counterpartUnread keeps them out of every unread
		// count. Otherwise the two disagree — the Nachrichten badge (counterpart-side,
		// non-request_created) flags an earlier staff decision while the preview shows
		// the parent's OWN newest "Anfrage gestellt", which reads as "my own submission
		// is unread". The pill still lives in the timeline (ListByThread) and still
		// fires the SSE wake below; only the preview/sort skips it. A brand-new thread
		// whose only pill is request_created keeps NULL last_message_at and renders the
		// empty-preview fallback (never a broken row). Every OTHER pill (decision,
		// withdrawal, self-service mirror) advances the preview as before.
		if ev.EventType == usersModels.ParentMessageEventRequestCreated {
			return nil
		}
		// Touch the thread with the row's own DB-stamped created_at (one clock —
		// see AppendMessage) and attribute it to the TRIGGERING side so the staff
		// inbox's awaiting-reply signal and the guardian's unread badge both fire
		// correctly, including for dual-role accounts.
		return e.threadRepo.TouchLastMessage(txCtx, thread.ID, msg.CreatedAt, msg.ID, ev.ActorKind, ev.Body)
	})
	if err != nil {
		loggerOr(e.logger).Warn("parent messaging: child event emit failed",
			slog.Int64("tenant_id", tenantID),
			slog.Int64("student_id", studentID),
			slog.String("event_type", ev.EventType),
			slog.String("error", err.Error()),
		)
		return
	}
	if suppressReason != "" {
		loggerOr(e.logger).Info("parent messaging: child event pill suppressed",
			slog.Int64("tenant_id", tenantID),
			slog.Int64("student_id", studentID),
			slog.String("event_type", ev.EventType),
			slog.String("reason", suppressReason),
		)
		return
	}
	// The staff inbox and open threads refetch only on a parent_message SSE
	// event — not student_updated — so without this the pill stays invisible
	// until a manual reload. When the guardian's access was revoked we still wake
	// the tenant's staff (so their timeline drops the resolved request's CTA) but
	// pass account id 0 to skip the guardian fan-out — a parent who can no longer
	// read the child never gets the SSE. Guardian account ids are always > 0, so
	// id 0 matches no guardian client while BroadcastParentMessage's tenant-wide
	// staff wake still fires.
	broadcastGuardianID := guardianAccountID
	if skipGuardianBroadcast {
		broadcastGuardianID = 0
	}
	Broadcast(e.broadcaster, e.logger, tenantID, broadcastGuardianID, threadID, studentID)

	// A staff decision is the one pill a parent should hear about away from the
	// app (#1671). Deliberately placed next to the SSE wake and under the same
	// condition, so pill, wake and push always agree on who was reached: a
	// guardian whose access was revoked gets none of the three, and a school with
	// messaging off gets none either — the decision pill is that school's
	// in-app channel too, and pushing about something the app does not show
	// would be a dead end. The push additionally re-reads the access itself (see
	// notifyRequestDecision): it runs in a transaction of its own, so it must not
	// inherit a verdict from one that has already committed. That can only make
	// the push narrower than the pill, never wider.
	if !messagingOff && !skipGuardianBroadcast {
		e.notifyRequestDecision(tenantID, studentID, guardianAccountID, ev)
	}
}

// isStaffDecisionPill reports whether a pill is the OGS deciding a parent's
// request — the only pill worth interrupting a parent's day for.
//
// It deliberately excludes the two neighbouring shapes: a guardian withdrawing
// their own request (they just did it) and an "offen" status (that is the
// submission itself, which the parent also just made).
func isStaffDecisionPill(ev ChildEvent) bool {
	if ev.EventType != usersModels.ParentMessageEventRequestStatus {
		return false
	}
	if ev.ActorKind != usersModels.ParentMessageSenderStaff {
		return false
	}
	switch ev.RequestStatus {
	case usersModels.ParentMessageRequestStatusDone, usersModels.ParentMessageRequestStatusRejected:
		return true
	default:
		return false
	}
}

// requestDecisionCopy returns the display-safe German title and body for a
// decided request. It names the subject area but never the child: the payload
// is rendered on a lock screen, and which child it concerns is behind the
// authenticated deep link.
func requestDecisionCopy(requestType, requestStatus string) (title, body string) {
	subject := "Ihre Anfrage"
	switch requestType {
	case usersModels.ParentMessageRequestCareSchedule:
		subject = "Ihre Anfrage zu den Betreuungszeiten"
	case usersModels.ParentMessageRequestMasterData:
		subject = "Ihre Anfrage zu den Stammdaten"
	case usersModels.ParentMessageRequestExcusedAbsence:
		subject = "Ihre Abmeldung"
	}
	if requestStatus == usersModels.ParentMessageRequestStatusRejected {
		return "Anfrage abgelehnt", subject + " wurde abgelehnt."
	}
	return "Anfrage genehmigt", subject + " wurde genehmigt."
}

// notifyRequestDecision pushes a staff decision to the submitting guardian.
//
// Only staff-authored terminal request pills qualify: a guardian withdrawing
// their own request is not news to them, and an "offen" pill is the submission
// itself. Fire-and-forget on a detached background context, like everything else
// past the emitter's own commit.
func (e *Emitter) notifyRequestDecision(tenantID, studentID, guardianAccountID int64, ev ChildEvent) {
	if e.notifier == nil || e.preferences == nil || e.db == nil || e.threadRepo == nil {
		return
	}
	if !isStaffDecisionPill(ev) {
		return
	}

	// The access recheck, the consent read and the delivery are RLS-scoped, so
	// they need a tenant transaction of their own — the pill's transaction
	// committed before this call.
	bgCtx := context.Background()
	err := tenant.WithTenantTx(bgCtx, e.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		// Authorization first, and read here rather than carried over from the
		// pill's transaction: a push payload is rendered on a lock screen, so the
		// question "may this account still hear about this child?" is answered
		// against the students_guardians row this transaction sees, not against a
		// verdict from a snapshot that is already history. Consent is the second
		// gate and never a substitute for the first.
		hasAccess, aerr := e.guardianHasChildAccess(txCtx, studentID, guardianAccountID)
		if aerr != nil {
			return aerr
		}
		if !hasAccess {
			return nil
		}
		optedIn, ferr := e.preferences.FilterOptedIn(txCtx, notifications.TypeParentRequestDecided, []int64{guardianAccountID})
		if ferr != nil {
			return ferr
		}
		if len(optedIn) == 0 {
			return nil
		}
		title, body := requestDecisionCopy(ev.RequestType, ev.RequestStatus)
		return e.notifier.Notify(txCtx, notifications.Event{
			Type:     notifications.TypeParentRequestDecided,
			Title:    title,
			Body:     body,
			DeepLink: fmt.Sprintf("/children/%d", studentID),
			Priority: notifications.PriorityNormal,
			Audience: notifications.Audience{
				TenantID:           tenantID,
				Scope:              notifications.ScopeGuardian,
				GuardianAccountIDs: optedIn,
			},
		})
	})
	switch {
	case errors.Is(err, notifications.ErrDisabled), errors.Is(err, notifications.ErrOutsideActiveWindow):
		return
	case err != nil:
		loggerOr(e.logger).Warn("parent messaging: request decision push failed",
			slog.Int64("tenant_id", tenantID),
			slog.Int64("student_id", studentID),
			slog.String("request_type", ev.RequestType),
			slog.String("error", err.Error()),
		)
	}
}

// BroadcastChildUpdateToGuardians wakes EVERY guardian of the child with a
// message-INDEPENDENT SSE invalidation (EventParentChildUpdated) so an already-
// open parents-app tab refetches the child's care state in real time — no matter
// which guardian acted and no matter whether parent messaging is enabled. Unlike
// EmitChildEvent it creates NO thread and NO pill and never touches the messaging
// setting; it is a pure wake over the parent-message SSE fan-out. The guardian set
// is the same parent_portal.access-scoped list the messaging thread fan-out uses,
// so a guardian who lost access is not woken. Fire-and-forget: schedule it AFTER
// the originating transaction commits (from a RegisterAfterCommit callback) so a
// woken client never reads the pre-commit snapshot. A nil dependency is a no-op.
func (e *Emitter) BroadcastChildUpdateToGuardians(tenantID, studentID int64) {
	if e == nil || e.broadcaster == nil || e.threadRepo == nil || e.db == nil {
		return
	}
	if tenantID <= 0 || studentID <= 0 {
		return
	}
	// Detached background context: this outlives the (already committed)
	// originating transaction. Reading the guardian list is RLS-scoped, so it runs
	// inside its own tenant transaction like EmitChildEvent's work.
	bgCtx := context.Background()
	var guardians []*usersModels.MessageableGuardian
	if err := tenant.WithTenantTx(bgCtx, e.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		gs, gerr := e.threadRepo.ListGuardiansForStudent(txCtx, studentID)
		if gerr != nil {
			return gerr
		}
		guardians = gs
		return nil
	}); err != nil {
		loggerOr(e.logger).Warn("parent messaging: guardian fan-out lookup failed",
			slog.Int64("tenant_id", tenantID),
			slog.Int64("student_id", studentID),
			slog.String("error", err.Error()),
		)
		return
	}
	for _, g := range guardians {
		if g == nil || g.AccountID <= 0 {
			continue
		}
		event := realtime.NewParentChildUpdatedEvent(g.AccountID, studentID)
		// Guardian-only: parent_child_updated is a pure care-state invalidation for
		// the family. Routing it via BroadcastParentMessage would also push a
		// sanitized copy to EVERY staff client — once per guardian — flooding staff
		// channels with events they neither listen for nor need (the staff queue is
		// woken separately via change_requests_changed). BroadcastToGuardian delivers
		// to this guardian's own tabs and no one else (#1845 review).
		if err := e.broadcaster.BroadcastToGuardian(tenantID, g.AccountID, event); err != nil {
			loggerOr(e.logger).Warn("parent messaging: guardian child-update broadcast failed",
				slog.Int64("tenant_id", tenantID),
				slog.Int64("student_id", studentID),
				slog.Int64("guardian_account_id", g.AccountID),
				slog.String("error", err.Error()),
			)
		}
	}
}

// guardianHasChildAccess reports whether guardianAccountID is STILL a linked
// guardian of the child with parent_portal.access — the identical JSONB
// containment / active-account-tenant filter the parent read paths and the
// chat's requireLinkedGuardian gate apply, so a staff write never targets an
// account the parent APIs already hide. Must run inside a tenant transaction
// (it is RLS-scoped via the ambient tenant tx).
func (e *Emitter) guardianHasChildAccess(ctx context.Context, studentID, guardianAccountID int64) (bool, error) {
	guardians, err := e.threadRepo.ListGuardiansForStudent(ctx, studentID)
	if err != nil {
		return false, err
	}
	for _, g := range guardians {
		if g != nil && g.AccountID == guardianAccountID {
			return true, nil
		}
	}
	return false, nil
}

// MessagingEnabledForTenant reports whether parent-OGS messaging
// (operations.parent_notes_enabled) is ON for the tenant, so a sibling service
// can refuse to APPLY a change request whose only notification channel — the
// decision pill this emitter drops while messaging is off — would silently go
// nowhere. It mirrors the read-path fail-OPEN direction (a transient
// settings-resolve blip counts as enabled), the same contract the chat's
// requireEnabled gate used before this flow was decoupled. A nil emitter or nil
// settings resolver (partially-wired test emitter) also counts as enabled, so
// the gate never blocks a flow whose emitter carries no settings dependency.
func (e *Emitter) MessagingEnabledForTenant(ctx context.Context, tenantID int64) bool {
	if e == nil || e.settings == nil {
		return true
	}
	return MessagingEnabledForTenant(ctx, e.settings, tenantID, e.logger)
}

// GuardianHasChildAccess exposes the linked-guardian / parent_portal.access
// check to sibling services that must decide whether to APPLY (and notify a
// parent about) a change request — the care-schedule request flow refuses to
// approve a request whose submitting guardian has since lost access. It runs on
// the CALLER's context, so the caller's ambient tenant transaction and RLS
// scope apply; call it from inside a tenant transaction. Returns an error when
// the emitter is not wired with a thread repository.
func (e *Emitter) GuardianHasChildAccess(ctx context.Context, studentID, guardianAccountID int64) (bool, error) {
	if e == nil || e.threadRepo == nil {
		return false, errors.New("parentmessaging: emitter not configured for guardian access check")
	}
	return e.guardianHasChildAccess(ctx, studentID, guardianAccountID)
}
