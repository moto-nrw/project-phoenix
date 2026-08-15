package schedule

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
)

// Common validation error messages for pickup schedule models.
const (
	errMsgStudentIDRequired = "student_id is required"
	errMsgCreatedByRequired = "created_by is required"
)

// Exception authorship sources. A date-specific exception is either written by
// staff (created_by → users.staff) or by a guardian from the parents portal
// (created_by_guardian → auth.accounts). The source column makes the author
// first-class so the staff UI can flag parent-driven changes.
const (
	ExceptionSourceStaff    = "staff"
	ExceptionSourceGuardian = "guardian"
)

// errMsgGuardianAuthorRequired is returned when a guardian-sourced exception
// carries no guardian account id.
const errMsgGuardianAuthorRequired = "created_by_guardian is required for guardian-authored exceptions"

// validateExceptionAuthor enforces the source/author invariant shared by the
// pickup and arrival exception models: a staff row needs created_by, a guardian
// row needs created_by_guardian. An empty source is treated as staff to match
// the DB default applied before insert.
func validateExceptionAuthor(source string, createdBy int64, createdByGuardian *int64) error {
	switch source {
	case "", ExceptionSourceStaff:
		if createdBy <= 0 {
			return errors.New(errMsgCreatedByRequired)
		}
	case ExceptionSourceGuardian:
		if createdByGuardian == nil || *createdByGuardian <= 0 {
			return errors.New(errMsgGuardianAuthorRequired)
		}
	default:
		return errors.New("invalid exception source")
	}
	return nil
}

// Recurring pickup-schedule row sources (#2290). Unlike the exception
// sources above, these describe WHERE the weekly Gehzeit came from: staff
// keeps it manually, care_offering rows are materialized from an
// Angebots-Gehzeit and belong to automatic reconciliation.
const (
	PickupScheduleSourceStaff        = "staff"
	PickupScheduleSourceCareOffering = "care_offering"
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
	base.TenantModel

	StudentID  int64     `bun:"student_id,notnull" json:"student_id"`
	Weekday    int       `bun:"weekday,notnull" json:"weekday"`
	PickupTime time.Time `bun:"pickup_time,notnull" json:"pickup_time"`
	Notes      *string   `bun:"notes" json:"notes,omitempty"`
	CreatedBy  int64     `bun:"created_by,notnull" json:"created_by"`
	// Source marks how the row came to be: staff-maintained (default) or
	// materialized from a care offering's Angebots-Gehzeit (#2290). A manual
	// edit of a care_offering row flips it back to staff, which shields it
	// from automatic reconciliation.
	Source         string `bun:"source,nullzero,notnull,default:'staff'" json:"source"`
	CareOfferingID *int64 `bun:"care_offering_id,nullzero" json:"care_offering_id,omitempty"`
}

