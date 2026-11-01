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
// the series' planned occurrences on those weekdays. Booking-created rows,
// earlier days, other weekdays and rows recording attendance or a hand
// decision stay.
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
	if result.RosterRows, err = s.narrowHandRows(ctx, rows, inSeries, segmentIDs, wanted, removal.From); err != nil {
		return result, err
	}
	if result.Occurrences, err = s.leaveOccurrences(ctx, rows, inSeries, wanted, removal); err != nil {
		return result, err
	}
	s.deps.Logger.Info("removed student from hand-maintained template weekdays",
		slog.Int64("template_id", removal.TemplateID),
		slog.Int64("student_id", removal.StudentID),
		slog.Any("weekdays", removal.Weekdays),
		slog.String("from", removal.From.String()),
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

// narrowHandRows rewrites every hand row of the series that plans one of the
// weekdays from the date on and returns how many it rewrote.
func (s *StudentTemplateDays) narrowHandRows(
	ctx context.Context, rows []*activityContract.StudentEnrollment, inSeries map[int64]bool, segmentIDs []int64,
	wanted map[int]bool, from calendar.Date,
) (int, error) {
	scheduleDays, err := s.scheduleWeekdays(ctx, segmentIDs)
	if err != nil {
		return 0, err
	}
	narrowed := 0
	for _, row := range reachingRows(rows, from, false) {
		if !inSeries[row.ActivityGroupID] {
			continue
		}
		changed, err := s.narrowHandRow(ctx, row, rowWeekdays(row, scheduleDays[row.ActivityGroupID]), wanted, from)
		if err != nil {
			return narrowed, err
		}
		if changed {
			narrowed++
		}
	}
	return narrowed, nil
}

// narrowHandRow ends the row at the date (deleting it when it starts there or
// later) and continues the weekdays that stay with weekday-scoped rows.
func (s *StudentTemplateDays) narrowHandRow(
	ctx context.Context, row *activityContract.StudentEnrollment, days, wanted map[int]bool, from calendar.Date,
) (bool, error) {
	remaining := make([]int, 0, len(days))
	removes := false
	for _, weekday := range sortedWeekdays(days) {
		if wanted[weekday] {
			removes = true
			continue
		}
		remaining = append(remaining, weekday)
	}
	if !removes {
		return false, nil
	}
	start := calendar.Date(row.ValidFrom)
	if start.Before(from) {
		if err := s.deps.Enrollments.SetStudentEnrollmentValidUntil(ctx, row.ID, from.String()); err != nil {
			return false, fmt.Errorf("student template days: end hand row: %w", err)
		}
		start = from
	} else if err := s.deps.Enrollments.DeleteStudentEnrollment(ctx, row.ID); err != nil {
		return false, fmt.Errorf("student template days: delete hand row: %w", err)
	}
	for _, weekday := range remaining {
		if err := s.continueHandRow(ctx, row, start, weekday); err != nil {
			return false, err
		}
	}
	return true, nil
}

func (s *StudentTemplateDays) continueHandRow(ctx context.Context, row *activityContract.StudentEnrollment, start calendar.Date, weekday int) error {
	var validUntil *string
	if row.ValidUntil != nil {
		text := calendar.Date(*row.ValidUntil).String()
		validUntil = &text
	}
	var kept *string
	if row.KeptWithoutBookingThrough != nil {
		text := row.KeptWithoutBookingThrough.String()
		kept = &text
	}
	scoped := weekday
	_, err := s.deps.Enrollments.CreateStudentEnrollment(ctx, timetable.StudentEnrollmentInput{
		StudentID: row.StudentID, ActivityGroupID: row.ActivityGroupID,
		ValidFrom: start.String(), ValidUntil: validUntil,
		CalendarPeriodID: row.CalendarPeriodID, AttendanceStatus: row.AttendanceStatus, Weekday: &scoped,
		KeptWithoutBookingThrough: kept,
	})
	if err != nil {
		return fmt.Errorf("student template days: continue hand row: %w", err)
	}
	return nil
}

// leaveOccurrences removes the student from the series' planned occurrences
// on the weekdays, keeping rows a booking still plans and rows that record
// more than a plan.
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
