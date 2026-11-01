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
	segments, err := s.deps.Groups.ListGroups(ctx, timetable.ActiveTemplateSeries(keep.TemplateID))
	if err != nil {
		return 0, fmt.Errorf("student template days: list series: %w", err)
	}
	if len(segments) == 0 {
		return 0, timetable.ErrGroupNotFound
	}
	inSeries := make(map[int64]bool, len(segments))
	segmentIDs := make([]int64, 0, len(segments))
	for _, segment := range segments {
		inSeries[segment.ID] = true
		segmentIDs = append(segmentIDs, segment.ID)
	}
	rows, err := studentEnrollments(ctx, s.deps.Enrollments, keep.StudentID)
	if err != nil {
		return 0, err
	}
	scheduleDays, err := s.scheduleWeekdays(ctx, segmentIDs)
	if err != nil {
		return 0, err
	}
	ids := make([]int64, 0)
	// Rows reaching the booking end; a mark on a row that ended since then
	// changes nothing, so the decision needs no clock.
	for _, row := range reachingRows(rows, keep.BookingEndedOn, false) {
		if inSeries[row.ActivityGroupID] && plansAnyWeekday(rowWeekdays(row, scheduleDays[row.ActivityGroupID]), wanted) {
			ids = append(ids, row.ID)
		}
	}
	var through *string
	if keep.Kept {
		text := keep.BookingEndedOn.String()
		through = &text
	}
	if err := s.deps.Enrollments.SetStudentEnrollmentsKeptWithoutBooking(ctx, keep.StudentID, ids, through); err != nil {
		return 0, fmt.Errorf("student template days: keep hand rows: %w", err)
	}
	return len(ids), nil
}

func plansAnyWeekday(days, wanted map[int]bool) bool {
	for weekday := range days {
		if wanted[weekday] {
			return true
		}
	}
	return false
}
