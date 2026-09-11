package messaging

import (
	"context"
	"log/slog"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// EventEmitter is Communication's side of the parent-event emitter: it owns
// the thread and pill writes, the detached tenant transaction they run in, and
// the realtime fan-out. Consumers reach it through the services/parentmessaging
// shell, which adds the pure audience and decision rules on top.
type EventEmitter struct {
	db          *bun.DB
	runtime     tenant.UnitOfWork
	threadRepo  usersModels.ParentMessageThreadRepository
	messageRepo usersModels.ParentMessageRepository
	settings    parentmessaging.TenantSettingsResolver
	broadcaster realtime.Broadcaster
	logger      *slog.Logger
}

// NewEventEmitter wires the owner-side emitter. runtime is the unit of work
// the detached pill and wake paths carry on their background context; a zero
// runtime (partially-wired test factories) makes those paths log and skip.
// Any nil dependency (except broadcaster/logger) turns EmitChildEvent into a
// no-op for the same reason.
func NewEventEmitter(
	db *bun.DB,
	runtime tenant.UnitOfWork,
	threadRepo usersModels.ParentMessageThreadRepository,
	messageRepo usersModels.ParentMessageRepository,
	settings parentmessaging.TenantSettingsResolver,
	broadcaster realtime.Broadcaster,
	logger *slog.Logger,
) *EventEmitter {
	return &EventEmitter{
		db: db, runtime: runtime, threadRepo: threadRepo, messageRepo: messageRepo,
		settings: settings, broadcaster: broadcaster, logger: logger,
	}
}

// backgroundContext is the detached context the pill and wake paths run on:
// it outlives the (already committed) request transaction and must not
// inherit its cancellation, but it does carry the unit of work those paths
// open their own tenant transaction through.
func (e *EventEmitter) backgroundContext() context.Context {
	return tenant.WithUnitOfWork(context.Background(), e.runtime)
}

// InTransaction reports whether ctx carries an active tenant transaction.
func (e *EventEmitter) InTransaction(ctx context.Context) bool {
	_, active := tenant.TransactionFromContext(ctx)
	return active
}

// PortalGuardians lists the child's guardians with parent_portal.access on
// the caller's context, so the caller's tenant transaction and RLS scope
// apply.
func (e *EventEmitter) PortalGuardians(ctx context.Context, studentID int64) ([]parentmessaging.PortalGuardian, error) {
	if e == nil || e.threadRepo == nil {
		return nil, parentmessaging.ErrEmitterNotConfigured
	}
	guardians, err := e.threadRepo.ListGuardiansForStudent(ctx, studentID)
	if err != nil {
		return nil, err
	}
	result := make([]parentmessaging.PortalGuardian, 0, len(guardians))
	for _, guardian := range guardians {
		if guardian == nil {
			continue
		}
		result = append(result, parentmessaging.PortalGuardian{AccountID: guardian.AccountID, PortalLocale: guardian.PortalLocale})
	}
	return result, nil
}

// MessagingEnabledForTenant reports whether parent-OGS messaging is ON for the
// tenant with the read-path fail-OPEN direction. A nil emitter or nil settings
// resolver (partially-wired test emitter) also counts as enabled, so the gate
// never blocks a flow whose emitter carries no settings dependency.
func (e *EventEmitter) MessagingEnabledForTenant(ctx context.Context, tenantID int64) bool {
	if e == nil || e.settings == nil {
		return true
	}
	return parentmessaging.MessagingEnabledForTenant(ctx, e.settings, tenantID, e.logger)
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
func (e *EventEmitter) EmitChildEvent(tenantID, studentID, guardianAccountID int64, ev parentmessaging.ChildEvent) {
	if e == nil || e.db == nil || e.threadRepo == nil || e.messageRepo == nil || e.settings == nil {
		return
	}
	if tenantID <= 0 || studentID <= 0 || guardianAccountID <= 0 {
		return
	}
	// Detached background context: the pill outlives the (already committed)
	// request transaction and must not inherit its cancellation.
	bgCtx := e.backgroundContext()
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
	isTerminal := parentmessaging.IsTerminalRequestEvent(ev)
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
			Payload:         ev.Payload,
		}
		msg.SetTenantID(thread.TenantID)
		if err := e.threadRepo.LockForMessageAppend(txCtx, thread.ID); err != nil {
			return err
		}
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
}

// WakeChildGuardians wakes EVERY guardian of the child with a message-
// INDEPENDENT SSE invalidation (EventParentChildUpdated) so an already-open
// parents-app tab refetches the child's care state in real time — no matter
// which guardian acted and no matter whether parent messaging is enabled. Unlike
// EmitChildEvent it creates NO thread and NO pill and never touches the messaging
// setting; it is a pure wake over the parent-message SSE fan-out. The guardian set
// is the same parent_portal.access-scoped list the messaging thread fan-out uses,
// so a guardian who lost access is not woken. Fire-and-forget: schedule it AFTER
// the originating transaction commits (from a RegisterAfterCommit callback) so a
// woken client never reads the pre-commit snapshot. A nil dependency is a no-op.
func (e *EventEmitter) WakeChildGuardians(tenantID, studentID int64) {
	if e == nil || e.broadcaster == nil || e.threadRepo == nil || e.db == nil {
		return
	}
	if tenantID <= 0 || studentID <= 0 {
		return
	}
	// Detached background context: this outlives the (already committed)
	// originating transaction. Reading the guardian list is RLS-scoped, so it runs
	// inside its own tenant transaction like EmitChildEvent's work.
	bgCtx := e.backgroundContext()
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
func (e *EventEmitter) guardianHasChildAccess(ctx context.Context, studentID, guardianAccountID int64) (bool, error) {
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

var _ parentmessaging.Events = (*EventEmitter)(nil)
