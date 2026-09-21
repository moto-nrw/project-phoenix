package presenceservice

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/application/presence"
)

// NewStatusDays builds the status-day reads and writes over Care Plan's
// status-day adapter; a full-day status never silently overwrites a
// time-specific excusal on the same date.
func NewStatusDays(repo StudentStatusDayRepository, pickupExceptions ManualPartialAbsenceReader, db studentpresence.DatabaseHandle, lockExceptionDay LockExceptionDay, clocks ...func() time.Time) studentpresence.StatusDays {
	return statusDays{
		StudentStatusDayService: presence.NewStudentStatusDayServiceWithPartialAbsences(repo, pickupExceptions, db, lockExceptionDay, clocks...),
		repo:                    repo,
	}
}

// statusDays publishes the status-day capability. The reads and the single
// clear are Care Plan's adapter operations under the capability's names; the
// guarded writes run through the application service.
type statusDays struct {
	*presence.StudentStatusDayService
	repo StudentStatusDayRepository
}

func (s statusDays) DeleteStatusDay(ctx context.Context, wc studentpresence.StatusDayWriteContext, statusDayID, studentID int64) error {
	return s.DeleteByID(ctx, wc, statusDayID, studentID)
}

// GetActiveByStudentIDsAndDate returns the active status rows of many
// students for one calendar date.
func (s statusDays) GetActiveByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]*presence.StatusDayRow, error) {
	return s.repo.FindActiveByStudentIDsAndDate(ctx, studentIDs, date)
}

// GetSignedOffByStudentIDsAndDate returns the status rows of many students
// that count as a registered sign-off for one date: active rows plus rows the
// end-of-day scheduler archived (see the repository doc for why those stay in).
func (s statusDays) GetSignedOffByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]*presence.StatusDayRow, error) {
	return s.repo.FindSignedOffByStudentIDsAndDate(ctx, studentIDs, date)
}

// GetActiveByStudentAndDateRange returns a student's active status rows
// within the date range.
func (s statusDays) GetActiveByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*presence.StatusDayRow, error) {
	return s.repo.FindActiveByStudentAndDateRange(ctx, studentID, startDate, endDate)
}

// GetByStudentAndDateRange returns ALL (including cleared) status rows of
// a student within the date range.
func (s statusDays) GetByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*presence.StatusDayRow, error) {
	return s.repo.FindByStudentAndDateRange(ctx, studentID, startDate, endDate)
}

// MarkCleared clears a student's status for one date.
func (s statusDays) MarkCleared(ctx context.Context, studentID int64, status string, date timezone.Date, clearedAt time.Time, source string) error {
	return s.repo.MarkCleared(ctx, studentID, status, date, clearedAt, source)
}

// NewStatusDayOverviews builds the tenant-wide absence overview.
func NewStatusDayOverviews(repo StudentStatusDayOverviewRepository, people StatusDayOverviewPeople) studentpresence.StatusDayOverviews {
	return presence.NewStudentStatusDayOverviewService(repo, people)
}
