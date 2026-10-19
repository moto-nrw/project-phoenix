package schedulerecords

import (
	"errors"
	"time"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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

// Arrival-schedule provenance (#2414, ADR 0005). It is never persisted: a
// stored row is a manual override by definition, and the projection sets these
// fields on the rows it builds at read time.
const (
	ArrivalScheduleSourceStaff         = "staff"
	ArrivalScheduleSourceClassSchedule = "class_schedule"
	// ArrivalScheduleSourceClassException marks a projected row whose time
	// comes from a class-wide day exception (#2962): the whole class arrives
	// at a different time on that date, which outranks both the class time
	// and a per-child weekly deviation.
	ArrivalScheduleSourceClassException = "class_exception"
)

// StudentArrivalSchedule represents a recurring weekly arrival schedule for a student
type StudentArrivalSchedule struct {
	Model `bun:"schema:schedule,table:student_arrival_schedules"`
	TenantModel

	StudentID int64 `bun:"student_id,notnull" json:"student_id"`
	Weekday   int   `bun:"weekday,notnull" json:"weekday"`
	// ExpectedArrival is optional (#2414, ADR 0005): the zero value means
	// "take the time from this child's class timetable for that weekday". A
	// set value is a deliberate per-child deviation and wins over the class.
	ExpectedArrival time.Time `bun:"expected_arrival,nullzero" json:"expected_arrival,omitzero"`
	Notes           *string   `bun:"notes" json:"notes,omitempty"`
	CreatedBy       int64     `bun:"created_by,notnull" json:"created_by"`

	// Source and SourceClass are hydrated on read-time projections only and
	// have no column: "staff" marks a stored manual override, "class_schedule"
	// a time projected from the class timetable, with SourceClass naming the
	// class it came from. Mirrors StudentPickupSchedule.CareOfferingName.
	Source      string `bun:"-" json:"source,omitempty"`
	SourceClass string `bun:"-" json:"source_class,omitempty"`
	// SourceLabel is the ready-made line a class-wide day exception carries
	// ("Klasse 4a: Unterricht fällt aus", #2962); empty for every other source.
	SourceLabel string `bun:"-" json:"source_label,omitempty"`
}

// Validate ensures arrival schedule data is valid
func (s *StudentArrivalSchedule) Validate() error {
	if s.StudentID <= 0 {
		return errors.New(errMsgArrivalStudentIDRequired)
	}
	if s.Weekday < WeekdayMonday || s.Weekday > WeekdayFriday {
		return errors.New("weekday must be between 1 (Monday) and 5 (Friday)")
	}
	if s.CreatedBy <= 0 {
		return errors.New(errMsgArrivalCreatedByRequired)
	}
	if s.Notes != nil && len(*s.Notes) > scheduleNotesMaxLength {
		return errors.New("notes cannot exceed 500 characters")
	}
	return nil
}

// InheritsClassTime reports whether this row takes its time from the class
// timetable instead of carrying its own (#2414). The row still marks the
// weekday as a care day either way.
func (s *StudentArrivalSchedule) InheritsClassTime() bool {
	return s.ExpectedArrival.IsZero()
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
	Model `bun:"schema:schedule,table:student_arrival_exceptions"`
	TenantModel

	StudentID         int64         `bun:"student_id,notnull" json:"student_id"`
	ExceptionDate     calendar.Date `bun:"exception_date,notnull" json:"exception_date"`
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
	if e.Reason != nil && utf8.RuneCountInString(*e.Reason) > scheduleReasonMaxLength {
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
	Model `bun:"schema:schedule,table:student_arrival_notes"`
	TenantModel

	StudentID int64         `bun:"student_id,notnull" json:"student_id"`
	NoteDate  calendar.Date `bun:"note_date,notnull" json:"note_date"`
	Content   string        `bun:"content,notnull" json:"content"`
	CreatedBy int64         `bun:"created_by,notnull" json:"created_by"`
}

// ports.StudentArrivalScheduleRecords defines operations for managing student arrival schedules

// ports.StudentArrivalExceptionRecords defines operations for managing student arrival exceptions

// ports.StudentArrivalNoteRecords defines operations for managing student arrival notes
