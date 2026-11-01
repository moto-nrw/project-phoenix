package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// KeepStudentTemplateDays records "Bleibt so" on the student's hand rows of
// the series (#5102). It changes no plan: the rows keep their days, only the
// booking leftover hint reads the mark. Booking-created rows are never
// marked.
func (s *StudentTemplateDays) KeepStudentTemplateDays(ctx context.Context, keep timetable.StudentTemplateDaysKeep) (int, error) {
	if keep.TemplateID <= 0 {
		return 0, fmt.Errorf("%w: template_id is required", timetable.ErrStudentTemplateDaysInvalid)
	}
	wanted, err := validStudentTemplateDays(keep.StudentID, keep.BookingEndedOn, keep.Weekdays)
	if err != nil {
		return 0, err
	}
	if len(wanted) == 0 {
		return 0, fmt.Errorf("%w: weekdays are required", timetable.ErrStudentTemplateDaysInvalid)
	}
	// The same gate as the removal and the series edits, which copy the
	// decision onto the rows they rewrite: none of them reads a row this
	// decision is about to change.
	if err := s.deps.RecurrenceLock.LockRecurrenceWrites(ctx); err != nil {
		return 0, err
	}
	ids, err := s.keptRowIDs(ctx, keep.TemplateID, keep.StudentID, wanted)
	if err != nil {
		return 0, err
	}
	var through *string
	if keep.Kept {
		text := keep.BookingEndedOn.String()
		through = &text
	}
	if err := s.deps.Enrollments.SetStudentEnrollmentsKeptWithoutBooking(ctx, keep.StudentID, ids, sortedWeekdays(wanted), through); err != nil {
		return 0, fmt.Errorf("student template days: keep hand rows: %w", err)
	}
	return len(ids), nil
}

// keptRowIDs lists every hand row of the student in the series that plans
// one of the weekdays, ended rows included: the hint judges each row on its
// own planned days, so a decision must reach the row of every gap. A mark on
// a row without planned days changes nothing.
func (s *StudentTemplateDays) keptRowIDs(ctx context.Context, templateID, studentID int64, wanted map[int]bool) ([]int64, error) {
	segments, err := s.deps.Groups.ListGroups(ctx, timetable.ActiveTemplateSeries(templateID))
	if err != nil {
		return nil, fmt.Errorf("student template days: list series: %w", err)
	}
	if len(segments) == 0 {
		return nil, timetable.ErrGroupNotFound
	}
	inSeries := make(map[int64]bool, len(segments))
	segmentIDs := make([]int64, 0, len(segments))
	for _, segment := range segments {
		inSeries[segment.ID] = true
		segmentIDs = append(segmentIDs, segment.ID)
	}
	rows, err := studentEnrollments(ctx, s.deps.Enrollments, studentID)
	if err != nil {
		return nil, err
	}
	scheduleDays, err := s.scheduleWeekdays(ctx, segmentIDs)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0)
	for _, row := range rows {
		if row.EnrollmentRequestChildID == nil && inSeries[row.ActivityGroupID] &&
			plansAnyWeekday(rowWeekdays(row, scheduleDays[row.ActivityGroupID]), wanted) {
			ids = append(ids, row.ID)
		}
	}
	return ids, nil
}

func plansAnyWeekday(days, wanted map[int]bool) bool {
	for weekday := range days {
		if wanted[weekday] {
			return true
		}
	}
	return false
}
