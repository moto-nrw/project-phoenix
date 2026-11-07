package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// absenceReviewFacts holds the two batch results for this queue page, not a
// request memo. A failed batch leaves its map nil: each item then retries that
// read in the original scalar order, without discarding the other batch.
type absenceReviewFacts struct {
	statusDays map[int64][]careplan.StudentStatusDay
	pickups    map[int64][]careplan.PickupException
}

func visibleExcusedRequests(rows []careplan.ExcusedAbsenceRequest, students map[int64]careplan.ReviewStudent, scope careplan.ReviewScope, today careplan.Date) []careplan.ExcusedAbsenceRequest {
	visible := make([]careplan.ExcusedAbsenceRequest, 0, len(rows))
	for _, row := range rows {
		student, ok := students[row.StudentID]
		// Graduation and care end hide surviving pending requests as well.
		if ok && scope.Allows(&student) && !student.Alumnus && !student.CareEndedOn(today) {
			visible = append(visible, row)
		}
	}
	return visible
}

func (w *ExcusedRequests) loadAbsenceReviewFacts(ctx context.Context, rows []careplan.ExcusedAbsenceRequest, students map[int64]careplan.ReviewStudent, today careplan.Date) absenceReviewFacts {
	var facts absenceReviewFacts
	statusIDs, from, to := absenceReviewRange(rows)
	if len(statusIDs) < 2 {
		return facts
	}
	days, err := w.carePlan.ListStudentStatusDays(ctx, careplan.StudentStatusDayFilter{
		StudentIDs: statusIDs, From: from, To: to, ActiveOnly: true,
	})
	if err == nil {
		facts.statusDays = make(map[int64][]careplan.StudentStatusDay, len(statusIDs))
		for _, id := range statusIDs {
			facts.statusDays[id] = nil
		}
		for _, day := range days {
			facts.statusDays[day.StudentID] = append(facts.statusDays[day.StudentID], day)
		}
	}
	// Past days and days beyond care end short-circuit before the partial
	// absence read. Do not include those requests in its batch either.
	partialRows := make([]careplan.ExcusedAbsenceRequest, 0, len(rows))
	for _, row := range rows {
		student := students[row.StudentID]
		if absenceBulkEligible(row.Dates, today) && !requestExtendsBeyondCare(&row, &student) {
			partialRows = append(partialRows, row)
		}
	}
	facts.pickups = w.loadAbsenceReviewPickups(ctx, partialRows)
	return facts
}

func (w *ExcusedRequests) loadAbsenceReviewPickups(ctx context.Context, rows []careplan.ExcusedAbsenceRequest) map[int64][]careplan.PickupException {
	ids, from, to := absenceReviewRange(rows)
	if len(ids) < 2 {
		return nil
	}
	pickups, err := w.carePlan.ListPickupExceptions(ctx, careplan.StudentScheduleFilter{StudentIDs: ids, From: from, To: to})
	if err != nil {
		return nil
	}
	byStudent := make(map[int64][]careplan.PickupException, len(ids))
	for _, id := range ids {
		byStudent[id] = nil
	}
	for _, row := range pickups {
		byStudent[row.StudentID] = append(byStudent[row.StudentID], row)
	}
	return byStudent
}

// Dates are stored ascending, as in activeStatusDays/hasManualPartialAbsence.
func absenceReviewRange(rows []careplan.ExcusedAbsenceRequest) ([]int64, careplan.Date, careplan.Date) {
	var ids []int64
	var from, to careplan.Date
	for _, row := range rows {
		if len(row.Dates) == 0 {
			continue
		}
		ids = append(ids, row.StudentID)
		if from.IsZero() || row.Dates[0].Before(from) {
			from = row.Dates[0]
		}
		if to.IsZero() || row.Dates[len(row.Dates)-1].After(to) {
			to = row.Dates[len(row.Dates)-1]
		}
	}
	return uniquePositive(ids), from, to
}

func (facts absenceReviewFacts) statusView(ctx context.Context, w *ExcusedRequests, req *careplan.ExcusedAbsenceRequest) (currentStatusView, error) {
	if days, loaded := facts.statusDays[req.StudentID]; loaded {
		return statusViewFromRows(req, days), nil
	}
	return w.currentStatusView(ctx, req)
}

func (facts absenceReviewFacts) manualPartialAbsence(ctx context.Context, w *ExcusedRequests, studentID int64, dates []careplan.Date) (bool, error) {
	if pickups, loaded := facts.pickups[studentID]; loaded {
		return manualPartialAbsenceInRows(dates, pickups), nil
	}
	return w.hasManualPartialAbsence(ctx, studentID, dates)
}
