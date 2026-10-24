package presence

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// NewStatusDays builds the status-day reads and writes over Care Plan's
// status-day adapter; a full-day status never silently overwrites a
// time-specific excusal on the same date.
func NewStatusDays(repo StudentStatusDayRepository, pickupExceptions ManualPartialAbsenceReader, db studentpresence.DatabaseHandle, lockExceptionDay LockExceptionDay, clocks ...func() time.Time) studentpresence.StatusDays {
	return statusDays{
		StudentStatusDayService: NewStudentStatusDayServiceWithPartialAbsences(repo, pickupExceptions, db, lockExceptionDay, clocks...),
		statusDayRecords:        repo,
	}
}

// statusDays publishes the status-day capability. The reads and the single
// clear are Care Plan's adapter operations under their own names; the
// guarded writes run through the application service.
type statusDays struct {
	*StudentStatusDayService
	statusDayRecords
}

// statusDayRecords is the part of the status-day port the capability
// publishes unchanged.
type statusDayRecords interface {
	studentpresence.StatusDayReads
	MarkCleared(ctx context.Context, studentID int64, status string, date calendar.Date, clearedAt time.Time, source string) error
}

func (s statusDays) DeleteStatusDay(ctx context.Context, wc studentpresence.StatusDayWriteContext, statusDayID, studentID int64) error {
	return s.DeleteByID(ctx, wc, statusDayID, studentID)
}

// NewStatusDayOverviews builds the tenant-wide absence overview.
func NewStatusDayOverviews(repo StudentStatusDayOverviewRepository, people StatusDayOverviewPeople) studentpresence.StatusDayOverviews {
	return NewStudentStatusDayOverviewService(repo, people)
}
