package careplan

import (
	"context"
	"errors"
	"time"
)

const (
	ScheduleSourceStaff          = "staff"
	ScheduleSourceClassSchedule  = "class_schedule"
	ScheduleSourceClassException = "class_exception"
	ScheduleSourceCareOffering   = "care_offering"
	ExceptionSourceStaff         = "staff"
	ExceptionSourceGuardian      = "guardian"
)

var (
	ErrInvalidStudentSchedule  = errors.New("invalid student arrival or pickup schedule")
	ErrStudentScheduleNotFound = errors.New("student arrival or pickup schedule not found")
)

// StudentScheduleFilter is the closed query contract shared by the six
// arrival/pickup record types. Dates are inclusive and use DateLayout.
type StudentScheduleFilter struct {
	IDs           []int64
	StudentIDs    []int64
	Weekday       int
	Date          Date
	From          Date
	To            Date
	UpcomingFrom  Date
	LockForUpdate bool
	Options       *StudentScheduleQueryOptions
}

// StudentScheduleQueryOptions preserves the legacy repository listing shape
// without exposing persistence-layer query types through the Care Plan API.
type StudentScheduleQueryOptions struct {
	Filter  *StudentScheduleQueryFilter
	Limit   int
	Offset  int
	Sorting []StudentScheduleSortField
}

type StudentScheduleQueryFilter struct {
	Conditions []StudentScheduleQueryCondition
	Or         []StudentScheduleQueryFilter
	And        []StudentScheduleQueryFilter
}

type StudentScheduleQueryCondition struct {
	Field    string
	Operator string
	Value    any
}

type StudentScheduleSortField struct {
	Field      string
	Descending bool
}

type ArrivalSchedule struct {
	ID              int64
	TenantID        int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
	StudentID       int64
	Weekday         int
	ExpectedArrival time.Time
	Notes           *string
	CreatedBy       int64
	Source          string
	SourceClass     string
	SourceLabel     string
}

// InheritsClassTime reports whether the care day has no manual arrival time.
func (s *ArrivalSchedule) InheritsClassTime() bool { return s.ExpectedArrival.IsZero() }

type ArrivalException struct {
	ID                int64
	TenantID          int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
	StudentID         int64
	ExceptionDate     Date
	ExpectedArrival   *time.Time
	Reason            *string
	Source            string
	CreatedBy         int64
	CreatedByGuardian *int64
}

type ArrivalNote struct {
	ID        int64
	TenantID  int64
	CreatedAt time.Time
	UpdatedAt time.Time
	StudentID int64
	NoteDate  Date
	Content   string
	CreatedBy int64
}

type PickupSchedule struct {
	ID               int64
	TenantID         int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
	StudentID        int64
	Weekday          int
	PickupTime       time.Time
	Notes            *string
	CreatedBy        int64
	Source           string
	CareOfferingID   *int64
	CareOfferingName string
}

type PickupException struct {
	ID                    int64
	TenantID              int64
	CreatedAt             time.Time
	UpdatedAt             time.Time
	StudentID             int64
	ExceptionDate         Date
	PickupTime            *time.Time
	Reason                *string
	ExcusedFrom           *time.Time
	ExcusedReason         *string
	ExcusedCreatedBy      *int64
	ExcusedOwnsPickupTime bool
	ExcusedAuto           bool
	Source                string
	CreatedBy             int64
	CreatedByGuardian     *int64
}

// PickupNote is either dated (NoteDate) or recurring on a weekday (Weekday
// 1-5, #3369), never both. A recurring note says nothing about attendance: it
// exists so a day the child does not come can still carry a note.
type PickupNote struct {
	ID        int64
	TenantID  int64
	CreatedAt time.Time
	UpdatedAt time.Time
	StudentID int64
	NoteDate  Date
	Weekday   int
	Content   string
	CreatedBy int64
}

// StudentSchedulesQuery reads the weekly arrival and pickup plans of
// children with their exceptions and notes.
type StudentSchedulesQuery interface {
	ArrivalSchedulesQuery
	PickupSchedulesQuery
	CountStudentScheduleRows(context.Context, int64) (int, error)
}

// ArrivalSchedulesQuery reads arrival schedules, exceptions and notes.
type ArrivalSchedulesQuery interface {
	FindArrivalSchedule(context.Context, int64) (ArrivalSchedule, error)
	ListArrivalSchedules(context.Context, StudentScheduleFilter) ([]ArrivalSchedule, error)
	FindArrivalException(context.Context, int64, bool) (ArrivalException, error)
	ListArrivalExceptions(context.Context, StudentScheduleFilter) ([]ArrivalException, error)
	FindArrivalNote(context.Context, int64) (ArrivalNote, error)
	ListArrivalNotes(context.Context, StudentScheduleFilter) ([]ArrivalNote, error)
}

