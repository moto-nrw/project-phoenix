package parentmessaging

import (
	"context"
	"log/slog"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
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

// Emitter appends notification pills to parent-OGS threads on behalf of
// OUTSIDE services (schedule change requests, master-data review, parent
// self-service writes). It is deliberately best-effort and transactionally
// detached: EmitChildEvent must be called AFTER the originating action has
// committed (from a tenant.RegisterAfterCommit callback) and opens its OWN
// tenant transaction on a background context, so a pill failure can never
// roll back — or be rolled back by — the action it mirrors.
// eventTypeRequestCreated is the pill appended when a change request is first
// submitted. A staff decision marks exactly this pill read for the deciding
// admin (see markStaffReadUpToRequestPill).
const eventTypeRequestCreated = "request_created"

type Emitter struct {
	db          *bun.DB
	threadRepo  usersModels.ParentMessageThreadRepository
	messageRepo usersModels.ParentMessageRepository
	readRepo    usersModels.ParentMessageReadRepository
	settings    TenantSettingsResolver
	broadcaster realtime.Broadcaster
	logger      *slog.Logger
}

// NewEmitter wires an emitter. Any nil dependency (except readRepo/broadcaster/
// logger) turns EmitChildEvent into a no-op, so partially-wired test factories
// stay safe. A nil readRepo only disables the decide→mark-read step; pills
// still emit.
func NewEmitter(
	db *bun.DB,
	threadRepo usersModels.ParentMessageThreadRepository,
	messageRepo usersModels.ParentMessageRepository,
	readRepo usersModels.ParentMessageReadRepository,
	settings TenantSettingsResolver,
	broadcaster realtime.Broadcaster,
	logger *slog.Logger,
) *Emitter {
	return &Emitter{
		db:          db,
		threadRepo:  threadRepo,
		messageRepo: messageRepo,
		readRepo:    readRepo,
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
// notification; a wrongly-created thread is permanent.
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
	if err != nil || !enabled {
		return
	}

	var threadID int64
	err = tenant.WithTenantTx(bgCtx, e.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		thread, err := e.threadRepo.GetOrCreate(txCtx, tenantID, studentID, guardianAccountID)
		if err != nil {
			return err
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
	// A staff decision clears the deciding admin's unread badge for this thread:
	// deciding happens on the Änderungsanfragen page, not by opening the chat, so
	// without this the "Anfrage gestellt" pill would stay unread and keep the
	// Nachrichten badge lit. Marks read only UP TO the request-created pill, so a
	// parent chat message sent after the request stays unread. Runs before the
	// broadcast so the staff inbox refetch sees the advanced cursor. Best-effort.
	e.markStaffReadUpToRequestPill(bgCtx, tenantID, threadID, ev)

	// The staff inbox and open threads refetch only on a parent_message SSE
	// event — not student_updated — so without this the pill stays invisible
	// until a manual reload.
	Broadcast(e.broadcaster, e.logger, tenantID, guardianAccountID, threadID, studentID)
}

// markStaffReadUpToRequestPill advances the deciding admin's read cursor to the
// change request's "request created" pill after a staff decision, so the
// Nachrichten badge clears without opening the thread. No-op unless the event is
// a staff-triggered request decision with a resolvable reference. Best-effort:
// a failure here never affects the already-emitted decision pill.
func (e *Emitter) markStaffReadUpToRequestPill(bgCtx context.Context, tenantID, threadID int64, ev ChildEvent) {
	if e.readRepo == nil || threadID <= 0 {
		return
	}
	if ev.EventType != "request_status" ||
		ev.ActorKind != usersModels.ParentMessageSenderStaff ||
		ev.ActorAccountID <= 0 || ev.RefID == nil || ev.RefTable == "" {
		return
	}
	err := tenant.WithTenantTx(bgCtx, e.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		pill, err := e.messageRepo.FindEventByRef(txCtx, threadID, eventTypeRequestCreated, ev.RefTable, *ev.RefID)
		if err != nil {
			return err
		}
		if pill == nil {
			return nil
		}
		_, err = e.readRepo.MarkReadUpTo(txCtx, tenantID, threadID, ev.ActorAccountID, pill.CreatedAt, pill.ID)
		return err
	})
	if err != nil {
		loggerOr(e.logger).Warn("parent messaging: mark request pill read failed",
			slog.Int64("tenant_id", tenantID),
			slog.Int64("thread_id", threadID),
			slog.Int64("account_id", ev.ActorAccountID),
			slog.String("error", err.Error()),
		)
	}
}
