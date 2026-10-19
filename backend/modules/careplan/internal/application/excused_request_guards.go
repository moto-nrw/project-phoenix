package application

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

func (w *ExcusedRequests) reviewScope(ctx context.Context) (careplan.ReviewScope, error) {
	scope, err := w.scope(ctx)
	if err != nil {
		return careplan.ReviewScope{}, fmt.Errorf("care plan: resolve request reviewer scope: %w", err)
	}
	return scope, nil
}

func (w *ExcusedRequests) requireReviewer(ctx context.Context, student *careplan.ReviewStudent) error {
	scope, err := w.reviewScope(ctx)
	if err != nil {
		return err
	}
	if !scope.Allows(student) {
		return careplan.ErrExcusedRequestForbidden
	}
	return nil
}

// validateAbsenceApproval keeps every path that writes parent-sourced status
// days behind the same lifecycle, conflict, and guardian-access checks.
func (w *ExcusedRequests) validateAbsenceApproval(
	ctx context.Context, req *careplan.ExcusedAbsenceRequest, student *careplan.ReviewStudent,
) error {
	today := w.today()
	if student.Alumnus || student.CareEndedOn(today) {
		return careplan.ErrExcusedRequestNotFound
	}
	// Approving a request whose days have all passed would write absence
	// records into a settled past. Staff either reject it or mark it done.
	if parentRequestIsPast(excusedScopeEnd(req), today) {
		return careplan.ErrParentRequestPast
	}
	if requestExtendsBeyondCare(req, student) {
		return careplan.ErrExcusedRequestNotFound
	}
	if err := w.ensureNoPartialAbsence(ctx, req.StudentID, req.Dates); err != nil {
		return err
	}
	// Refuse to apply when the submitting guardian has lost access to the
	// child since the request was filed. Approving writes parent-sourced
	// status days and posts a parent-visible pill for a recipient the parent
	// APIs now hide; staff wind such a request down by rejecting.
	if w.messenger != nil {
		hasAccess, err := w.messenger.GuardianHasChildAccess(ctx, req.StudentID, req.SubmittedBy)
		if err != nil {
			return fmt.Errorf("care plan: absence request guardian link check: %w", err)
		}
		if !hasAccess {
			return careplan.ErrExcusedRequestGuardianAccessRevoked
		}
	}
	// Refuse to apply when the child's absence for a requested date was
	// created or changed after this request was filed: a blind approve would
	// silently overwrite that newer decision.
	return w.ensureNoNewerStatus(ctx, req)
}

// excusedBulkEligibility answers whether this request can ride a bulk
// approval. It takes the already-resolved status view so the queue reads the
// child's current days exactly once per request, whichever branch decides.
func (w *ExcusedRequests) excusedBulkEligibility(ctx context.Context, req *careplan.ExcusedAbsenceRequest, student careplan.ReviewStudent, view currentStatusView, today careplan.Date) (bool, string, string, error) {
	if !absenceBulkEligible(req.Dates, today) {
		return false, careplan.BulkIneligiblePast, "Mindestens ein Tag ist vorbei.", nil
	}
	if requestExtendsBeyondCare(req, &student) {
		return false, careplan.BulkIneligibleChildUnavailable, "Mindestens ein Tag liegt nach dem Betreuungsende.", nil
	}
	partial, err := w.hasManualPartialAbsence(ctx, req.StudentID, req.Dates)
	if err != nil {
		return false, "", "", err
	}
	if partial {
		return false, careplan.BulkIneligibleConflict, "Für mindestens einen Tag ist bereits eine Teilabwesenheit eingetragen.", nil
	}
	if w.messenger != nil {
		hasAccess, accessErr := w.messenger.GuardianHasChildAccess(ctx, req.StudentID, req.SubmittedBy)
		if accessErr != nil {
			return false, "", "", accessErr
		}
		if !hasAccess {
			return false, careplan.BulkIneligibleAccessRevoked, "Der Zugang der einreichenden Person ist nicht mehr aktiv.", nil
		}
	}
	if view.changedSinceRequest {
		return false, careplan.BulkIneligibleStale, "Für mindestens einen Tag gibt es einen neueren Abwesenheitsstatus.", nil
	}
	return true, "", "", nil
}

func (w *ExcusedRequests) hasManualPartialAbsence(ctx context.Context, studentID int64, dates []careplan.Date) (bool, error) {
	if len(dates) == 0 {
		return false, nil
	}
	rows, err := w.carePlan.ListPickupExceptions(ctx, careplan.StudentScheduleFilter{
		StudentIDs: []int64{studentID}, From: dates[0], To: dates[len(dates)-1],
	})
	if err != nil {
		return false, err
	}
	requested := make(map[careplan.Date]struct{}, len(dates))
	for _, date := range dates {
		requested[date] = struct{}{}
	}
	for _, row := range rows {
		// Only manual partial absences conflict; auto-derived excusals
		// (pulled-forward pickup time) coexist with a full-day status.
		if _, ok := requested[row.ExceptionDate]; ok && row.ExcusedFrom != nil && !row.ExcusedAuto {
			return true, nil
		}
	}
	return false, nil
}

// ensureNoPartialAbsence locks every requested care day in the canonical
// order (student row first, then care-day) and refuses when a manual
// partial-day excusal already owns one of the dates.
func (w *ExcusedRequests) ensureNoPartialAbsence(ctx context.Context, studentID int64, dates []careplan.Date) error {
	if len(dates) == 0 {
		return nil
	}
	// Sort before locking so multi-day writers always take care-day keys in
	// the same order (avoids AB-BA deadlocks across concurrent requests).
	sortedDates := dedupeSortedDates(dates)
	for _, date := range sortedDates {
		if err := w.carePlan.LockStudentAndExceptionDay(ctx, studentID, date.String()); err != nil {
			return err
		}
	}
	partial, err := w.hasManualPartialAbsence(ctx, studentID, sortedDates)
	if err != nil {
		return err
	}
	if partial {
		return careplan.ErrExcusedRequestStatusConflict
	}
	return nil
}

