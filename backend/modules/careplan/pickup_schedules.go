package careplan

import (
	"context"
	"time"

	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// PickupScheduleService is the composed schedule capability used at wiring boundaries.
// Consumers that only need plans, exceptions, notes, or effective times can
// depend on those capabilities independently.
type PickupScheduleService interface {
	PickupPlans
	PickupExceptions
	PickupNotes
	PickupTimes
}

// PickupPlans reads and updates the recurring weekly plan.
type PickupPlans interface {
	BulkUpsertPickupSchedules(context.Context, PickupBulkFilter, []PickupScheduleInput, int64) (*BulkUpsertResult, error)
	GetStudentPickupSchedules(ctx context.Context, studentID int64) ([]*PickupSchedule, error)
	GetWeeklySchedulesByStudentIDsAndWeekday(ctx context.Context, studentIDs []int64, weekday int) ([]*PickupSchedule, error)
	// GetWeeklySchedulesByStudentIDs returns every weekday row for the given
	// students as of today (class-roster report, #2290).
	GetWeeklySchedulesByStudentIDs(ctx context.Context, studentIDs []int64) ([]*PickupSchedule, error)
	// GetWeeklySchedulesByStudentIDsForDate returns the full recurring pickup
	// plan applicable on date in one batch projection.
	GetWeeklySchedulesByStudentIDsForDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]*PickupSchedule, error)
	GetStudentPickupScheduleForWeekday(ctx context.Context, studentID int64, weekday int) (*PickupSchedule, error)
	HasBookedOfferingPickupForWeekday(ctx context.Context, studentID int64, weekday int) (bool, error)
	UpsertStudentPickupSchedule(ctx context.Context, scheduleData *PickupSchedule) error
	UpsertBulkStudentPickupSchedules(ctx context.Context, studentID int64, schedules []*PickupSchedule) error
	UpsertBulkStudentPickupSchedulesForDate(ctx context.Context, studentID int64, date timezone.Date, schedules []*PickupSchedule) error
	DeleteStudentPickupSchedule(ctx context.Context, scheduleID int64) error
	DeleteAllStudentPickupSchedules(ctx context.Context, studentID int64) error
}

// PickupExceptions manages dated deviations from the weekly plan.
type PickupExceptions interface {
	GetStudentPickupExceptionByID(ctx context.Context, exceptionID int64) (*PickupException, error)
	GetStudentPickupExceptionForDate(ctx context.Context, studentID int64, date timezone.Date) (*PickupException, error)
	GetStudentPickupExceptions(ctx context.Context, studentID int64) ([]*PickupException, error)
	GetUpcomingStudentPickupExceptions(ctx context.Context, studentID int64) ([]*PickupException, error)
	CreateStudentPickupException(ctx context.Context, exception *PickupException) error
	UpdateStudentPickupException(ctx context.Context, exception *PickupException) error
	DeleteStudentPickupException(ctx context.Context, exceptionID, studentID int64) error
	DeleteAllStudentPickupExceptions(ctx context.Context, studentID int64) error
	CreateOrReclaimException(ctx context.Context, studentID int64, date timezone.Date, pickupTime *time.Time, reason *string, staffID int64, resolveStaffID func() (int64, error)) (*PickupException, error)
	UpdateException(ctx context.Context, exceptionID, studentID int64, date timezone.Date, reason *string, pickupTime *time.Time, clearPickupTime bool, resolveStaffID func() (int64, error)) (*PickupException, error)
}

// PickupNotes manages the notes attached to a student's care days.
type PickupNotes interface {
	GetStudentPickupNoteByID(ctx context.Context, noteID int64) (*PickupNote, error)
	GetStudentPickupNotes(ctx context.Context, studentID int64) ([]*PickupNote, error)
	GetStudentPickupNotesForDate(ctx context.Context, studentID int64, date timezone.Date) ([]*PickupNote, error)
	CreateStudentPickupNote(ctx context.Context, note *PickupNote) error
	UpdateStudentPickupNote(ctx context.Context, note *PickupNote) error
	DeleteStudentPickupNote(ctx context.Context, noteID int64) error
	DeleteAllStudentPickupNotes(ctx context.Context, studentID int64) error
}

// PickupTimes resolves the effective plan and its dated presentation.
type PickupTimes interface {
	BulkPickupTimes
	GetStudentPickupData(ctx context.Context, studentID int64) (*StudentPickupData, error)
	GetStudentPickupDataForRange(ctx context.Context, studentID int64, from, to timezone.Date) (*StudentPickupData, error)
	GetEffectivePickupTimeForDate(ctx context.Context, studentID int64, date timezone.Date) (*EffectivePickupTime, error)
}

// BulkPickupTimes is the tenant-scoped effective-time projection used by dashboards.
type BulkPickupTimes interface {
	GetBulkEffectivePickupTimesForDate(context.Context, []int64, timezone.Date) (map[int64]*EffectivePickupTime, error)
}

type PickupScheduleInput struct {
	Weekday    int    `json:"weekday"`
	PickupTime string `json:"pickup_time"`
}

type StudentPickupData struct {
	Schedules          []*PickupSchedule     `json:"schedules"`
	EffectiveSchedules []DatedPickupSchedule `json:"effective_schedules,omitempty"`
	Exceptions         []*PickupException    `json:"exceptions"`
	Notes              []*PickupNote         `json:"notes"`
}

type DatedPickupSchedule struct {
	Date             timezone.Date
	Schedule         *PickupSchedule
	OfferingSchedule *PickupSchedule
}
