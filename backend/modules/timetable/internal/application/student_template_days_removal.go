package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// RemoveStudentTemplateDays takes the student off the series on the weekdays
// from the date on: hand roster rows end at the date and continue with one
// weekday-scoped row per remaining weekday (the editor-owned scope of #2129,
// so the Regeltermin editor keeps maintaining them), and the student leaves
// the series' planned occurrences on those weekdays. With an end (#5497) the
// removed weekdays continue the same way from the end on, and occurrences
// from then stay. Booking-created rows, earlier days, other weekdays and rows
// recording attendance or a hand decision stay.
func (s *StudentTemplateDays) RemoveStudentTemplateDays(ctx context.Context, removal timetable.StudentTemplateDaysRemoval) (timetable.StudentTemplateDaysRemoved, error) {
	wanted, err := s.validRemoval(removal)
	if err != nil {
		return timetable.StudentTemplateDaysRemoved{}, err
	}
	// Today has begun: its occurrences and rows stay, the removal starts
	// tomorrow at the earliest (#5102).
	if tomorrow := s.deps.Today().AddDays(1); removal.From.Before(tomorrow) {
		removal.From = tomorrow
	}
	if removal.Until != nil && !removal.Until.After(removal.From) {
		return timetable.StudentTemplateDaysRemoved{}, fmt.Errorf("%w: until must lie after from, tomorrow at the earliest", timetable.ErrStudentTemplateDaysInvalid)
	}
	if err := s.deps.RecurrenceLock.LockRecurrenceWrites(ctx); err != nil {
		return timetable.StudentTemplateDaysRemoved{}, err
	}
	segments, err := s.deps.Groups.ListGroups(ctx, timetable.ActiveTemplateSeries(removal.TemplateID))
	if err != nil {
		return timetable.StudentTemplateDaysRemoved{}, fmt.Errorf("student template days: list series: %w", err)
	}
	if len(segments) == 0 {
		return timetable.StudentTemplateDaysRemoved{}, timetable.ErrGroupNotFound
	}
	inSeries := make(map[int64]bool, len(segments))
	segmentIDs := make([]int64, 0, len(segments))
	for _, segment := range segments {
		inSeries[segment.ID] = true
		segmentIDs = append(segmentIDs, segment.ID)
	}
	var result timetable.StudentTemplateDaysRemoved
	rows, err := studentEnrollments(ctx, s.deps.Enrollments, removal.StudentID)
	if err != nil {
		return result, err
	}
	if result.RosterRows, err = s.narrowHandRows(ctx, rows, inSeries, segmentIDs, wanted, removalSpan{from: removal.From, until: removal.Until}); err != nil {
		return result, err
	}
	if result.Occurrences, err = s.leaveOccurrences(ctx, rows, inSeries, wanted, removal); err != nil {
		return result, err
	}
	// The child left the plan: rosters, staffing and the leftover hint of
	// other tabs refetch (#5102).
	announceStaffingChanged(ctx, s.deps.Staffing, s.deps.Logger, "student_template_days_remove")
	s.deps.Logger.Info("removed student from hand-maintained template weekdays",
		slog.Int64("template_id", removal.TemplateID),
		slog.Int64("student_id", removal.StudentID),
		slog.Any("weekdays", removal.Weekdays),
		slog.String("from", removal.From.String()),
		slog.String("until", openDateText(removal.Until)),
		slog.Int("roster_rows", result.RosterRows),
		slog.Int("occurrences", result.Occurrences),
	)
	return result, nil
}

func (s *StudentTemplateDays) validRemoval(removal timetable.StudentTemplateDaysRemoval) (map[int]bool, error) {
	if removal.TemplateID <= 0 {
		return nil, fmt.Errorf("%w: template_id is required", timetable.ErrStudentTemplateDaysInvalid)
	}
	wanted, err := validStudentTemplateDays(removal.StudentID, removal.From, removal.Weekdays)
	if err != nil {
		return nil, err
	}
	if len(wanted) == 0 {
		return nil, fmt.Errorf("%w: weekdays are required", timetable.ErrStudentTemplateDaysInvalid)
	}
	if removal.From.Before(s.deps.Today()) {
		return nil, fmt.Errorf("%w: from must not be in the past", timetable.ErrStudentTemplateDaysInvalid)
	}
	return wanted, nil
}

// removalSpan is the days a removal takes the student off: from (inclusive)
// until (exclusive; nil is open).
type removalSpan struct {
	from  calendar.Date
	until *calendar.Date
}

// endsAfter reports whether the span is still open on the date.
func (span removalSpan) endsAfter(date calendar.Date) bool {
	return span.until == nil || date.Before(*span.until)
}

