package schedule

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
)

// Common validation error messages for arrival schedule models.
const (
	errMsgArrivalStudentIDRequired = "student_id is required"
	errMsgArrivalCreatedByRequired = "created_by is required"
)

// Field-length bounds shared by student arrival/pickup schedule models.
const (
	// scheduleNotesMaxLength caps free-text notes and note content.
	scheduleNotesMaxLength = 500
	// scheduleReasonMaxLength caps exception reason text.
	scheduleReasonMaxLength = 255
)

// StudentArrivalSchedule represents a recurring weekly arrival schedule for a student
type StudentArrivalSchedule struct {
	base.Model `bun:"schema:schedule,table:student_arrival_schedules"`
	base.TenantModel

	StudentID       int64     `bun:"student_id,notnull" json:"student_id"`
	Weekday         int       `bun:"weekday,notnull" json:"weekday"`
	ExpectedArrival time.Time `bun:"expected_arrival,notnull" json:"expected_arrival"`
	Notes           *string   `bun:"notes" json:"notes,omitempty"`
	CreatedBy       int64     `bun:"created_by,notnull" json:"created_by"`
}

// Validate ensures arrival schedule data is valid
func (s *StudentArrivalSchedule) Validate() error {
	if s.StudentID <= 0 {
		return errors.New(errMsgArrivalStudentIDRequired)
	}
	if s.Weekday < WeekdayMonday || s.Weekday > WeekdayFriday {
		return errors.New("weekday must be between 1 (Monday) and 5 (Friday)")
	}
	if s.ExpectedArrival.IsZero() {
		return errors.New("expected_arrival is required")
	}
	if s.CreatedBy <= 0 {
		return errors.New(errMsgArrivalCreatedByRequired)
	}
	if s.Notes != nil && len(*s.Notes) > scheduleNotesMaxLength {
		return errors.New("notes cannot exceed 500 characters")
	}
	return nil
}

// GetWeekdayName returns the German name for this schedule's weekday
func (s *StudentArrivalSchedule) GetWeekdayName() string {
	if name, ok := WeekdayNames[s.Weekday]; ok {
		return name
	}
	return ""
}

// StudentArrivalException represents a date-specific arrival exception
type StudentArrivalException struct {
	base.Model `bun:"schema:schedule,table:student_arrival_exceptions"`
	base.TenantModel

	StudentID         int64         `bun:"student_id,notnull" json:"student_id"`
	ExceptionDate     timezone.Date `bun:"exception_date,notnull" json:"exception_date"`
	ExpectedArrival   *time.Time    `bun:"expected_arrival" json:"expected_arrival,omitempty"`
	Reason            *string       `bun:"reason" json:"reason,omitempty"`
	Source            string        `bun:"source,nullzero,notnull,default:'staff'" json:"source"`
	CreatedBy         int64         `bun:"created_by,nullzero" json:"created_by,omitempty"`
	CreatedByGuardian *int64        `bun:"created_by_guardian,nullzero" json:"created_by_guardian,omitempty"`
}

// Validate ensures arrival exception data is valid
func (e *StudentArrivalException) Validate() error {
	if e.StudentID <= 0 {
		return errors.New(errMsgArrivalStudentIDRequired)
	}
	if e.ExceptionDate.IsZero() {
		return errors.New("exception_date is required")
	}
	if e.Reason != nil && len(*e.Reason) > scheduleReasonMaxLength {
		return errors.New("reason cannot exceed 255 characters")
	}
	if err := validateExceptionAuthor(e.Source, e.CreatedBy, e.CreatedByGuardian); err != nil {
		return err
	}
	return nil
}

// IsAbsent returns true if this exception indicates the student will not arrive (no arrival)
func (e *StudentArrivalException) IsAbsent() bool {
	return e.ExpectedArrival == nil
}

// StudentArrivalNote represents a date-specific note for a student's arrival
type StudentArrivalNote struct {
	base.Model `bun:"schema:schedule,table:student_arrival_notes"`
	base.TenantModel

	StudentID int64         `bun:"student_id,notnull" json:"student_id"`
	NoteDate  timezone.Date `bun:"note_date,notnull" json:"note_date"`
	Content   string        `bun:"content,notnull" json:"content"`
	CreatedBy int64         `bun:"created_by,notnull" json:"created_by"`
}

// Validate ensures arrival note data is valid
func (n *StudentArrivalNote) Validate() error {
	if n.StudentID <= 0 {
		return errors.New(errMsgArrivalStudentIDRequired)
	}
	if n.NoteDate.IsZero() {
		return errors.New("note_date is required")
	}
	if n.Content == "" {
		return errors.New("content is required")
	}
	if len(n.Content) > scheduleNotesMaxLength {
		return errors.New("content cannot exceed 500 characters")
	}
	if n.CreatedBy <= 0 {
		return errors.New(errMsgArrivalCreatedByRequired)
	}
	return nil
}

