package presence

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
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
		if err := s.ensureNoPartialAbsenceConflicts(ctx, entry.StudentID, []timezone.Date{entry.Date}); err != nil {
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