// narrowHandRows rewrites every hand row of the series that plans one of the
// weekdays within the span and returns how many it rewrote.
func (s *StudentTemplateDays) narrowHandRows(
	ctx context.Context, rows []*activityContract.StudentEnrollment, inSeries map[int64]bool, segmentIDs []int64,
	wanted map[int]bool, span removalSpan,
) (int, error) {
	scheduleDays, err := s.scheduleWeekdays(ctx, segmentIDs)
	if err != nil {
		return 0, err
	}
	narrowed := 0
	for _, row := range reachingRows(rows, span.from, false) {
		// A row that starts on or after the end plans only days the removal
		// keeps.
		if !inSeries[row.ActivityGroupID] || !span.endsAfter(calendar.Date(row.ValidFrom)) {
			continue
		}
		changed, err := s.narrowHandRow(ctx, row, rowWeekdays(row, scheduleDays[row.ActivityGroupID]), wanted, span)
		if err != nil {
			return narrowed, err
		}
		if changed {
			narrowed++
		}
	}
	return narrowed, nil
}

// narrowHandRow ends the row at the span's start (deleting it when it starts
// there or later), continues the weekdays that stay with weekday-scoped rows
// and, after a bounded span, the removed weekdays from its end on.
func (s *StudentTemplateDays) narrowHandRow(
	ctx context.Context, row *activityContract.StudentEnrollment, days, wanted map[int]bool, span removalSpan,
) (bool, error) {
	var remaining, removed []int
	for _, weekday := range sortedWeekdays(days) {
		if wanted[weekday] {
			removed = append(removed, weekday)
			continue
		}
		remaining = append(remaining, weekday)
	}
	if len(removed) == 0 {
		return false, nil
	}
	start := calendar.Date(row.ValidFrom)
	if start.Before(span.from) {
		if err := s.deps.Enrollments.SetStudentEnrollmentValidUntil(ctx, row.ID, span.from.String()); err != nil {
			return false, fmt.Errorf("student template days: end hand row: %w", err)
		}
		start = span.from
	} else if err := s.deps.Enrollments.DeleteStudentEnrollment(ctx, row.ID); err != nil {
		return false, fmt.Errorf("student template days: delete hand row: %w", err)
	}
	for _, weekday := range remaining {
		if err := s.continueHandRow(ctx, row, start, weekday); err != nil {
			return false, err
		}
	}
	return true, s.resumeHandRow(ctx, row, span, removed)
}

// resumeHandRow continues the removed weekdays of the row from the span's end
// on, when the row reaches past it.
func (s *StudentTemplateDays) resumeHandRow(ctx context.Context, row *activityContract.StudentEnrollment, span removalSpan, removed []int) error {
	if span.until == nil || (row.ValidUntil != nil && !calendar.Date(*row.ValidUntil).After(*span.until)) {
		return nil
	}
	for _, weekday := range removed {
		if err := s.continueHandRow(ctx, row, *span.until, weekday); err != nil {
			return err
		}
	}
	return nil
}

func (s *StudentTemplateDays) continueHandRow(ctx context.Context, row *activityContract.StudentEnrollment, start calendar.Date, weekday int) error {
	var validUntil *string
	if row.ValidUntil != nil {
		text := calendar.Date(*row.ValidUntil).String()
		validUntil = &text
	}
	scoped := weekday
	_, err := s.deps.Enrollments.CreateStudentEnrollment(ctx, timetable.StudentEnrollmentInput{
		StudentID: row.StudentID, ActivityGroupID: row.ActivityGroupID,
		ValidFrom: start.String(), ValidUntil: validUntil,
		CalendarPeriodID: row.CalendarPeriodID, AttendanceStatus: row.AttendanceStatus, Weekday: &scoped,
		// The weekday-scoped row keeps that weekday's decision only.
		KeptWithoutBooking: keptWeekdayStrings(row.KeptWithoutBooking, &scoped),
	})
	if err != nil {
		return fmt.Errorf("student template days: continue hand row: %w", err)
	}
	return nil
}

// leaveOccurrences removes the student from the series' planned occurrences
// on the weekdays before the removal's end, keeping rows a booking still plans
// and rows that record more than a plan.
func (s *StudentTemplateDays) leaveOccurrences(
	ctx context.Context, rows []*activityContract.StudentEnrollment, inSeries map[int64]bool, wanted map[int]bool,
	removal timetable.StudentTemplateDaysRemoval,
) (int, error) {
	occurrences, err := s.plannedOccurrences(ctx, removal.StudentID, removal.From)
	if err != nil {
		return 0, err
	}
	left := 0
	for _, occurrence := range occurrences {
		if !inSeries[*occurrence.instance.ActivityGroupID] || !wanted[occurrence.planWeekday] ||
			(removal.Until != nil && !occurrence.instance.Date.Before(*removal.Until)) ||
			!instanceRowIsStillPlanned(occurrence.participant) || bookingPlansOccurrence(rows, occurrence) {
			continue
		}
		if err := deleteLegacyParticipant(ctx, s.deps.Participants.Plans, occurrence.participant.ID); err != nil {
			return left, err
		}
		left++
	}
	return left, nil
}

// openDateText renders an optional end for the log; empty when open.
func openDateText(date *calendar.Date) string {
	if date == nil {
		return ""
	}
	return date.String()
}
