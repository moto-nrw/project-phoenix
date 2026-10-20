package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

func (w *ExcusedRequests) record(ctx context.Context, entry ports.RequestLedgerEntry) error {
	if w.ledger == nil {
		return nil
	}
	return w.ledger.Record(ctx, entry)
}

func (w *ExcusedRequests) tenantOf(ctx context.Context, rowTenantID int64) int64 {
	if rowTenantID > 0 {
		return rowTenantID
	}
	return w.hooks.TenantID(ctx)
}

// emitRequestPill writes the durable decision intent in the ambient
// transaction, then schedules the best-effort chat pill after commit.
func (w *ExcusedRequests) emitRequestPill(ctx context.Context, req *careplan.ExcusedAbsenceRequest, ev ports.RequestEvent) error {
	if w.messenger == nil {
		return nil
	}
	tenantID := w.tenantOf(ctx, req.TenantID)
	ev.RefTable = excusedRequestRefTable
	ev.RefID = req.ID
	studentID, guardianAccountID := req.StudentID, req.SubmittedBy
	if err := w.messenger.EnqueueRequestDecision(ctx, tenantID, studentID, guardianAccountID, ev); err != nil {
		return fmt.Errorf("care plan: enqueue absence request decision: %w", err)
	}
	return w.messenger.EmitChildEvent(ctx, tenantID, studentID, guardianAccountID, ev)
}

// notifyOtherGuardians posts the neutral „Betreuungsstand
// geändert“ line to every other portal guardian of the child. The guardians
// are resolved inside the transaction; only the pills are posted after commit.
func (w *ExcusedRequests) notifyOtherGuardians(ctx context.Context, req *careplan.ExcusedAbsenceRequest, ev ports.RequestEvent) error {
	if w.messenger == nil {
		return nil
	}
	audience, err := w.messenger.ResolveDecisionAudience(ctx, req.StudentID, req.SubmittedBy, w.sharedRecipients(ctx, req))
	if err != nil {
		w.logger.Warn("co-guardian notice: resolving guardians failed",
			slog.Int64("request_id", req.ID),
			slog.Int64("student_id", req.StudentID),
			slog.String("error", err.Error()),
		)
		return err
	}
	if len(audience.Full) == 0 && len(audience.Neutral) == 0 {
		return nil
	}
	tenantID := w.tenantOf(ctx, req.TenantID)
	neutral := ev
	neutral.Body = coGuardianNoticeBody(req)
	studentID := req.StudentID
	return w.messenger.EmitDecisionAudience(ctx, tenantID, studentID, audience, ev, neutral)
}

// sharedRecipients resolves the explicit recipients of one request,
// tolerating an unwired resolver. An error is not fatal: the fallback is that
// everyone gets the neutral line, which is the safe direction.
func (w *ExcusedRequests) sharedRecipients(ctx context.Context, req *careplan.ExcusedAbsenceRequest) []int64 {
	if w.shares == nil {
		return nil
	}
	accountIDs, err := w.shares.SharedRecipientAccountIDs(ctx, req.StudentID, careplan.ParentRequestTypeExcusedAbsence, req.ID)
	if err != nil {
		w.logger.Warn("resolving explicit request recipients failed, falling back to neutral notices",
			slog.Int64("request_id", req.ID),
			slog.Int64("student_id", req.StudentID),
			slog.String("error", err.Error()),
		)
		return nil
	}
	return accountIDs
}

// --- store helpers

func (w *ExcusedRequests) listPendingForStudent(ctx context.Context, studentID int64) ([]*careplan.ExcusedAbsenceRequest, error) {
	rows, err := w.carePlan.ListExcusedAbsenceRequests(ctx, careplan.ExcusedAbsenceRequestFilter{
		StudentID: studentID, Statuses: []string{careplan.ExcusedRequestStatusPending},
	})
	if err != nil {
		return nil, err
	}
	result := make([]*careplan.ExcusedAbsenceRequest, 0, len(rows))
	for i := range rows {
		result = append(result, &rows[i])
	}
	return result, nil
}

func (w *ExcusedRequests) listPendingForTenant(ctx context.Context, queue *careplan.RequestQueueFilter) ([]careplan.ExcusedAbsenceRequest, error) {
	return w.carePlan.ListExcusedAbsenceRequests(ctx, careplan.ExcusedAbsenceRequestFilter{
		Statuses: []string{careplan.ExcusedRequestStatusPending}, Queue: queue,
	})
}

func (w *ExcusedRequests) loadStudentNames(ctx context.Context, studentIDs []int64, what string) (map[int64]careplan.ReviewStudent, map[int64]careplan.PersonName, error) {
	students, err := w.students.FindStudents(ctx, studentIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("care plan: load students for %s: %w", what, err)
	}
	personIDs := make([]int64, 0, len(students))
	for _, st := range students {
		personIDs = append(personIDs, st.PersonID)
	}
	persons, err := w.students.PersonNames(ctx, personIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("care plan: load persons for %s: %w", what, err)
	}
	return students, persons, nil
}
