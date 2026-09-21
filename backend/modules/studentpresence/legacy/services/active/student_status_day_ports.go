package active

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
)

// StudentStatusDayRepository is the status-day port of the presence services.
// Care Plan persists broad day statuses (sick / excused / class trip) and
// CASCADES them into per-slot attendance: UpsertReported and the MarkCleared*
// methods also apply/release the status on matching
// schedule.instance_students rows (see #1913), so no write path skips it.
type StudentStatusDayRepository interface {
	// UpsertReported inserts or refreshes a reported status day AND marks the
	// student's still-expected slots on that date absent with status-day
	// provenance (schedule.instance_students.student_status_day_id).
	UpsertReported(ctx context.Context, entry *absencerecords.StudentStatusDay) error
	// ArchiveAndClearStatusFlag archives a legacy boolean student flag into
	// student_status_days for the date and clears the flag on
	// users.students. Returns the number of students cleared. Column names
	// must be trusted constants, never user input.
	ArchiveAndClearStatusFlag(ctx context.Context, flagColumn, sinceColumn, status string, date timezone.Date, reportedFallback time.Time, source string) (int64, error)
	// CountEffectiveDashboardAbsences counts today's effective dashboard
	// absence buckets from live flags and status-day rows, applying the same
	// precedence as student responses: sick wins, class trip counts as excused.
	CountEffectiveDashboardAbsences(ctx context.Context, date timezone.Date) (*absencerecords.StudentStatusCounts, error)
	// MarkCleared / MarkClearedByID / MarkClearedForDates clear status days
	// AND release the cascade: slot absences owned by the cleared status day
	// revert to the latest remaining active status for that date, or back to
	// expected (absent when the instance already completed).
	MarkCleared(ctx context.Context, studentID int64, status string, date timezone.Date, clearedAt time.Time, source string) error
	MarkClearedByID(ctx context.Context, id int64, clearedAt time.Time, source string) error
	MarkClearedForDates(ctx context.Context, studentID int64, status string, dates []timezone.Date, clearedAt time.Time, source string) error
	FindActiveByID(ctx context.Context, id int64) (*absencerecords.StudentStatusDay, error)
	FindActiveByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*absencerecords.StudentStatusDay, error)
	FindActiveByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]*absencerecords.StudentStatusDay, error)
	// FindSignedOffByStudentIDsAndDate returns active rows plus end-of-day
	// archived rows (source = "end_of_day") for the date — the full set of
	// valid registered sign-offs for that day.
	FindSignedOffByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]*absencerecords.StudentStatusDay, error)
	FindByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*absencerecords.StudentStatusDay, error)
}

// StudentStatusDayOverviewRepository adds the ordered read required by the
// paginated absence overview.
type StudentStatusDayOverviewRepository interface {
	StudentStatusDayRepository
	ListOverviewWithOptions(ctx context.Context, options *modelBase.QueryOptions, orderedStudentIDs []int64) ([]*absencerecords.StudentStatusDay, error)
	CountWithOptions(ctx context.Context, options *modelBase.QueryOptions) (int, error)
}
