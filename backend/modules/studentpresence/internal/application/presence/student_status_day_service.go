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

// GetActiveByStudentIDsAndDate returns the active status rows of many
// students for one calendar date.
func (s *StudentStatusDayService) GetActiveByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]*absencerecords.StudentStatusDay, error) {
	return s.repo.FindActiveByStudentIDsAndDate(ctx, studentIDs, date)
}

// GetSignedOffByStudentIDsAndDate returns the status rows of many students
// that count as a registered sign-off for one date: active rows plus rows the
// end-of-day scheduler archived (see the repository doc for why those stay in).
func (s *StudentStatusDayService) GetSignedOffByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]*absencerecords.StudentStatusDay, error) {
	return s.repo.FindSignedOffByStudentIDsAndDate(ctx, studentIDs, date)
}

// GetActiveByStudentAndDateRange returns a student's active status rows
// within the date range.
func (s *StudentStatusDayService) GetActiveByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*absencerecords.StudentStatusDay, error) {
	return s.repo.FindActiveByStudentAndDateRange(ctx, studentID, startDate, endDate)
}

// GetByStudentAndDateRange returns ALL (including cleared) status rows of
// a student within the date range.
func (s *StudentStatusDayService) GetByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*absencerecords.StudentStatusDay, error) {
	return s.repo.FindByStudentAndDateRange(ctx, studentID, startDate, endDate)
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

// MarkCleared clears a student's status for one date.
func (s *StudentStatusDayService) MarkCleared(ctx context.Context, studentID int64, status string, date timezone.Date, clearedAt time.Time, source string) error {
	return s.repo.MarkCleared(ctx, studentID, status, date, clearedAt, source)
}
