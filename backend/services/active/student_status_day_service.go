package active

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/careplanning"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/uptrace/bun"
)

// StudentStatusDayService owns status-day persistence and the absence-overview
// data assembly. Simple persistence operations return repository results and
// errors verbatim; GetOverview also joins roster and person data and applies
// enrollment eligibility for each row.
type StudentStatusDayService struct {
	repo             activeModels.StudentStatusDayRepository
	pickupExceptions scheduleModels.StudentPickupExceptionRepository
	db               *bun.DB
	people           statusDayOverviewPeople
	overviewRepo     statusDayOverviewRepository
}

type statusDayOverviewRepository interface {
	ListOverviewWithOptions(ctx context.Context, options *modelBase.QueryOptions) ([]*activeModels.StudentStatusDay, error)
	CountWithOptions(ctx context.Context, options *modelBase.QueryOptions) (int, error)
}

// NewStudentStatusDayService creates a StudentStatusDayService backed by the
// status-day repository.
func NewStudentStatusDayService(repo activeModels.StudentStatusDayRepository) *StudentStatusDayService {
	overviewRepo, _ := repo.(statusDayOverviewRepository)
	return &StudentStatusDayService{repo: repo, overviewRepo: overviewRepo}
}

// NewStudentStatusDayServiceWithPartialAbsences also prevents a full-day
// status from silently overwriting a time-specific excusal on the same date.
func NewStudentStatusDayServiceWithPartialAbsences(
	repo activeModels.StudentStatusDayRepository,
	pickupExceptions scheduleModels.StudentPickupExceptionRepository,
	db *bun.DB,
	people statusDayOverviewPeople,
) *StudentStatusDayService {
	overviewRepo, _ := repo.(statusDayOverviewRepository)
	return &StudentStatusDayService{repo: repo, pickupExceptions: pickupExceptions, db: db, people: people, overviewRepo: overviewRepo}
}

// GetActiveByStudentIDsAndDate returns the active status rows of many
// students for one calendar date.
func (s *StudentStatusDayService) GetActiveByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]*activeModels.StudentStatusDay, error) {
	return s.repo.FindActiveByStudentIDsAndDate(ctx, studentIDs, date)
}

// GetSignedOffByStudentIDsAndDate returns the status rows of many students
// that count as a registered sign-off for one date: active rows plus rows the
// end-of-day scheduler archived (see the repository doc for why those stay in).
func (s *StudentStatusDayService) GetSignedOffByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]*activeModels.StudentStatusDay, error) {
	return s.repo.FindSignedOffByStudentIDsAndDate(ctx, studentIDs, date)
}

// GetActiveByStudentAndDateRange returns a student's active status rows
// within the date range.
func (s *StudentStatusDayService) GetActiveByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*activeModels.StudentStatusDay, error) {
	return s.repo.FindActiveByStudentAndDateRange(ctx, studentID, startDate, endDate)
}

// GetByStudentAndDateRange returns ALL (including cleared) status rows of
// a student within the date range.
func (s *StudentStatusDayService) GetByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*activeModels.StudentStatusDay, error) {
	return s.repo.FindByStudentAndDateRange(ctx, studentID, startDate, endDate)
}

// GetActiveByID returns the active status row with the given ID.
func (s *StudentStatusDayService) GetActiveByID(ctx context.Context, id int64) (*activeModels.StudentStatusDay, error) {
	return s.repo.FindActiveByID(ctx, id)
}

// UpsertReported records a reported status day.
func (s *StudentStatusDayService) UpsertReported(ctx context.Context, entry *activeModels.StudentStatusDay) error {
	if entry != nil && s.pickupExceptions != nil {
		if s.db == nil {
			return errors.New("student status day service database is not configured")
		}
		if err := careplanning.LockExceptionDay(ctx, s.db, entry.StudentID, entry.Date); err != nil {
			return err
		}
		if err := s.ensureNoPartialAbsenceConflicts(ctx, entry.StudentID, []timezone.Date{entry.Date}); err != nil {
			return err
		}
	}
	return s.repo.UpsertReported(ctx, entry)
}

// MarkCleared clears a student's status for one date.
func (s *StudentStatusDayService) MarkCleared(ctx context.Context, studentID int64, status string, date timezone.Date, clearedAt time.Time, source string) error {
	return s.repo.MarkCleared(ctx, studentID, status, date, clearedAt, source)
}

// MarkClearedByID clears the status row with the given ID.
func (s *StudentStatusDayService) MarkClearedByID(ctx context.Context, id int64, clearedAt time.Time, source string) error {
	return s.repo.MarkClearedByID(ctx, id, clearedAt, source)
}

// MarkClearedForDates clears a student's status for several dates.
func (s *StudentStatusDayService) MarkClearedForDates(ctx context.Context, studentID int64, status string, dates []timezone.Date, clearedAt time.Time, source string) error {
	return s.repo.MarkClearedForDates(ctx, studentID, status, dates, clearedAt, source)
}
