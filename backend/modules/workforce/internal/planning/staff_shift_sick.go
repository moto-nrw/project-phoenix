package planning

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// A single shift write planned after a sick report takes the state the sick
// cascade gives the shifts that existed when the report came in (#6302): a
// created or moved shift on a full sick day of its staff member is cancelled
// with the report's stamp, and a sick person cannot cover a gap. Past days
// stay history, as the cascade leaves them.

// WithStaffShiftSickReports binds the sick reports a created or moved shift
// takes its day's state from. Without it (unit tests) nobody is sick.
func WithStaffShiftSickReports(reader SickReportReader) StaffShiftOption {
	return func(s *staffShiftService) { s.sickReports = reader }
}

// WithStaffShiftToday overrides the calendar-day clock the sick-state guard
// compares against. A nil clock keeps the default.
func WithStaffShiftToday(today func() calendar.Date) StaffShiftOption {
	return func(s *staffShiftService) {
		if today != nil {
			s.today = today
		}
	}
}

func (s *staffShiftService) todayDate() calendar.Date {
	if s.today == nil {
		return calendar.TodayDate()
	}
	return s.today()
}

// sickAbsenceOn names the sick report that makes day a full sick day of the
// staff member, zero when there is none or the day is past. The caller holds
// the staff member's shift lock.
func (s *staffShiftService) sickAbsenceOn(ctx context.Context, staffID int64, day calendar.Date) (int64, error) {
	if s.sickReports == nil || day.Before(s.todayDate()) {
		return 0, nil
	}
	days, err := sickDaysIn(ctx, s.sickReports, staffID, day, day)
	if err != nil {
		return 0, err
	}
	return days[day], nil
}

// applyEditedSickState gives a shift whose edit changes its day the sick
// state of the new day, like a move. A shift the cascade cancelled leaves
// that state behind first, so it is planned again on a healthy day. A
// planner's own cancellation stays. An edit on the same day takes ownership
// of the shift; an ordinary edit that changes nothing, such as a retried
// request, keeps the sick report's stamp. ordinaryEdit is false for
// ApplyCancellation, whose cover planning is always the planner's decision.
func (s *staffShiftService) applyEditedSickState(ctx context.Context, shift, existing *workforce.ShiftRecord, ordinaryEdit bool) error {
	if shift.OriginShiftID != nil {
		return nil
	}
	if shift.Date == existing.Date {
		if ordinaryEdit && existing.SickAbsenceID != nil && editChangesNothing(shift, existing) {
			shift.SickAbsenceID = existing.SickAbsenceID
			shift.ChangeReason = existing.ChangeReason
			shift.SeriesOverride = existing.SeriesOverride
		}
		return nil
	}
	if existing.SickAbsenceID != nil {
		releaseSickState(shift)
	}
	return s.applySickStateOfDay(ctx, shift)
}

// editChangesNothing reports whether an edit of a sick-stamped shift writes
// the stored values again. Such a shift carries the server's sick label, so a
// request that leaves the reason empty, as the edit dialog does, matches it.
func editChangesNothing(shift, existing *workforce.ShiftRecord) bool {
	return !staffShiftMoveChanged(existing, shift) &&
		shift.Notes == existing.Notes &&
		shift.Cancelled == existing.Cancelled &&
		(sameOptionalText(shift.ChangeReason, existing.ChangeReason) || requestsNoReason(shift.ChangeReason))
}

func requestsNoReason(requested *string) bool {
	return requested == nil || *requested == ""
}

func sameOptionalText(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// releaseSickState undoes applySickReport: the shift is planned again and
// drops the sick label, as the cascade's reactivation does.
func releaseSickState(shift *workforce.ShiftRecord) {
	shift.Cancelled = false
	shift.SickAbsenceID = nil
	if shift.ChangeReason != nil && *shift.ChangeReason == workforce.SickShiftChangeReason {
		shift.ChangeReason = nil
	}
}

// applySickStateOfDay gives a newly placed shift the sick state of its day.
// A cover by a sick person is refused, as the Betreuungsplan refuses a sick
// substitute. A plain shift on a full sick day is cancelled with the
// report's stamp. A shift the caller already cancels keeps that decision and
// its reason, as the cascade leaves an admin's cancellation alone.
func (s *staffShiftService) applySickStateOfDay(ctx context.Context, shift *workforce.ShiftRecord) error {
	if shift.Cancelled && shift.OriginShiftID == nil {
		return nil
	}
	absenceID, err := s.sickAbsenceOn(ctx, shift.StaffID, shift.Date)
	if err != nil || absenceID == 0 {
		return err
	}
	if shift.OriginShiftID != nil {
		return withReason(fmt.Errorf("%w: replacement staff member is absent on %s", ErrShiftInvalid, shift.Date), workforce.ErrReplacementStaffAbsent)
	}
	applySickReport(shift, absenceID)
	return nil
}
