package application

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/careplan/masterdatarequests"
)

// German pill texts of a Stammdaten decision. The neutral line for the other
// guardians names the AREA that changed, never the value: a co-guardian who
// was not made an explicit recipient has no business reading the requested
// name (#2267, story 47).
const (
	masterDataApprovedBody   = "Anfrage bestätigt, Stammdaten übernommen"
	masterDataRejectedBody   = "Anfrage abgelehnt"
	masterDataCoGuardianBody = "Betreuungsstand geändert: Stammdaten"
)

// deferDecisionPill writes the durable decision intent in the decision
// transaction, then posts the best-effort chat pill after commit. Every other
// guardian of the child hears about it too: the full pill for explicit share
// recipients, the neutral line for the rest.
func (s *MasterDataDecisions) deferDecisionPill(ctx context.Context, req *careplan.StudentDataChangeRequest, actorAccountID int64, reason string, approved bool) error {
	if s.messenger == nil {
		return nil
	}
	tenantID := s.hooks.TenantID(ctx)
	event := masterDataDecisionEvent(req, actorAccountID, strings.TrimSpace(reason), approved)
	if err := s.messenger.EnqueueRequestDecision(ctx, tenantID, req.StudentID, req.SubmittedBy, event); err != nil {
		return fmt.Errorf("review: enqueue master-data request decision: %w", err)
	}
	studentID, guardianAccountID := req.StudentID, req.SubmittedBy
	s.hooks.AfterCommit(ctx, func() {
		s.messenger.EmitChildEvent(tenantID, studentID, guardianAccountID, event)
	})
	s.notifyOtherGuardiansAfterCommit(ctx, req, event)
	return nil
}

func masterDataDecisionEvent(req *careplan.StudentDataChangeRequest, actorAccountID int64, reason string, approved bool) ports.RequestEvent {
	body := masterDataApprovedBody
	status := careplan.ParentMessageRequestStatusDone
	if !approved {
		status = careplan.ParentMessageRequestStatusReject
		body = masterDataRejectedBody
		if reason != "" {
			body = masterDataRejectedBody + ": " + reason
		}
	}
	return ports.RequestEvent{
		EventType:      careplan.ParentMessageEventRequestStatus,
		ActorKind:      careplan.ParentMessageActorStaff,
		ActorAccountID: actorAccountID,
		Body:           body,
		RequestType:    masterdatarequests.ParentRequestType,
		RequestStatus:  status,
		DecisionReason: reason,
		RefTable:       masterdatarequests.RefTable,
		RefID:          req.ID,
	}
}

// notifyOtherGuardiansAfterCommit resolves the audience inside the
// transaction (a tenant-scoped read); only the pills go out after commit.
func (s *MasterDataDecisions) notifyOtherGuardiansAfterCommit(ctx context.Context, req *careplan.StudentDataChangeRequest, event ports.RequestEvent) {
	audience, err := s.messenger.ResolveDecisionAudience(ctx, req.StudentID, req.SubmittedBy, s.sharedRecipients(ctx, req))
	if err != nil {
		s.logger.Warn("co-guardian notice: resolving guardians failed",
			slog.Int64("request_id", req.ID),
			slog.Int64("student_id", req.StudentID),
			slog.String("error", err.Error()),
		)
		return
	}
	if len(audience.Full) == 0 && len(audience.Neutral) == 0 {
		return
	}
	tenantID := s.hooks.TenantID(ctx)
	neutral := event
	neutral.Body = masterDataCoGuardianBody
	studentID := req.StudentID
	s.hooks.AfterCommit(ctx, func() {
		s.messenger.EmitDecisionAudience(tenantID, studentID, audience, event, neutral)
	})
}

// sharedRecipients resolves who the parent explicitly shared the request
// with, tolerating an unwired resolver. On error everyone falls back to the
// neutral line.
func (s *MasterDataDecisions) sharedRecipients(ctx context.Context, req *careplan.StudentDataChangeRequest) []int64 {
	if s.shares == nil {
		return nil
	}
	accountIDs, err := s.shares.SharedRecipientAccountIDs(ctx, req.StudentID, masterdatarequests.ParentRequestType, req.ID)
	if err != nil {
		s.logger.Warn("resolving explicit request recipients failed, falling back to neutral notices",
			slog.Int64("request_id", req.ID),
			slog.Int64("student_id", req.StudentID),
			slog.String("error", err.Error()),
		)
		return nil
	}
	return accountIDs
}

// deferStudentUpdated invalidates child cards after commit.
func (s *MasterDataDecisions) deferStudentUpdated(ctx context.Context, studentID int64) {
	if s.broadcaster == nil {
		return
	}
	tenantID := s.hooks.TenantID(ctx)
	s.hooks.AfterCommit(ctx, func() {
		if tenantID <= 0 {
			s.logger.Warn("review: skipping student_updated broadcast without tenant",
				slog.Int64("student_id", studentID),
			)
			return
		}
		if err := s.broadcaster.StudentUpdated(tenantID); err != nil {
			s.logger.Warn("review: failed to broadcast student update",
				slog.Int64("tenant_id", tenantID),
				slog.Int64("student_id", studentID),
				slog.String("error", err.Error()),
			)
		}
	})
}

// deferStudentCompanionsChanged announces, after commit, that the approved
// change trimmed the child's Laufgemeinschaft — the signal every mounted
// "läuft mit" view refetches on.
func (s *MasterDataDecisions) deferStudentCompanionsChanged(ctx context.Context, studentID int64) {
	if s.broadcaster == nil {
		return
	}
	tenantID := s.hooks.TenantID(ctx)
	s.hooks.AfterCommit(ctx, func() {
		if tenantID <= 0 {
			return
		}
		if err := s.broadcaster.StudentCompanionsChanged(tenantID); err != nil {
			s.logger.Warn("review: failed to broadcast student companions change",
				slog.Int64("tenant_id", tenantID),
				slog.Int64("student_id", studentID),
				slog.String("error", err.Error()),
			)
		}
	})
}
