package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

// CareRequestEffectPorts are the owners a care request transition reports
// to. Records and Students are required. A nil effect port skips its effect,
// as in the excused-absence twin: a graph without parent messaging posts no
// pill and checks no guardian.
type CareRequestEffectPorts struct {
	Records     ports.CareRequestRows
	Students    ports.CareRequestStudentNames
	Messenger   ports.RequestMessenger
	Ledger      ports.RequestLedger
	Shares      ports.ShareVisibility
	Broadcaster ports.CareScheduleBroadcaster
	Logger      *slog.Logger
}

// CareRequestEffects tells the families and the staff tabs about a care
// request transition: the ledger line, the chat pills (#3135), the
// co-guardian notices (#2267) and the realtime wake-ups. The ledger line and
// the durable decision intent are written in the ambient transaction, so a
// rolled-back change leaves neither. Chat pills join that transaction; only
// realtime wake-ups and duplicate diagnostic logs run after commit.
type CareRequestEffects struct {
	deps  CareRequestEffectPorts
	hooks ports.TransactionHooks
}

// NewCareRequestEffects binds the effects to their owners and the tenant
// transaction.
func NewCareRequestEffects(deps CareRequestEffectPorts, hooks ports.TransactionHooks) (*CareRequestEffects, error) {
	if deps.Records == nil || deps.Students == nil || hooks == nil {
		return nil, errors.New("care requests: request records, student names, and transaction hooks are required")
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	return &CareRequestEffects{deps: deps, hooks: hooks}, nil
}

// RecordSubmission writes the "submitted" ledger line.
func (e *CareRequestEffects) RecordSubmission(ctx context.Context, req *carerequests.Request) error {
	return e.record(ctx, req, careplan.ParentRequestEventSubmitted, req.SubmittedBy, nil)
}

// NotifySubmission posts the "Anfrage" pill to the submitting guardian.
func (e *CareRequestEffects) NotifySubmission(ctx context.Context, req *carerequests.Request) error {
	event := ports.RequestEvent{
		EventType: careplan.ParentMessageEventRequestCreated, ActorKind: careplan.ParentMessageActorGuardian,
		ActorAccountID: req.SubmittedBy, Body: careRequestCreatedBody,
		RequestType: careRequestPillType(req.RequestKind), RequestStatus: careplan.ParentMessageRequestStatusOpen,
	}
	if req.RequestKind == "pickup_change" {
		event.Body = withPickupDetail(pickupPillBody(pickupRequestCreatedVerb, req), req)
		event.Payload = pickupPillPayload(req)
	}
	return e.emitPill(ctx, req, event)
}

// WakeGuardians fans a message-independent parent_child_updated out to EVERY
// guardian of the child after the ambient transaction commits, so a
// co-guardian with the child open refetches the request and pickup state a
// lifecycle transition changed. The pills only touch the submitting
// guardian's own thread, and staff-side broadcasts never reach the parents
// SSE stream, so without this a second guardian keeps a stale "Anfrage
// offen" badge after a submit, or a stale pickup time after an approve,
// until they refocus or reload (#1725).
func (e *CareRequestEffects) WakeGuardians(ctx context.Context, req *carerequests.Request) {
	if e.deps.Messenger == nil {
		return
	}
	tenantID := e.tenantOf(ctx, req.TenantID)
	studentID := req.StudentID
	e.hooks.AfterCommit(ctx, func() {
		e.deps.Messenger.BroadcastChildUpdateToGuardians(tenantID, studentID)
	})
}

// RecordGuardianEdit writes the "guardian_edited" ledger line.
func (e *CareRequestEffects) RecordGuardianEdit(ctx context.Context, req *carerequests.Request, actorID int64) error {
	return e.record(ctx, req, careplan.ParentRequestEventGuardianEdit, actorID, nil)
}

// RegisterDecision schedules everything a decision tells: the audit line,
// the staff wake-ups of an approval, the decision pill, the co-guardian
// notices and the guardians' refetch.
func (e *CareRequestEffects) RegisterDecision(ctx context.Context, req *carerequests.Request, input carerequests.DecideInput, reason string, companionsChanged bool) error {
	if input.Approve {
		e.hooks.AfterCommit(ctx, func() { e.recordApplyAudit(req, input.ReviewedBy) })
		e.broadcastCareScheduleChanges(ctx, req.TenantID, req.StudentID, companionsChanged)
	} else {
		e.hooks.AfterCommit(ctx, func() {
			e.deps.Logger.Info("care request rejected",
				"request_id", req.ID,
				"student_id", req.StudentID,
				"tenant_id", req.TenantID,
				"reviewed_by", input.ReviewedBy,
			)
		})
	}
	status, body := careDecisionPill(req, input.Approve, reason)
	decisionPill := ports.RequestEvent{
		EventType: careplan.ParentMessageEventRequestStatus, ActorKind: careplan.ParentMessageActorStaff,
		ActorAccountID: input.ReviewedBy, Body: body,
		RequestType: careRequestPillType(req.RequestKind), RequestStatus: status,
		DecisionReason: reason, Payload: pickupPillPayload(req),
	}
	if err := e.emitPill(ctx, req, decisionPill); err != nil {
		return err
	}
	// Every other guardian of this child hears about the decision: the full
	// pill for explicit share recipients, a neutral line for the rest.
	if err := e.notifyOtherGuardians(ctx, req, decisionPill); err != nil {
		return err
	}
	e.WakeGuardians(ctx, req)
	return nil
}

// ReloadDecision reads the decided request back as the review item the
// caller returns. The child's name only labels the item; a failed lookup
// leaves it blank.
func (e *CareRequestEffects) ReloadDecision(ctx context.Context, requestID int64) (*carerequests.ReviewItem, error) {
	row, err := e.deps.Records.FindCareScheduleRequest(ctx, requestID, false)
	if err != nil {
		return nil, fmt.Errorf("schedule: reload decided care request: %w", err)
	}
	item := &carerequests.ReviewItem{Request: &row}
	if row.RequestKind == "pickup_change" {
		item.Reason = pickupChangeReason(&row)
	}
	if name, nameErr := e.deps.Students.StudentName(ctx, row.StudentID); nameErr == nil {
		item.FirstName, item.LastName = name.FirstName, name.LastName
	}
	return item, nil
}

// RecordDecision writes the "decided" ledger line.
func (e *CareRequestEffects) RecordDecision(ctx context.Context, req *carerequests.Request, actorID int64, payload map[string]any) error {
	return e.record(ctx, req, careplan.ParentRequestEventDecided, actorID, payload)
}

// RecordMarkedDone writes the "marked_done" ledger line.
func (e *CareRequestEffects) RecordMarkedDone(ctx context.Context, req *carerequests.Request, actorID int64, reason string) error {
	return e.record(ctx, req, careplan.ParentRequestEventMarkedDone, actorID, map[string]any{"reason": reason})
}

// NotifyMarkedDone posts the neutral "Anfrage abgeschlossen" pill.
func (e *CareRequestEffects) NotifyMarkedDone(ctx context.Context, req *carerequests.Request, actorID int64, reason string) error {
	return e.emitPill(ctx, req, ports.RequestEvent{
		EventType: careplan.ParentMessageEventRequestStatus, ActorKind: careplan.ParentMessageActorStaff,
		ActorAccountID: actorID, Body: withPickupDetail(parentRequestDoneBody, req),
		RequestType: careRequestPillType(req.RequestKind), RequestStatus: careplan.ParentMessageRequestStatusDone,
		DecisionReason: reason, Payload: pickupPillPayload(req),
	})
}

// RecordCorrection writes the "corrected" ledger line.
func (e *CareRequestEffects) RecordCorrection(ctx context.Context, req *carerequests.Request, actorID int64, payload map[string]any) error {
	return e.record(ctx, req, careplan.ParentRequestEventCorrected, actorID, payload)
}

// NotifyCorrection posts the "Entscheidung geändert" pill.
func (e *CareRequestEffects) NotifyCorrection(ctx context.Context, req *carerequests.Request, actorID int64, approve bool, reason string) error {
	status, body := correctedCarePill(req, approve)
	return e.emitPill(ctx, req, ports.RequestEvent{
		EventType: careplan.ParentMessageEventRequestStatus, ActorKind: careplan.ParentMessageActorStaff,
		ActorAccountID: actorID, Body: body,
		RequestType: careRequestPillType(req.RequestKind), RequestStatus: status, DecisionReason: reason, Payload: pickupPillPayload(req),
	})
}

// record writes one ledger entry inside the ambient transaction of the
// change it describes.
func (e *CareRequestEffects) record(ctx context.Context, req *carerequests.Request, eventType string, actorAccountID int64, payload map[string]any) error {
	if e.deps.Ledger == nil {
		return nil
	}
	if err := e.deps.Ledger.Record(ctx, ports.RequestLedgerEntry{
		StudentID:      req.StudentID,
		RequestType:    carerequests.LedgerRequestType(req.RequestKind),
		RequestID:      req.ID,
		EventType:      eventType,
		ActorAccountID: actorAccountID,
		UpdatedAt:      req.UpdatedAt,
		Payload:        payload,
	}); err != nil {
		return fmt.Errorf("schedule: record care request event: %w", err)
	}
	return nil
}

func (e *CareRequestEffects) tenantOf(ctx context.Context, rowTenantID int64) int64 {
	if rowTenantID > 0 {
		return rowTenantID
	}
	return e.hooks.TenantID(ctx)
}

// emitPill writes the durable decision intent in the ambient transaction,
// and appends the chat pill before commit.
func (e *CareRequestEffects) emitPill(ctx context.Context, req *carerequests.Request, ev ports.RequestEvent) error {
	if e.deps.Messenger == nil {
		return nil
	}
	tenantID := e.tenantOf(ctx, req.TenantID)
	ev.RefTable = careRequestRefTable
	ev.RefID = req.ID
	studentID, guardianAccountID := req.StudentID, req.SubmittedBy
	if err := e.deps.Messenger.EnqueueRequestDecision(ctx, tenantID, studentID, guardianAccountID, ev); err != nil {
		return fmt.Errorf("schedule: enqueue care request decision: %w", err)
	}
	return e.deps.Messenger.EmitChildEvent(ctx, tenantID, studentID, guardianAccountID, ev)
}

// broadcastCareScheduleChanges invalidates the staff-side caches an approved
// care-schedule change touches: the student master record (departure modes,
// student_updated, per school) and the arrival and pickup schedules
// (arrival_schedule_changed, global: the event the student list and detail
// and the "not arriving today" badges invalidate on). Registered after
// commit, so a woken client refetches the persisted plan, never the
// pre-commit snapshot. A broadcast error only costs other tabs an
// auto-refresh.
func (e *CareRequestEffects) broadcastCareScheduleChanges(ctx context.Context, tenantID, studentID int64, companionsChanged bool) {
	if e.deps.Broadcaster == nil {
		return
	}
	e.hooks.AfterCommit(ctx, func() {
		e.warnBroadcast("student_updated", tenantID, studentID, e.deps.Broadcaster.StudentUpdated(tenantID))
		// Only when the write actually trimmed a "läuft mit" link: links are
		// rows on ANOTHER child's card too. student_updated is not that
		// signal: companion views must not react to every student write, or
		// an unrelated edit discards someone's in-progress draft. A request
		// that only moves arrival or pickup times, or that re-merges the
		// departure modes the child already had, changes no link and stays
		// silent here for the same reason.
		if companionsChanged {
			e.warnBroadcast("student_companions_changed", tenantID, studentID, e.deps.Broadcaster.StudentCompanionsChanged(tenantID))
		}
		e.warnBroadcast("arrival_schedule_changed", tenantID, studentID, e.deps.Broadcaster.ArrivalScheduleChanged())
	})
}

func (e *CareRequestEffects) warnBroadcast(event string, tenantID, studentID int64, err error) {
	if err == nil {
		return
	}
	e.deps.Logger.Warn("schedule: broadcast "+event+" after care request approve failed",
		slog.Int64("tenant_id", tenantID),
		slog.Int64("student_id", studentID),
		slog.String("error", err.Error()),
	)
}

// notifyOtherGuardians posts the decision to the child's other guardians
// (#2267, story 47): the full pill to whoever the parent shared the request
// with, the neutral line to everyone else. The audience is resolved inside
// the transaction (a tenant-scoped read); only the pills go out after commit.
func (e *CareRequestEffects) notifyOtherGuardians(ctx context.Context, req *carerequests.Request, ev ports.RequestEvent) error {
	if e.deps.Messenger == nil {
		return nil
	}
	audience, err := e.deps.Messenger.ResolveDecisionAudience(ctx, req.StudentID, req.SubmittedBy, e.sharedRecipients(ctx, req))
	if err != nil {
		e.deps.Logger.Warn("co-guardian notice: resolving guardians failed",
			"request_id", req.ID,
			"student_id", req.StudentID,
			"error", err.Error(),
		)
		return err
	}
	if len(audience.Full) == 0 && len(audience.Neutral) == 0 {
		return nil
	}
	tenantID := e.tenantOf(ctx, req.TenantID)
	neutral := ev
	neutral.Body = careCoGuardianNoticeBody(req)
	// The neutral line names the day but nothing the parent did not share:
	// no requested time, no previous time (#3135).
	neutral.Payload = nil
	studentID := req.StudentID
	return e.deps.Messenger.EmitDecisionAudience(ctx, tenantID, studentID, audience, ev, neutral)
}

// sharedRecipients resolves the explicit recipients of one request,
// tolerating an unwired resolver. An error is not fatal: the fallback is that
// everyone gets the neutral line, which is the safe direction.
func (e *CareRequestEffects) sharedRecipients(ctx context.Context, req *carerequests.Request) []int64 {
	if e.deps.Shares == nil {
		return nil
	}
	accountIDs, err := e.deps.Shares.SharedRecipientAccountIDs(ctx, req.StudentID, carerequests.LedgerRequestType(req.RequestKind), req.ID)
	if err != nil {
		e.deps.Logger.Warn("resolving explicit request recipients failed, falling back to neutral notices",
			"request_id", req.ID,
			"student_id", req.StudentID,
			"error", err.Error(),
		)
		return nil
	}
	return accountIDs
}

// recordApplyAudit writes the audit trail for an applied request. A durable
// per-child change-audit table does not exist yet (#1455 owns that); until
// then this is a structured, GDPR-safe slog record: actor, child, tenant and
// WHICH weekdays changed, never the values.
func (e *CareRequestEffects) recordApplyAudit(req *carerequests.Request, accountID int64) {
	e.deps.Logger.Info("care request applied",
		slog.Int64("request_id", req.ID),
		slog.Int64("student_id", req.StudentID),
		slog.Int64("tenant_id", req.TenantID),
		slog.Int64("account_id", accountID),
		slog.String("changed", careScheduleAuditSummary(req.Payload)),
	)
}

// careScheduleAuditSummary lists the changed weekday numbers, keys only and
// no values, for the audit log.
func careScheduleAuditSummary(payload json.RawMessage) string {
	var p carerequests.WeeklyChange
	if json.Unmarshal(payload, &p) != nil {
		return ""
	}
	days := make([]string, 0, len(p.Weekdays))
	for _, wd := range p.Weekdays {
		days = append(days, strconv.Itoa(wd.Weekday))
	}
	return "weekdays:" + strings.Join(days, ",")
}

// CorrectionHistory reads the optional decision ledger already bound to the effects.
func (s *CareRequestEffects) CorrectionHistory(ctx context.Context, request *carerequests.Request) ([]ports.RequestCorrectionEvent, error) {
	reader, ok := s.deps.Ledger.(interface {
		CorrectionHistory(context.Context, *carerequests.Request) ([]ports.RequestCorrectionEvent, error)
	})
	if !ok {
		return nil, nil
	}
	return reader.CorrectionHistory(ctx, request)
}