// Validate ensures pickup schedule data is valid
func (s *StudentPickupSchedule) Validate() error {
	if s.StudentID <= 0 {
		return errors.New(errMsgStudentIDRequired)
	}
	if s.Weekday < WeekdayMonday || s.Weekday > WeekdayFriday {
		return errors.New("weekday must be between 1 (Monday) and 5 (Friday)")
	}
	if s.PickupTime.IsZero() {
		return errors.New("pickup_time is required")
	}
	if s.CreatedBy <= 0 {
		return errors.New(errMsgCreatedByRequired)
	}
	if s.Notes != nil && len(*s.Notes) > scheduleNotesMaxLength {
		return errors.New("notes cannot exceed 500 characters")
	}
	switch s.Source {
	case "":
		s.Source = PickupScheduleSourceStaff
	case PickupScheduleSourceStaff:
		// no offering reference required
	case PickupScheduleSourceCareOffering:
		if s.CareOfferingID == nil || *s.CareOfferingID <= 0 {
			return errors.New("care_offering_id is required for care_offering-sourced rows")
		}
	default:
		return errors.New("invalid pickup schedule source")
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

// StudentPickupException represents a date-specific pickup exception
type StudentPickupException struct {
	base.Model `bun:"schema:schedule,table:student_pickup_exceptions"`
	base.TenantModel

	StudentID     int64         `bun:"student_id,notnull" json:"student_id"`
	ExceptionDate timezone.Date `bun:"exception_date,notnull" json:"exception_date"`
	PickupTime    *time.Time    `bun:"pickup_time" json:"pickup_time,omitempty"`
	Reason        *string       `bun:"reason" json:"reason,omitempty"`
	// ExcusedFrom turns this existing day-specific pickup override into the
	// source for a partial excusal. PickupTime normally matches it, but may
	// differ after an explicit pickup-time decision.
	ExcusedFrom           *time.Time `bun:"excused_from" json:"excused_from,omitempty"`
	ExcusedReason         *string    `bun:"excused_reason" json:"excused_reason,omitempty"`
	ExcusedCreatedBy      *int64     `bun:"excused_created_by" json:"excused_created_by,omitempty"`
	ExcusedOwnsPickupTime bool       `bun:"excused_owns_pickup_time,notnull,default:false" json:"excused_owns_pickup_time"`
	Source                string     `bun:"source,nullzero,notnull,default:'staff'" json:"source"`
	CreatedBy             int64      `bun:"created_by,nullzero" json:"created_by,omitempty"`
	CreatedByGuardian     *int64     `bun:"created_by_guardian,nullzero" json:"created_by_guardian,omitempty"`
}

// Validate ensures pickup exception data is valid
func (e *StudentPickupException) Validate() error {
	if e.StudentID <= 0 {
		return errors.New(errMsgStudentIDRequired)
	}
	if e.ExceptionDate.IsZero() {
		return errors.New("exception_date is required")
	}
	if e.Reason != nil && len(*e.Reason) > scheduleReasonMaxLength {
		return errors.New("reason cannot exceed 255 characters")
	}
	if e.ExcusedReason != nil && len(*e.ExcusedReason) > scheduleReasonMaxLength {
		return errors.New("excused_reason cannot exceed 255 characters")
	}
	if e.ExcusedFrom == nil {
		if e.ExcusedReason != nil || e.ExcusedCreatedBy != nil || e.ExcusedOwnsPickupTime {
			return errors.New("partial absence metadata requires excused_from")
		}
	} else if e.ExcusedCreatedBy == nil || *e.ExcusedCreatedBy <= 0 {
		return errors.New("excused_created_by is required for partial absences")
	} else if e.ExcusedOwnsPickupTime && e.PickupTime == nil {
		return errors.New("owned partial-absence pickup time is required")
	}
	if err := validateExceptionAuthor(e.Source, e.CreatedBy, e.CreatedByGuardian); err != nil {
		return err
	}
	return nil
}

// IsAbsent returns true if this exception indicates the student will be absent (no pickup)
func (e *StudentPickupException) IsAbsent() bool {
	return e.PickupTime == nil
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

	// FindByStudentIDs returns every weekday row for the given students in a
	// single query. Mirror of the arrival-side helper; see there for why the
	// care-day derivation needs the whole week rather than one weekday.
	FindByStudentIDs(ctx context.Context, studentIDs []int64) ([]*StudentPickupSchedule, error)

	// UpsertSchedule creates or updates a pickup schedule for a student and weekday
	UpsertSchedule(ctx context.Context, schedule *StudentPickupSchedule) error

	// DeleteByStudentID deletes all pickup schedules for a student
	DeleteByStudentID(ctx context.Context, studentID int64) error
}

// StudentPickupExceptionRepository defines operations for managing student pickup exceptions
type StudentPickupExceptionRepository interface {
	base.Repository[*StudentPickupException]

	// FindByIDForUpdate retrieves and locks an exception for an atomic
	// invariant check followed by mutation.
	FindByIDForUpdate(ctx context.Context, id any) (*StudentPickupException, error)

	// FindByStudentID finds all pickup exceptions for a student
	FindByStudentID(ctx context.Context, studentID int64) ([]*StudentPickupException, error)

	// FindUpcomingByStudentID finds upcoming pickup exceptions for a student (from today onwards)
	FindUpcomingByStudentID(ctx context.Context, studentID int64) ([]*StudentPickupException, error)

	// FindByStudentIDAndDate finds a pickup exception for a specific student and date
	FindByStudentIDAndDate(ctx context.Context, studentID int64, date timezone.Date) (*StudentPickupException, error)

	// FindByStudentIDsAndDate finds pickup exceptions for multiple students and a specific date (bulk query)
	FindByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]*StudentPickupException, error)

	// FindByStudentIDAndDateRange finds pickup exceptions for a student whose
	// exception_date falls within the inclusive [from, to] range, sorted by
	// date. Used by the timetable per-student week endpoint to pre-load all
	// exceptions in a single query.
	FindByStudentIDAndDateRange(ctx context.Context, studentID int64, from, to timezone.Date) ([]*StudentPickupException, error)

	// FindByStudentIDsAndDateRange is the bulk form of the above: every
	// exception for the given students within the inclusive range, so a
	// planner window resolves in one query instead of one per day.
	FindByStudentIDsAndDateRange(ctx context.Context, studentIDs []int64, from, to timezone.Date) ([]*StudentPickupException, error)

	// DeleteByStudentID deletes all pickup exceptions for a student
	DeleteByStudentID(ctx context.Context, studentID int64) error

	// DeletePastExceptions deletes all exceptions older than the given date
	DeletePastExceptions(ctx context.Context, beforeDate timezone.Date) (int64, error)
}

// StudentPickupNote represents a date-specific note for a student's pickup
type StudentPickupNote struct {
	base.Model `bun:"schema:schedule,table:student_pickup_notes"`
	base.TenantModel

	StudentID int64         `bun:"student_id,notnull" json:"student_id"`
	NoteDate  timezone.Date `bun:"note_date,notnull" json:"note_date"`
	Content   string        `bun:"content,notnull" json:"content"`
	CreatedBy int64         `bun:"created_by,notnull" json:"created_by"`
}

// Validate ensures pickup note data is valid
func (n *StudentPickupNote) Validate() error {
	if n.StudentID <= 0 {
		return errors.New(errMsgStudentIDRequired)
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
		return errors.New(errMsgCreatedByRequired)
	}
	return nil
}

// StudentPickupNoteRepository defines operations for managing student pickup notes
type StudentPickupNoteRepository interface {
	base.Repository[*StudentPickupNote]

	// FindByStudentID finds all pickup notes for a student
	FindByStudentID(ctx context.Context, studentID int64) ([]*StudentPickupNote, error)

	// FindByStudentIDAndDate finds all pickup notes for a student on a specific date
	FindByStudentIDAndDate(ctx context.Context, studentID int64, date timezone.Date) ([]*StudentPickupNote, error)

	// FindByStudentIDsAndDate finds all pickup notes for multiple students on a specific date (bulk query)
	FindByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]*StudentPickupNote, error)

	// DeleteByStudentID deletes all pickup notes for a student
	DeleteByStudentID(ctx context.Context, studentID int64) error

	// DeletePastNotes deletes all notes older than the given date
	DeletePastNotes(ctx context.Context, beforeDate timezone.Date) (int64, error)
}
