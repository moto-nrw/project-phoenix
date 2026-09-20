package careplan

import (
	"context"
	"time"

	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ArrivalScheduleService is the composed schedule capability used at wiring boundaries.
// Consumers that only need plans, exceptions, notes, or effective times can
// depend on those capabilities independently.
type ArrivalScheduleService interface {
	ArrivalPlans
	ArrivalExceptions
	ArrivalNotes
	ArrivalTimes
	ClassArrivalExceptions
}

// ArrivalPlans reads and updates the recurring weekly plan.
type ArrivalPlans interface {
	BulkUpsertArrivalSchedules(context.Context, ArrivalScheduleBulkFilter, []ArrivalScheduleInput, int64) (*BulkUpsertResult, error)
	GetClassArrivalTimes(context.Context, string) (*ClassArrivalTimes, error)
	GetStudentArrivalSchedules(ctx context.Context, studentID int64) ([]*ArrivalSchedule, error)
	GetWeeklySchedulesByStudentIDsAndWeekday(ctx context.Context, studentIDs []int64, weekday int) ([]*ArrivalSchedule, error)
	// GetWeeklySchedulesByStudentIDsForDate returns the full recurring arrival
	// plan applicable on date in one batch projection.
	GetWeeklySchedulesByStudentIDsForDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]*ArrivalSchedule, error)
	GetStudentArrivalScheduleForWeekday(ctx context.Context, studentID int64, weekday int) (*ArrivalSchedule, error)
	UpsertStudentArrivalSchedule(ctx context.Context, scheduleData *ArrivalSchedule) error
	UpsertBulkStudentArrivalSchedules(ctx context.Context, studentID int64, schedules []*ArrivalSchedule) error
	DeleteStudentArrivalSchedule(ctx context.Context, scheduleID int64) error
	DeleteAllStudentArrivalSchedules(ctx context.Context, studentID int64) error
}

// ArrivalExceptions manages dated deviations from the weekly plan.
type ArrivalExceptions interface {
	GetStudentArrivalExceptionByID(ctx context.Context, exceptionID int64) (*ArrivalException, error)
	GetStudentArrivalExceptionForDate(ctx context.Context, studentID int64, date timezone.Date) (*ArrivalException, error)
	GetStudentArrivalExceptions(ctx context.Context, studentID int64) ([]*ArrivalException, error)
	GetUpcomingStudentArrivalExceptions(ctx context.Context, studentID int64) ([]*ArrivalException, error)
	CreateStudentArrivalException(ctx context.Context, exception *ArrivalException) error
	UpdateStudentArrivalException(ctx context.Context, exception *ArrivalException) error
	DeleteStudentArrivalException(ctx context.Context, exceptionID, studentID int64) error
	DeleteAllStudentArrivalExceptions(ctx context.Context, studentID int64) error
	CreateOrReclaimException(ctx context.Context, studentID int64, date timezone.Date, arrivalTime *time.Time, reason *string, staffID int64, resolveStaffID func() (int64, error)) (*ArrivalException, error)
	UpdateException(ctx context.Context, exceptionID, studentID int64, date timezone.Date, reason *string, arrivalTime *time.Time, clearArrivalTime bool, resolveStaffID func() (int64, error)) (*ArrivalException, error)
}

// ArrivalNotes manages the notes attached to a student's care days.
type ArrivalNotes interface {
	GetStudentArrivalNoteByID(ctx context.Context, noteID int64) (*ArrivalNote, error)
	GetStudentArrivalNotes(ctx context.Context, studentID int64) ([]*ArrivalNote, error)
	GetStudentArrivalNotesForDate(ctx context.Context, studentID int64, date timezone.Date) ([]*ArrivalNote, error)
	CreateStudentArrivalNote(ctx context.Context, note *ArrivalNote) error
	UpdateStudentArrivalNote(ctx context.Context, note *ArrivalNote) error
	DeleteStudentArrivalNote(ctx context.Context, noteID int64) error
	DeleteAllStudentArrivalNotes(ctx context.Context, studentID int64) error
}

// ArrivalTimes resolves the effective plan and its dated presentation.
type ArrivalTimes interface {
	BulkArrivalTimes
	GetStudentArrivalData(ctx context.Context, studentID int64) (*StudentArrivalData, error)
	GetStudentArrivalDataForDate(ctx context.Context, studentID int64, date timezone.Date) (*StudentArrivalData, error)
	GetStudentArrivalDataForDateRange(ctx context.Context, studentID int64, from, to timezone.Date) (*StudentArrivalData, error)
	GetStudentsWithStoredArrivalSchedules(ctx context.Context, studentIDs []int64) (map[int64]bool, error)
	GetEffectiveArrivalTimeForDate(ctx context.Context, studentID int64, date timezone.Date) (*EffectiveArrivalTime, error)
}

// BulkArrivalTimes is the tenant-scoped effective-time projection used by dashboards.
type BulkArrivalTimes interface {
	GetBulkEffectiveArrivalTimesForDate(context.Context, []int64, timezone.Date) (map[int64]*EffectiveArrivalTime, error)
}

type StudentArrivalData struct {
	Schedules  []*ArrivalSchedule  `json:"schedules"`
	Exceptions []*ArrivalException `json:"exceptions"`
	Notes      []*ArrivalNote      `json:"notes"`
}

type ArrivalScheduleInput struct {
	Weekday     int    `json:"weekday"`
	ArrivalTime string `json:"expected_arrival"`
}

// ArrivalScheduleBulkFilter selects exactly one class, OGS group, or explicit
// student list. A whole class updates its timetable, not per-child deviations.
type ArrivalScheduleBulkFilter struct {
	SchoolClass string
	GroupID     int64
	StudentIDs  []int64
	Authorize   func(context.Context, ScheduleStudent) (bool, error)
}

type ClassArrivalTimes struct {
	SchoolClass string            `json:"school_class"`
	Times       map[string]string `json:"times"`
	UpdatedAt   *time.Time        `json:"updated_at,omitempty"`
}