// ensureNoNewerStatus refuses the approval when any active status day on a
// requested date was reported after the request was created: a newer
// absence decision that approving would clobber.
func (w *ExcusedRequests) ensureNoNewerStatus(ctx context.Context, req *careplan.ExcusedAbsenceRequest) error {
	view, err := w.currentStatusView(ctx, req)
	if err != nil {
		return err
	}
	if view.changedSinceRequest {
		return careplan.ErrExcusedRequestStatusConflict
	}
	return nil
}

// currentStatusView is what the child's days look like right now, for exactly
// the days this request asks about. The approve gate and the review queue
// share it, so they can never disagree.
type currentStatusView struct {
	byDate              map[string]string
	changedSinceRequest bool
}

func (w *ExcusedRequests) currentStatusView(ctx context.Context, req *careplan.ExcusedAbsenceRequest) (currentStatusView, error) {
	view := currentStatusView{byDate: make(map[string]string, len(req.Dates))}
	if len(req.Dates) == 0 {
		return view, nil
	}
	// Every requested day starts as "present"; the rows below overwrite the
	// days that carry a status, so a day with no record is still answered for.
	for _, date := range req.Dates {
		view.byDate[date.String()] = careplan.StudentStatusDayPresent
	}
	rows, err := w.activeStatusDays(ctx, req)
	if err != nil {
		return currentStatusView{}, err
	}
	for _, row := range rows {
		key := row.Date.String()
		if _, requested := view.byDate[key]; !requested {
			continue
		}
		view.byDate[key] = row.Status
		if row.ReportedAt.After(req.CreatedAt) {
			view.changedSinceRequest = true
		}
	}
	return view, nil
}

// activeStatusDays reads the child's active status rows across the request's
// date span. Dates are stored sorted ascending, so first/last bound the range.
func (w *ExcusedRequests) activeStatusDays(ctx context.Context, req *careplan.ExcusedAbsenceRequest) ([]careplan.StudentStatusDay, error) {
	return w.carePlan.ListStudentStatusDays(ctx, careplan.StudentStatusDayFilter{
		StudentIDs: []int64{req.StudentID}, From: req.Dates[0], To: req.Dates[len(req.Dates)-1], ActiveOnly: true,
	})
}

// applyAbsenceRequest writes the approved request using its requested status,
// mirroring the direct parent path. It also updates today's live sick flag.
func (w *ExcusedRequests) applyAbsenceRequest(ctx context.Context, req *careplan.ExcusedAbsenceRequest) error {
	if len(req.Dates) == 0 {
		return careplan.ErrExcusedRequestNoDates
	}
	now := time.Now()
	note := req.Note
	for _, other := range careplan.StudentStatusDayStatusesExcept(req.AbsenceStatus) {
		if err := w.carePlan.ClearStudentStatusDays(ctx, req.StudentID, other, req.Dates, now, careplan.StudentStatusSourceParent); err != nil {
			return err
		}
	}
	for _, d := range req.Dates {
		guardianAccountID := req.SubmittedBy
		if _, err := w.carePlan.UpsertStudentStatusDay(ctx, careplan.StudentStatusDay{
			StudentID: req.StudentID, Date: d, Status: req.AbsenceStatus, ReportedAt: now,
			Source: careplan.StudentStatusSourceParent, GuardianAccountID: &guardianAccountID, Note: &note,
		}); err != nil {
			return err
		}
	}
	if containsDate(req.Dates, w.today()) {
		if err := w.students.SetLiveAbsenceFlags(ctx, req.StudentID, req.AbsenceStatus, now); err != nil {
			return err
		}
	}
	return nil
}

// revertApprovedAbsence clears exactly the days this request wrote, and only
// while they are provably still its own. A day that was re-reported after
// the approval belongs to whoever wrote it last.
func (w *ExcusedRequests) revertApprovedAbsence(ctx context.Context, req *careplan.ExcusedAbsenceRequest) error {
	if req.Status != careplan.ExcusedRequestStatusApproved || len(req.Dates) == 0 {
		return nil
	}
	rows, err := w.activeStatusDays(ctx, req)
	if err != nil {
		return err
	}
	requested := make(map[careplan.Date]struct{}, len(req.Dates))
	for _, date := range req.Dates {
		requested[date] = struct{}{}
	}
	for i := range rows {
		row := &rows[i]
		if _, ok := requested[row.Date]; !ok || row.Status != req.AbsenceStatus {
			continue
		}
		if !absenceRowWrittenByRequest(row, req) {
			return fmt.Errorf("%w: der Eintrag für den %s wurde nach der Entscheidung geändert",
				careplan.ErrParentRequestCorrectionUnsupported, germanDate(row.Date))
		}
	}
	return w.carePlan.ClearStudentStatusDays(ctx, req.StudentID, req.AbsenceStatus, req.Dates, time.Now(), careplan.StudentStatusSourceParent)
}

// absenceRowWrittenByRequest reports whether this status day is still the one
// the request's approval produced: parent-sourced, from the same guardian, and
// not re-reported since the decision.
func absenceRowWrittenByRequest(row *careplan.StudentStatusDay, req *careplan.ExcusedAbsenceRequest) bool {
	if row.Source != careplan.StudentStatusSourceParent {
		return false
	}
	if row.GuardianAccountID == nil || *row.GuardianAccountID != req.SubmittedBy {
		return false
	}
	return req.ReviewedAt == nil || !row.ReportedAt.After(*req.ReviewedAt)
}
