package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
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
	ctx context.Context, req *careplan.ExcusedAbsenceRequest, student *careplan.ReviewStudent, replacePartial bool,
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
	if err := w.ensureNoPartialAbsence(ctx, req.StudentID, req.Dates, replacePartial); err != nil {
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

// absenceEligibility is the bulk verdict of one request plus the manual
// partial absences on its days, which the review card names (#5698).
type absenceEligibility struct {
	eligible    bool
	code, text  string
	partialFrom map[string]string
}

// excusedBulkEligibility answers whether this request can ride a bulk
// approval. It takes the already-resolved status view so the queue reads the
// child's current days exactly once per request, whichever branch decides.
func (w *ExcusedRequests) excusedBulkEligibility(ctx context.Context, req *careplan.ExcusedAbsenceRequest, student careplan.ReviewStudent, view currentStatusView, today careplan.Date, facts absenceReviewFacts) (absenceEligibility, error) {
	beyondCare := requestExtendsBeyondCare(req, &student)
	// The card names the partial absences whenever a single approval is
	// still possible, also when an earlier day of the request has passed.
	var partialFrom map[string]string
	if !beyondCare && !parentRequestIsPast(excusedScopeEnd(req), today) {
		var err error
		if partialFrom, err = facts.manualPartialAbsences(ctx, w, req.StudentID, req.Dates); err != nil {
			return absenceEligibility{}, err
		}
	}
	if !absenceBulkEligible(req.Dates, today) {
		return absenceEligibility{code: careplan.BulkIneligiblePast, text: "Mindestens ein Tag ist vorbei.", partialFrom: partialFrom}, nil
	}
	if beyondCare {
		return absenceEligibility{code: careplan.BulkIneligibleChildUnavailable, text: "Mindestens ein Tag liegt nach dem Betreuungsende."}, nil
	}
	if len(partialFrom) > 0 {
		// Replacing a partial absence is a deliberate single decision.
		return absenceEligibility{
			code: careplan.BulkIneligibleConflict, text: "Für mindestens einen Tag ist bereits eine Teilabwesenheit eingetragen.",
			partialFrom: partialFrom,
		}, nil
	}
	if w.messenger != nil {
		hasAccess, accessErr := w.messenger.GuardianHasChildAccess(ctx, req.StudentID, req.SubmittedBy)
		if accessErr != nil {
			return absenceEligibility{}, accessErr
		}
		if !hasAccess {
			return absenceEligibility{code: careplan.BulkIneligibleAccessRevoked, text: "Der Zugang der einreichenden Person ist nicht mehr aktiv."}, nil
		}
	}
	if view.changedSinceRequest {
		return absenceEligibility{code: careplan.BulkIneligibleStale, text: "Für mindestens einen Tag gibt es einen neueren Abwesenheitsstatus."}, nil
	}
	return absenceEligibility{eligible: true}, nil
}

// manualPartialAbsences returns the staff-entered partial absences on the
// requested days as date -> "HH:MM" (the time the excusal starts).
func (w *ExcusedRequests) manualPartialAbsences(ctx context.Context, studentID int64, dates []careplan.Date) (map[string]string, error) {
	if len(dates) == 0 {
		return nil, nil
	}
	rows, err := w.carePlan.ListPickupExceptions(ctx, careplan.StudentScheduleFilter{
		StudentIDs: []int64{studentID}, From: dates[0], To: dates[len(dates)-1],
	})
	if err != nil {
		return nil, err
	}
	return manualPartialAbsenceFrom(dates, rows), nil
}

func manualPartialAbsenceFrom(dates []careplan.Date, rows []careplan.PickupException) map[string]string {
	requested := make(map[careplan.Date]struct{}, len(dates))
	for _, date := range dates {
		requested[date] = struct{}{}
	}
	var out map[string]string
	for _, row := range rows {
		// Only manual partial absences conflict; auto-derived excusals
		// (pulled-forward pickup time) coexist with a full-day status.
		if _, ok := requested[row.ExceptionDate]; !ok || !row.HasManualPartialAbsence() {
			continue
		}
		if out == nil {
			out = make(map[string]string)
		}
		out[row.ExceptionDate.String()] = calendar.NormalizeWallClock(*row.ExcusedFrom).Format("15:04")
	}
	return out
}

// lockRequestedDays locks every requested care day in the canonical order
// (student row first, then care-day) and returns the sorted days.
func (w *ExcusedRequests) lockRequestedDays(ctx context.Context, studentID int64, dates []careplan.Date) ([]careplan.Date, error) {
	// Sort before locking so multi-day writers always take care-day keys in
	// the same order (avoids AB-BA deadlocks across concurrent requests).
	sortedDates := dedupeSortedDates(dates)
	for _, date := range sortedDates {
		if err := w.carePlan.LockStudentAndExceptionDay(ctx, studentID, date.String()); err != nil {
			return nil, err
		}
	}
	return sortedDates, nil
}

// ensureNoPartialAbsence locks every requested care day and refuses when a
// manual partial-day excusal already owns one of the dates. With replace set,
// the reviewer confirmed replacing those partial absences (#5698): Care Plan
// removes them under the same locks, restoring blocks and pickup time exactly
// like deleting them, before the approval writes the full-day status.
func (w *ExcusedRequests) ensureNoPartialAbsence(ctx context.Context, studentID int64, dates []careplan.Date, replace bool) error {
	if len(dates) == 0 {
		return nil
	}
	sortedDates, err := w.lockRequestedDays(ctx, studentID, dates)
	if err != nil {
		return err
	}
	partial, err := w.manualPartialAbsences(ctx, studentID, sortedDates)
	if err != nil || len(partial) == 0 {
		return err
	}
	if !replace {
		return careplan.ErrExcusedRequestStatusConflict
	}
	if w.partialAbsences == nil {
		return errors.New("care plan: partial absence replacement is not configured")
	}
	return w.partialAbsences.ReplaceManualPartialAbsences(ctx, studentID, calendarDates(sortedDates))
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
	if len(req.Dates) == 0 {
		return statusViewFromRows(req, nil), nil
	}
	rows, err := w.activeStatusDays(ctx, req)
	if err != nil {
		return currentStatusView{}, err
	}
	return statusViewFromRows(req, rows), nil
}

func statusViewFromRows(req *careplan.ExcusedAbsenceRequest, rows []careplan.StudentStatusDay) currentStatusView {
	view := currentStatusView{byDate: make(map[string]string, len(req.Dates))}
	// Every requested day starts as "present"; the rows below overwrite the
	// days that carry a status, so a day with no record is still answered for.
	for _, date := range req.Dates {
		view.byDate[date.String()] = careplan.StudentStatusDayPresent
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
	return view
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
	guardianAccountID := req.SubmittedBy
	// Each write replaces the status standing for its day (#6958).
	for _, d := range req.Dates {
		if _, err := w.carePlan.UpsertStudentStatusDay(ctx, careplan.StudentStatusDay{
			StudentID: req.StudentID, Date: d, Status: req.AbsenceStatus, ReportedAt: now,
			Source: careplan.StudentStatusSourceParent, GuardianAccountID: &guardianAccountID, Note: &note,
			ReportedByAccountID: &guardianAccountID,
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
func (w *ExcusedRequests) revertApprovedAbsence(ctx context.Context, req *careplan.ExcusedAbsenceRequest, reviewer int64) error {
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
	return w.carePlan.ClearStudentStatusDays(ctx, req.StudentID, req.AbsenceStatus, req.Dates, time.Now(), careplan.StudentStatusSourceManual, &reviewer)
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