// PickupSchedulesQuery reads pickup schedules, exceptions and notes.
type PickupSchedulesQuery interface {
	FindPickupSchedule(context.Context, int64) (PickupSchedule, error)
	ListPickupSchedules(context.Context, StudentScheduleFilter) ([]PickupSchedule, error)
	FindPickupException(context.Context, int64, bool) (PickupException, error)
	ListPickupExceptions(context.Context, StudentScheduleFilter) ([]PickupException, error)
	FindPickupNote(context.Context, int64) (PickupNote, error)
	ListPickupNotes(context.Context, StudentScheduleFilter) ([]PickupNote, error)
}

// StudentSchedulesCommand writes the weekly arrival and pickup plans of
// children with their exceptions and notes, and ends or restores them when
// care ends.
type StudentSchedulesCommand interface {
	ArrivalSchedulesCommand
	ArrivalExceptionsCommand
	ArrivalNotesCommand
	PickupSchedulesCommand
	PickupExceptionsCommand
	PickupNotesCommand
	EndStudentSchedulesForCareExit(context.Context, []int64, Date) (int64, error)
	RestoreStudentSchedulesForCareExit(context.Context, []int64) (int64, error)
}

// ArrivalSchedulesCommand writes weekly arrival schedules.
type ArrivalSchedulesCommand interface {
	CreateArrivalSchedule(context.Context, ArrivalSchedule) (ArrivalSchedule, error)
	UpdateArrivalSchedule(context.Context, ArrivalSchedule) error
	UpsertArrivalSchedule(context.Context, ArrivalSchedule) (ArrivalSchedule, error)
	DeleteArrivalSchedule(context.Context, int64) error
	DeleteArrivalSchedulesByStudent(context.Context, int64) error
}

// ArrivalExceptionsCommand writes dated arrival exceptions.
type ArrivalExceptionsCommand interface {
	CreateArrivalException(context.Context, ArrivalException) (ArrivalException, error)
	UpdateArrivalException(context.Context, ArrivalException) error
	DeleteArrivalException(context.Context, int64) error
	DeleteArrivalExceptionsByStudent(context.Context, int64) error
	DeleteArrivalExceptionsBefore(context.Context, Date) (int64, error)
}

// ArrivalNotesCommand writes arrival notes.
type ArrivalNotesCommand interface {
	CreateArrivalNote(context.Context, ArrivalNote) (ArrivalNote, error)
	UpdateArrivalNote(context.Context, ArrivalNote) error
	DeleteArrivalNote(context.Context, int64) error
	DeleteArrivalNotesByStudent(context.Context, int64) error
	DeleteArrivalNotesBefore(context.Context, Date) (int64, error)
}

// PickupSchedulesCommand writes weekly pickup schedules.
type PickupSchedulesCommand interface {
	CreatePickupSchedule(context.Context, PickupSchedule) (PickupSchedule, error)
	UpdatePickupSchedule(context.Context, PickupSchedule) error
	UpsertPickupSchedule(context.Context, PickupSchedule) (PickupSchedule, error)
	DeletePickupSchedule(context.Context, int64) error
	DeletePickupSchedulesByStudent(context.Context, int64) error
}

// PickupExceptionsCommand writes dated pickup exceptions.
type PickupExceptionsCommand interface {
	CreatePickupException(context.Context, PickupException) (PickupException, error)
	UpdatePickupException(context.Context, PickupException) error
	DeletePickupException(context.Context, int64) error
	DeletePickupExceptionsByStudent(context.Context, int64) error
	DeletePickupExceptionsBefore(context.Context, Date) (int64, error)
}

// PickupNotesCommand writes pickup notes.
type PickupNotesCommand interface {
	CreatePickupNote(context.Context, PickupNote) (PickupNote, error)
	UpdatePickupNote(context.Context, PickupNote) error
	DeletePickupNote(context.Context, int64) error
	DeletePickupNotesByStudent(context.Context, int64) error
	DeletePickupNotesBefore(context.Context, Date) (int64, error)
}

// WeekdayPickupNotesCommand replaces recurring notes without enlarging the
// retained StudentSchedulesCommand compatibility facade.
type WeekdayPickupNotesCommand interface {
	ReplaceWeekdayPickupNotes(context.Context, int64, int64, map[int]string) error
}
