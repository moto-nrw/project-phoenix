package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// offeringChangeRefTable names the request row for chat pills.
const offeringChangeRefTable = "enrollment.offering_change_requests"

// offeringChangeRejectedBody is the German pill text of a rejection. The
// staff portal renders it directly; the parents portal localizes from the
// structured event fields.
const offeringChangeRejectedBody = "Anfrage abgelehnt"

// offeringChangeApprovedBody names the date the office confirmed, so the
// pill answers the question a family asks next: from when (#2484).
func offeringChangeApprovedBody(effectiveFrom calendar.Date) string {
	return "Anfrage bestätigt, Betreuungsangebote werden ab " +
		effectiveFrom.Format("02.01.2006") + " umgestellt"
}

func requestCreatedEvent(accountID int64) ports.RequestEvent {
	return ports.RequestEvent{
		EventType: careplan.ParentMessageEventRequestCreated, ActorKind: careplan.ParentMessageActorGuardian,
		ActorAccountID: accountID, Body: offeringChangeCreatedBody, RequestType: careplan.OfferingChangeRequestType,
		RequestStatus: careplan.ParentMessageRequestStatusOpen,
	}
}

// recordOfferingRequestEvent appends one ledger entry inside the ambient
// transaction of the change it describes.
func (s *OfferingChanges) recordOfferingRequestEvent(
	ctx context.Context,
	row careplan.OfferingChangeRequest,
	eventType string,
	actorAccountID int64,
	payload map[string]any,
) error {
	if s.deps.Ledger == nil {
		return nil
	}
	if err := s.deps.Ledger.Record(ctx, ports.RequestLedgerEntry{
		StudentID: row.StudentID, RequestType: careplan.ParentRequestTypeOffering, RequestID: row.ID,
		EventType: eventType, ActorAccountID: actorAccountID, UpdatedAt: row.UpdatedAt, Payload: payload,
	}); err != nil {
		return fmt.Errorf("offering change: record request event: %w", err)
	}
	return nil
}

// recordOfferingDecision reloads the decided row so the ledger carries the
// version the decision produced, then appends the "decided" entry.
func (s *OfferingChanges) recordOfferingDecision(ctx context.Context, requestID, reviewedBy int64, approve bool, reason string) error {
	if s.deps.Ledger == nil {
		return nil
	}
	row, err := s.deps.Rows.Find(ctx, requestID)
	if err != nil {
		return fmt.Errorf("offering change: reload decided request: %w", err)
	}
	return s.recordOfferingRequestEvent(ctx, row, careplan.ParentRequestEventDecided, reviewedBy,
		map[string]any{"approve": approve, "reason": reason})
}

// decisionPill is one staff decision's pill to the submitting guardian.
type decisionPill struct {
	reviewedBy int64
	body       string
	status     string
	reason     string
	payload    map[string]any
}

func (s *OfferingChanges) emitDecisionPill(ctx context.Context, row careplan.OfferingChangeRequest, pill decisionPill) error {
	event := ports.RequestEvent{
		EventType: careplan.ParentMessageEventRequestStatus, ActorKind: careplan.ParentMessageActorStaff,
		ActorAccountID: pill.reviewedBy, Body: pill.body, RequestType: careplan.OfferingChangeRequestType,
		RequestStatus: pill.status, DecisionReason: pill.reason, Payload: pill.payload,
	}
	if err := s.emitPillAfterCommit(ctx, row, event); err != nil {
		return err
	}
	// Every other guardian of the child hears about the decision: the full
	// pill for explicit share recipients, a neutral line for the rest.
	s.notifyOtherGuardiansAfterCommit(ctx, row, event)
	return nil
}

func (s *OfferingChanges) rowTenantID(ctx context.Context, row careplan.OfferingChangeRequest) int64 {
	if row.TenantID > 0 {
		return row.TenantID
	}
	return s.deps.Hooks.TenantID(ctx)
}

// emitPillAfterCommit writes the durable decision intent in the surrounding
// transaction, then posts the best-effort parent-OGS pill after commit.
func (s *OfferingChanges) emitPillAfterCommit(ctx context.Context, row careplan.OfferingChangeRequest, event ports.RequestEvent) error {
	messenger := s.deps.Messenger
	if messenger == nil {
		return nil
	}
	tenantID := s.rowTenantID(ctx, row)
	event.RefTable, event.RefID = offeringChangeRefTable, row.ID
	studentID, guardianAccountID := row.StudentID, row.SubmittedBy
	if err := messenger.EnqueueRequestDecision(ctx, tenantID, studentID, guardianAccountID, event); err != nil {
		return fmt.Errorf("offering change: enqueue request decision: %w", err)
	}
	s.deps.Hooks.AfterCommit(ctx, func() {
		messenger.EmitChildEvent(tenantID, studentID, guardianAccountID, event)
		messenger.BroadcastChildUpdateToGuardians(tenantID, studentID)
	})
	return nil
}

// sharedRecipients resolves the explicit recipients, tolerating an unwired
// resolver. On error everyone falls back to the neutral line: seeing less
// than entitled is a nuisance, seeing more is a leak.
func (s *OfferingChanges) sharedRecipients(ctx context.Context, row careplan.OfferingChangeRequest) []int64 {
	if s.deps.Shares == nil {
		return nil
	}
	accountIDs, err := s.deps.Shares.SharedRecipientAccountIDs(ctx, row.StudentID, careplan.ParentRequestTypeOffering, row.ID)
	if err != nil {
		s.logger().Warn("resolving explicit request recipients failed, falling back to neutral notices",
			slog.Int64("request_id", row.ID),
			slog.Int64("student_id", row.StudentID),
			slog.String("error", err.Error()),
		)
		return nil
	}
	return accountIDs
}

// notifyOtherGuardiansAfterCommit posts the decision to the child's other
// guardians (#2267, story 47). The audience is resolved inside the
// transaction; the pills go out after commit.
func (s *OfferingChanges) notifyOtherGuardiansAfterCommit(ctx context.Context, row careplan.OfferingChangeRequest, event ports.RequestEvent) {
	messenger := s.deps.Messenger
	if messenger == nil {
		return
	}
	audience, err := messenger.ResolveDecisionAudience(ctx, row.StudentID, row.SubmittedBy, s.sharedRecipients(ctx, row))
	if err != nil {
		s.logger().Warn("co-guardian notice: resolving guardians failed",
			slog.Int64("request_id", row.ID),
			slog.Int64("student_id", row.StudentID),
			slog.String("error", err.Error()),
		)
		return
	}
	if len(audience.Full) == 0 && len(audience.Neutral) == 0 {
		return
	}
	tenantID := s.rowTenantID(ctx, row)
	neutral := event
	neutral.Body = "Betreuungsstand geändert: Angebote ab " + offeringChangeEffectiveFrom(row).Format("02.01.2006")
	studentID := row.StudentID
	s.deps.Hooks.AfterCommit(ctx, func() {
		messenger.EmitDecisionAudience(tenantID, studentID, audience, event, neutral)
	})
}
