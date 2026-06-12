package active

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
)

// StudentStatusDayService exposes student status-day persistence to the api
// layer (issue #584: handlers must not hold repositories). CONTRACT:
// repository results and errors are returned VERBATIM — the handlers keep
// their transaction wrappers, mutual-exclusion logic, and live-flag updates,
// so behaviour is byte-identical.
type StudentStatusDayService interface {
	// GetActiveByStudentIDsAndDate returns the active status rows of many
	// students for one calendar date.
	GetActiveByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]*activeModels.StudentStatusDay, error)

	// GetActiveByStudentAndDateRange returns a student's active status rows
	// within the date range.
	GetActiveByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*activeModels.StudentStatusDay, error)

	// GetByStudentAndDateRange returns ALL (including cleared) status rows of
	// a student within the date range.
	GetByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*activeModels.StudentStatusDay, error)

	// GetActiveByID returns the active status row with the given ID.
	GetActiveByID(ctx context.Context, id int64) (*activeModels.StudentStatusDay, error)

	// UpsertReported records a reported status day.
	UpsertReported(ctx context.Context, entry *activeModels.StudentStatusDay) error

	// MarkCleared clears a student's status for one date.
	MarkCleared(ctx context.Context, studentID int64, status string, date timezone.Date, clearedAt time.Time, source string) error

	// MarkClearedByID clears the status row with the given ID.
	MarkClearedByID(ctx context.Context, id int64, clearedAt time.Time, source string) error

	// MarkClearedForDates clears a student's status for several dates.
	MarkClearedForDates(ctx context.Context, studentID int64, status string, dates []timezone.Date, clearedAt time.Time, source string) error
}

type studentStatusDayService struct {
	repo activeModels.StudentStatusDayRepository
}

// NewStudentStatusDayService creates a StudentStatusDayService backed by the
// status-day repository.
func NewStudentStatusDayService(repo activeModels.StudentStatusDayRepository) StudentStatusDayService {
	return &studentStatusDayService{repo: repo}
}

func (s *studentStatusDayService) GetActiveByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]*activeModels.StudentStatusDay, error) {
	return s.repo.FindActiveByStudentIDsAndDate(ctx, studentIDs, date)
}

func (s *studentStatusDayService) GetActiveByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*activeModels.StudentStatusDay, error) {
	return s.repo.FindActiveByStudentAndDateRange(ctx, studentID, startDate, endDate)
}

func (s *studentStatusDayService) GetByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*activeModels.StudentStatusDay, error) {
	return s.repo.FindByStudentAndDateRange(ctx, studentID, startDate, endDate)
}

func (s *studentStatusDayService) GetActiveByID(ctx context.Context, id int64) (*activeModels.StudentStatusDay, error) {
	return s.repo.FindActiveByID(ctx, id)
}

func (s *studentStatusDayService) UpsertReported(ctx context.Context, entry *activeModels.StudentStatusDay) error {
	return s.repo.UpsertReported(ctx, entry)
}

func (s *studentStatusDayService) MarkCleared(ctx context.Context, studentID int64, status string, date timezone.Date, clearedAt time.Time, source string) error {
	return s.repo.MarkCleared(ctx, studentID, status, date, clearedAt, source)
}

func (s *studentStatusDayService) MarkClearedByID(ctx context.Context, id int64, clearedAt time.Time, source string) error {
	return s.repo.MarkClearedByID(ctx, id, clearedAt, source)
}

func (s *studentStatusDayService) MarkClearedForDates(ctx context.Context, studentID int64, status string, dates []timezone.Date, clearedAt time.Time, source string) error {
	return s.repo.MarkClearedForDates(ctx, studentID, status, dates, clearedAt, source)
}
