package presence

import (
	"context"
	"errors"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// StudentStatusDayService owns status-day persistence and the absence-overview
// data assembly. Simple persistence operations return repository results and
// errors verbatim; GetOverview also joins roster and person data and applies
// enrollment eligibility for each row.
type StudentStatusDayService struct {
	repo             StudentStatusDayRepository
	pickupExceptions ManualPartialAbsenceReader
	db               DatabaseHandle
	now              func() time.Time
	lockExceptionDay func(context.Context, int64, string) error
}

// NewStudentStatusDayServiceWithPartialAbsences also prevents a full-day
// status from silently overwriting a time-specific excusal on the same date.
func NewStudentStatusDayServiceWithPartialAbsences(
	repo StudentStatusDayRepository,
	pickupExceptions ManualPartialAbsenceReader,
	db DatabaseHandle,
	lockExceptionDay func(context.Context, int64, string) error,
	clocks ...func() time.Time,
) *StudentStatusDayService {
	now := time.Now
	if len(clocks) > 0 && clocks[0] != nil {
		now = clocks[0]
	}
	return &StudentStatusDayService{repo: repo, pickupExceptions: pickupExceptions, db: db, now: now, lockExceptionDay: lockExceptionDay}
}

// UpsertReported records a reported status day.
func (s *StudentStatusDayService) UpsertReported(ctx context.Context, entry *absencerecords.StudentStatusDay) error {
	if entry != nil && s.pickupExceptions != nil {
		if s.db == nil {
			return errors.New("student status day service database is not configured")
		}
		if err := s.lockStatusDate(ctx, entry.StudentID, entry.Date.String()); err != nil {
			return err
		}
		if err := s.ensureNoPartialAbsenceConflicts(ctx, entry.StudentID, []calendar.Date{entry.Date}); err != nil {
			return err
		}
	}
	return s.repo.UpsertReported(ctx, entry)
}

func (s *StudentStatusDayService) lockStatusDate(ctx context.Context, studentID int64, date string) error {
	if s.lockExceptionDay == nil {
		return errors.New("careplanning: exception-day lock is not bound for database")
	}
	return s.lockExceptionDay(ctx, studentID, date)
}

// ValidateProfileStatusChange preserves the explicit-switch contract even
// when a parent/planned/school report has no corresponding live flag, and
// refuses a status on a day a class trip holds.
func (s *StudentStatusDayService) ValidateProfileStatusChange(ctx context.Context, studentID int64, day calendar.Date, sick, excused *bool) error {
	setsSick := sick != nil && *sick
	setsExcused := excused != nil && *excused
	if !setsSick && !setsExcused {
		return nil
	}
	if err := s.lockStatusDate(ctx, studentID, day.String()); err != nil {
		return err
	}
	rows, err := s.repo.FindSignedOffByStudentIDsAndDate(ctx, []int64{studentID}, day)
	if err != nil {
		return err
	}
	standing := make(map[string]bool, len(rows))
	for _, row := range rows {
		if (setsSick && row.Status == absencerecords.StudentStatusDayExcused && (excused == nil || *excused)) ||
			(setsExcused && row.Status == absencerecords.StudentStatusDaySick && (sick == nil || *sick)) {
			return absencerecords.ErrSickExcusedConflict
		}
		standing[row.Status] = true
	}
	if reportsOverClassTrip(standing, setsSick, setsExcused) {
		return absencerecords.ErrStatusDayOccupied
	}
	return nil
}

// reportsOverClassTrip reports whether the profile change newly reports a
// status on a day a class trip holds. The profile has no switch for a class
// trip, so that is refused, as the status-day write refuses it (#6959). A
// status that already stands beside it stays a no-op.
func reportsOverClassTrip(standing map[string]bool, setsSick, setsExcused bool) bool {
	if !standing[absencerecords.StudentStatusDayClassTrip] {
		return false
	}
	return (setsSick && !standing[absencerecords.StudentStatusDaySick]) ||
		(setsExcused && !standing[absencerecords.StudentStatusDayExcused])
}
