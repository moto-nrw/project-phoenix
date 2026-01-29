package schedule

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// Weekday constants (ISO 8601: Monday = 1, Friday = 5)
const (
	WeekdayMonday    = 1
	WeekdayTuesday   = 2
	WeekdayWednesday = 3
	WeekdayThursday  = 4
	WeekdayFriday    = 5
)

// Weekend day constants (for completeness)
const (
	WeekdaySaturday = 6
	WeekdaySunday   = 7
)

// WeekdayNames maps weekday numbers to German names
var WeekdayNames = map[int]string{
	WeekdayMonday:    "Montag",
	WeekdayTuesday:   "Dienstag",
	WeekdayWednesday: "Mittwoch",
	WeekdayThursday:  "Donnerstag",
	WeekdayFriday:    "Freitag",
	WeekdaySaturday:  "Samstag",
	WeekdaySunday:    "Sonntag",
}

// StudentPickupSchedule represents a recurring weekly pickup schedule for a student
type StudentPickupSchedule struct {
	base.Model `bun:"schema:schedule,table:student_pickup_schedules"`

	StudentID  int64     `bun:"student_id,notnull" json:"student_id"`
	Weekday    int       `bun:"weekday,notnull" json:"weekday"`
	PickupTime time.Time `bun:"pickup_time,notnull" json:"pickup_time"`
	Notes      *string   `bun:"notes" json:"notes,omitempty"`
	CreatedBy  int64     `bun:"created_by,notnull" json:"created_by"`
}

func (s *StudentPickupSchedule) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(`schedule.student_pickup_schedules AS "schedule"`)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(`schedule.student_pickup_schedules AS "schedule"`)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(`schedule.student_pickup_schedules AS "schedule"`)
	}
	return nil
}

// TableName returns the database table name
func (s *StudentPickupSchedule) TableName() string {
	return "schedule.student_pickup_schedules"
}

// Validate ensures pickup schedule data is valid
func (s *StudentPickupSchedule) Validate() error {
	if s.StudentID <= 0 {
		return errors.New("student_id is required")
	}
	if s.Weekday < WeekdayMonday || s.Weekday > WeekdayFriday {
		return errors.New("weekday must be between 1 (Monday) and 5 (Friday)")
	}
	if s.PickupTime.IsZero() {
		return errors.New("pickup_time is required")
	}
	if s.CreatedBy <= 0 {
		return errors.New("created_by is required")
	}
	if s.Notes != nil && len(*s.Notes) > 500 {
		return errors.New("notes cannot exceed 500 characters")
	}
	return nil
}

// GetWeekdayName returns the German name for this schedule's weekday
func (s *StudentPickupSchedule) GetWeekdayName() string {
	if name, ok := WeekdayNames[s.Weekday]; ok {
		return name
	}
	return ""
}

// GetID implements the Entity interface
func (s *StudentPickupSchedule) GetID() any {
	return s.ID
}

// GetCreatedAt implements the Entity interface
func (s *StudentPickupSchedule) GetCreatedAt() time.Time {
	return s.CreatedAt
}

// GetUpdatedAt implements the Entity interface
func (s *StudentPickupSchedule) GetUpdatedAt() time.Time {
	return s.UpdatedAt
}

// StudentPickupException represents a date-specific pickup exception
type StudentPickupException struct {
	base.Model `bun:"schema:schedule,table:student_pickup_exceptions"`

	StudentID     int64      `bun:"student_id,notnull" json:"student_id"`
	ExceptionDate time.Time  `bun:"exception_date,notnull" json:"exception_date"`
	PickupTime    *time.Time `bun:"pickup_time" json:"pickup_time,omitempty"`
	Reason        *string    `bun:"reason" json:"reason,omitempty"`
	CreatedBy     int64      `bun:"created_by,notnull" json:"created_by"`
}

func (e *StudentPickupException) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(`schedule.student_pickup_exceptions AS "exception"`)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(`schedule.student_pickup_exceptions AS "exception"`)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(`schedule.student_pickup_exceptions AS "exception"`)
	}
	return nil
}

// TableName returns the database table name
func (e *StudentPickupException) TableName() string {
	return "schedule.student_pickup_exceptions"
}

// Validate ensures pickup exception data is valid
func (e *StudentPickupException) Validate() error {
	if e.StudentID <= 0 {
		return errors.New("student_id is required")
	}
	if e.ExceptionDate.IsZero() {
		return errors.New("exception_date is required")
	}
	if e.Reason != nil && len(*e.Reason) > 255 {
		return errors.New("reason cannot exceed 255 characters")
	}
	if e.CreatedBy <= 0 {
		return errors.New("created_by is required")
	}
	return nil
}

// IsAbsent returns true if this exception indicates the student will be absent (no pickup)
func (e *StudentPickupException) IsAbsent() bool {
	return e.PickupTime == nil
}

// GetID implements the Entity interface
func (e *StudentPickupException) GetID() any {
	return e.ID
}

// GetCreatedAt implements the Entity interface
func (e *StudentPickupException) GetCreatedAt() time.Time {
	return e.CreatedAt
}

