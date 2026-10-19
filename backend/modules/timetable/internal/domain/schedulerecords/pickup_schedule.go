package schedulerecords

import (
	"errors"
	"time"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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
// sources above, these describe WHERE the weekly Gehzeit came from. New
// stored rows are staff-maintained. care_offering identifies legacy
// materializations; current offering values are projected at read time.
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
	Model `bun:"schema:schedule,table:student_pickup_schedules"`
	TenantModel

	StudentID  int64     `bun:"student_id,notnull" json:"student_id"`
	Weekday    int       `bun:"weekday,notnull" json:"weekday"`
	PickupTime time.Time `bun:"pickup_time,notnull" json:"pickup_time"`
	Notes      *string   `bun:"notes" json:"notes,omitempty"`
	CreatedBy  int64     `bun:"created_by,nullzero" json:"created_by"`
	// Source marks a stored staff override (default) or a legacy offering
	// materialization. Read-time offering projections are synthetic and are
	// never inserted into this table.
	Source         string `bun:"source,nullzero,notnull,default:'staff'" json:"source"`
	CareOfferingID *int64 `bun:"care_offering_id,nullzero" json:"care_offering_id,omitempty"`
	// CareOfferingName is hydrated only on read-time projections. It is not
	// persisted because the offering catalog remains the source of the name.
	CareOfferingName string `bun:"-" json:"care_offering_name,omitempty"`
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
	if s.CreatedBy <= 0 && s.Source != PickupScheduleSourceCareOffering {
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
	Model `bun:"schema:schedule,table:student_pickup_exceptions"`
	TenantModel

	StudentID     int64         `bun:"student_id,notnull" json:"student_id"`
	ExceptionDate calendar.Date `bun:"exception_date,notnull" json:"exception_date"`
	PickupTime    *time.Time    `bun:"pickup_time" json:"pickup_time,omitempty"`
	Reason        *string       `bun:"reason" json:"reason,omitempty"`
	// ExcusedFrom turns this existing day-specific pickup override into the
	// source for a partial excusal. PickupTime normally matches it, but may
	// differ after an explicit pickup-time decision.
	ExcusedFrom           *time.Time `bun:"excused_from" json:"excused_from,omitempty"`
	ExcusedReason         *string    `bun:"excused_reason" json:"excused_reason,omitempty"`
	ExcusedCreatedBy      *int64     `bun:"excused_created_by" json:"excused_created_by,omitempty"`
	ExcusedOwnsPickupTime bool       `bun:"excused_owns_pickup_time,notnull,default:false" json:"excused_owns_pickup_time"`
	// ExcusedAuto marks a partial excusal that was derived automatically from a
	// pulled-forward pickup time (#2360). Auto rows always mirror PickupTime,
	// carry no staff author, and are re-synced (or released) whenever the
	// pickup exception changes. Manual partial absences keep ExcusedAuto false
	// and are never touched by the sync.
	ExcusedAuto       bool   `bun:"excused_auto,notnull,default:false" json:"excused_auto"`
	Source            string `bun:"source,nullzero,notnull,default:'staff'" json:"source"`
	CreatedBy         int64  `bun:"created_by,nullzero" json:"created_by,omitempty"`
	CreatedByGuardian *int64 `bun:"created_by_guardian,nullzero" json:"created_by_guardian,omitempty"`
}

// HasManualPartialAbsence reports whether this exception carries a staff-set
// partial excusal. Auto-derived excusals (ExcusedAuto) follow the pickup time
// and must not lock the exception against edits the way a manual one does.
func (e *StudentPickupException) HasManualPartialAbsence() bool {
	return e.ExcusedFrom != nil && !e.ExcusedAuto
}

// NormalizeWallClockTimes re-anchors the TIME-column values onto the
// canonical wall-clock reference date. The driver scans TIME columns onto
// year 0, which bun would bind back out of range on a full-row update —
// every writer that updates a loaded row must normalize first.
func (e *StudentPickupException) NormalizeWallClockTimes() {
	if e.PickupTime != nil {
		clock := calendar.NormalizeWallClock(*e.PickupTime)
		e.PickupTime = &clock
	}
	if e.ExcusedFrom != nil {
		clock := calendar.NormalizeWallClock(*e.ExcusedFrom)
		e.ExcusedFrom = &clock
	}
}

// Validate ensures pickup exception data is valid
func (e *StudentPickupException) Validate() error {
	if e.StudentID <= 0 {
		return errors.New(errMsgStudentIDRequired)
	}
	if e.ExceptionDate.IsZero() {
		return errors.New("exception_date is required")
	}
	if e.Reason != nil && utf8.RuneCountInString(*e.Reason) > scheduleReasonMaxLength {
		return errors.New("reason cannot exceed 255 characters")
	}
	if e.ExcusedReason != nil && utf8.RuneCountInString(*e.ExcusedReason) > scheduleReasonMaxLength {
		return errors.New("excused_reason cannot exceed 255 characters")
	}
	switch {
	case e.ExcusedFrom == nil:
		if e.ExcusedReason != nil || e.ExcusedCreatedBy != nil || e.ExcusedOwnsPickupTime || e.ExcusedAuto {
			return errors.New("partial absence metadata requires excused_from")
		}
	case e.ExcusedAuto:
		if e.ExcusedCreatedBy != nil {
			return errors.New("auto partial absences must not carry excused_created_by")
		}
		if e.ExcusedOwnsPickupTime {
			return errors.New("auto partial absences never own the pickup time")
		}
		if e.PickupTime == nil {
			return errors.New("auto partial absences require a pickup time")
		}
	case e.ExcusedCreatedBy == nil || *e.ExcusedCreatedBy <= 0:
		return errors.New("excused_created_by is required for partial absences")
	case e.ExcusedOwnsPickupTime && e.PickupTime == nil:
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

// ports.StudentPickupScheduleRecords defines operations for managing student pickup schedules

// ports.StudentPickupExceptionRecords defines operations for managing student pickup exceptions

// StudentPickupNote represents a date-specific note for a student's pickup
type StudentPickupNote struct {
	Model `bun:"schema:schedule,table:student_pickup_notes"`
	TenantModel

	StudentID int64 `bun:"student_id,notnull" json:"student_id"`
	// NoteDate is empty for a recurring weekday note (#3369): the row then
	// carries Weekday (1-5) instead and applies to every such weekday. A
	// recurring note says nothing about attendance, so a day the child does
	// not come can still carry one.
	NoteDate  calendar.Date `bun:"note_date,nullzero" json:"note_date,omitempty"`
	Weekday   int           `bun:"weekday,nullzero" json:"weekday,omitempty"`
	Content   string        `bun:"content,notnull" json:"content"`
	CreatedBy int64         `bun:"created_by,notnull" json:"created_by"`
}

// ports.StudentPickupNoteRecords defines operations for managing student pickup notes