// StudentArrivalScheduleRepository defines operations for managing student arrival schedules
type StudentArrivalScheduleRepository interface {
	base.Repository[*StudentArrivalSchedule]

	// FindByStudentID finds all arrival schedules for a student
	FindByStudentID(ctx context.Context, studentID int64) ([]*StudentArrivalSchedule, error)

	// FindByStudentIDAndWeekday finds an arrival schedule for a specific student and weekday
	FindByStudentIDAndWeekday(ctx context.Context, studentID int64, weekday int) (*StudentArrivalSchedule, error)

	// FindByStudentIDsAndWeekday finds arrival schedules for multiple students and a specific weekday (bulk query)
	FindByStudentIDsAndWeekday(ctx context.Context, studentIDs []int64, weekday int) ([]*StudentArrivalSchedule, error)

	// FindByStudentIDs returns every weekday row for the given students in a
	// single query. The care-day derivation needs the whole week per child:
	// "no row for today" only means "not in care today" if the child has rows
	// for other weekdays — a child with no plan at all must stay visible.
	FindByStudentIDs(ctx context.Context, studentIDs []int64) ([]*StudentArrivalSchedule, error)

	// UpsertSchedule creates or updates an arrival schedule for a student and weekday
	UpsertSchedule(ctx context.Context, schedule *StudentArrivalSchedule) error

	// DeleteByStudentID deletes all arrival schedules for a student
	DeleteByStudentID(ctx context.Context, studentID int64) error
}

// StudentArrivalExceptionRepository defines operations for managing student arrival exceptions
type StudentArrivalExceptionRepository interface {
	base.Repository[*StudentArrivalException]

	// FindByIDForUpdate retrieves and locks an exception for an atomic
	// invariant check followed by mutation.
	FindByIDForUpdate(ctx context.Context, id any) (*StudentArrivalException, error)

	// FindByStudentID finds all arrival exceptions for a student
	FindByStudentID(ctx context.Context, studentID int64) ([]*StudentArrivalException, error)

	// FindUpcomingByStudentID finds upcoming arrival exceptions for a student (from today onwards)
	FindUpcomingByStudentID(ctx context.Context, studentID int64) ([]*StudentArrivalException, error)

	// FindByStudentIDAndDate finds an arrival exception for a specific student and date
	FindByStudentIDAndDate(ctx context.Context, studentID int64, date timezone.Date) (*StudentArrivalException, error)

	// FindByStudentIDsAndDate finds arrival exceptions for multiple students and a specific date (bulk query)
	FindByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]*StudentArrivalException, error)

	// FindByStudentIDAndDateRange finds arrival exceptions for a student whose
	// exception_date falls within the inclusive [from, to] range, sorted by
	// date. Used by the timetable per-student week endpoint to pre-load all
	// exceptions in a single query.
	FindByStudentIDAndDateRange(ctx context.Context, studentID int64, from, to timezone.Date) ([]*StudentArrivalException, error)

	// FindByStudentIDsAndDateRange is the bulk form of the above: every
	// exception for the given students within the inclusive range, so a
	// planner window resolves in one query instead of one per day.
	FindByStudentIDsAndDateRange(ctx context.Context, studentIDs []int64, from, to timezone.Date) ([]*StudentArrivalException, error)

	// DeleteByStudentID deletes all arrival exceptions for a student
	DeleteByStudentID(ctx context.Context, studentID int64) error

	// DeletePastExceptions deletes all exceptions older than the given date
	DeletePastExceptions(ctx context.Context, beforeDate timezone.Date) (int64, error)
}

// StudentArrivalNoteRepository defines operations for managing student arrival notes
type StudentArrivalNoteRepository interface {
	base.Repository[*StudentArrivalNote]

	// FindByStudentID finds all arrival notes for a student
	FindByStudentID(ctx context.Context, studentID int64) ([]*StudentArrivalNote, error)

	// FindByStudentIDAndDate finds all arrival notes for a student on a specific date
	FindByStudentIDAndDate(ctx context.Context, studentID int64, date timezone.Date) ([]*StudentArrivalNote, error)

	// FindByStudentIDsAndDate finds all arrival notes for multiple students on a specific date (bulk query)
	FindByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]*StudentArrivalNote, error)

	// DeleteByStudentID deletes all arrival notes for a student
	DeleteByStudentID(ctx context.Context, studentID int64) error

	// DeletePastNotes deletes all notes older than the given date
	DeletePastNotes(ctx context.Context, beforeDate timezone.Date) (int64, error)
}