// GetUpdatedAt implements the Entity interface
func (e *StudentPickupException) GetUpdatedAt() time.Time {
	return e.UpdatedAt
}

// StudentPickupScheduleRepository defines operations for managing student pickup schedules
type StudentPickupScheduleRepository interface {
	base.Repository[*StudentPickupSchedule]

	// FindByStudentID finds all pickup schedules for a student
	FindByStudentID(ctx context.Context, studentID int64) ([]*StudentPickupSchedule, error)

	// FindByStudentIDAndWeekday finds a pickup schedule for a specific student and weekday
	FindByStudentIDAndWeekday(ctx context.Context, studentID int64, weekday int) (*StudentPickupSchedule, error)

	// FindByStudentIDsAndWeekday finds pickup schedules for multiple students and a specific weekday (bulk query)
	FindByStudentIDsAndWeekday(ctx context.Context, studentIDs []int64, weekday int) ([]*StudentPickupSchedule, error)

	// UpsertSchedule creates or updates a pickup schedule for a student and weekday
	UpsertSchedule(ctx context.Context, schedule *StudentPickupSchedule) error

	// DeleteByStudentID deletes all pickup schedules for a student
	DeleteByStudentID(ctx context.Context, studentID int64) error
}

// StudentPickupExceptionRepository defines operations for managing student pickup exceptions
type StudentPickupExceptionRepository interface {
	base.Repository[*StudentPickupException]

	// FindByStudentID finds all pickup exceptions for a student
	FindByStudentID(ctx context.Context, studentID int64) ([]*StudentPickupException, error)

	// FindUpcomingByStudentID finds upcoming pickup exceptions for a student (from today onwards)
	FindUpcomingByStudentID(ctx context.Context, studentID int64) ([]*StudentPickupException, error)

	// FindByStudentIDAndDate finds a pickup exception for a specific student and date
	FindByStudentIDAndDate(ctx context.Context, studentID int64, date time.Time) (*StudentPickupException, error)

	// FindByStudentIDsAndDate finds pickup exceptions for multiple students and a specific date (bulk query)
	FindByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date time.Time) ([]*StudentPickupException, error)

	// DeleteByStudentID deletes all pickup exceptions for a student
	DeleteByStudentID(ctx context.Context, studentID int64) error

	// DeletePastExceptions deletes all exceptions older than the given date
	DeletePastExceptions(ctx context.Context, beforeDate time.Time) (int64, error)
}

// StudentPickupNote represents a date-specific note for a student's pickup
type StudentPickupNote struct {
	base.Model `bun:"schema:schedule,table:student_pickup_notes"`

	StudentID int64     `bun:"student_id,notnull" json:"student_id"`
	NoteDate  time.Time `bun:"note_date,notnull" json:"note_date"`
	Content   string    `bun:"content,notnull" json:"content"`
	CreatedBy int64     `bun:"created_by,notnull" json:"created_by"`
}

func (n *StudentPickupNote) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(`schedule.student_pickup_notes AS "student_pickup_note"`)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(`schedule.student_pickup_notes AS "student_pickup_note"`)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(`schedule.student_pickup_notes AS "student_pickup_note"`)
	}
	return nil
}

// TableName returns the database table name
func (n *StudentPickupNote) TableName() string {
	return "schedule.student_pickup_notes"
}

// Validate ensures pickup note data is valid
func (n *StudentPickupNote) Validate() error {
	if n.StudentID <= 0 {
		return errors.New("student_id is required")
	}
	if n.NoteDate.IsZero() {
		return errors.New("note_date is required")
	}
	if n.Content == "" {
		return errors.New("content is required")
	}
	if len(n.Content) > 500 {
		return errors.New("content cannot exceed 500 characters")
	}
	if n.CreatedBy <= 0 {
		return errors.New("created_by is required")
	}
	return nil
}

// GetID implements the Entity interface
func (n *StudentPickupNote) GetID() any {
	return n.ID
}

// GetCreatedAt implements the Entity interface
func (n *StudentPickupNote) GetCreatedAt() time.Time {
	return n.CreatedAt
}

// GetUpdatedAt implements the Entity interface
func (n *StudentPickupNote) GetUpdatedAt() time.Time {
	return n.UpdatedAt
}

// StudentPickupNoteRepository defines operations for managing student pickup notes
type StudentPickupNoteRepository interface {
	base.Repository[*StudentPickupNote]

	// FindByStudentID finds all pickup notes for a student
	FindByStudentID(ctx context.Context, studentID int64) ([]*StudentPickupNote, error)

	// FindByStudentIDAndDate finds all pickup notes for a student on a specific date
	FindByStudentIDAndDate(ctx context.Context, studentID int64, date time.Time) ([]*StudentPickupNote, error)

	// FindByStudentIDsAndDate finds all pickup notes for multiple students on a specific date (bulk query)
	FindByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date time.Time) ([]*StudentPickupNote, error)

	// DeleteByStudentID deletes all pickup notes for a student
	DeleteByStudentID(ctx context.Context, studentID int64) error

	// DeletePastNotes deletes all notes older than the given date
	DeletePastNotes(ctx context.Context, beforeDate time.Time) (int64, error)
}
