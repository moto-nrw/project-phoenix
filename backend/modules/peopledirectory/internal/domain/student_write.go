package domain

import (
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
)

// ErrCompanionWouldLoseDeparture refuses a plan change that would leave a
// linked child with an accompanied weekday and no remaining "mit wem" detail.
var ErrCompanionWouldLoseDeparture = errors.New("companion would lose its departure detail")

// ErrCompanionLockBusy reports a companion row another transaction holds. It is
// retriable on purpose: the alternative to refusing is a deadlock abort.
var ErrCompanionLockBusy = errors.New("companion row is locked by another write")

// CompanionWeekdayKeys maps a stored edge's weekday number onto the key the
// departure plan uses. Edges only ever carry 1..5.
var CompanionWeekdayKeys = map[int]string{
	1: departure.PickupDayMonday,
	2: departure.PickupDayTuesday,
	3: departure.PickupDayWednesday,
	4: departure.PickupDayThursday,
	5: departure.PickupDayFriday,
}

// CompanionEdge is one stored "läuft mit" link, as the owner needs to read it:
// the two children and the weekday it applies to. Care Plan owns the row.
type CompanionEdge struct {
	ID                 int64
	StudentID          int64
	CompanionStudentID int64
	Weekday            int
}

// Other answers the child at the far end of this edge, or false when the edge
// does not touch the given child at all.
func (e CompanionEdge) Other(studentID int64) (int64, bool) {
	switch studentID {
	case e.StudentID:
		return e.CompanionStudentID, true
	case e.CompanionStudentID:
		return e.StudentID, true
	default:
		return 0, false
	}
}

// CompanionTrim is the outcome of reconciling the links against a plan: the
// edges the plan no longer allows, and the weekdays on which the child keeps a
// link — its structured "mit wem" answer, per day.
type CompanionTrim struct {
	DropIDs  []int64
	KeptDays map[string]bool
}

// The batch a coordinated multi-child write defers its verdicts into. It is a
// leaf type because the caller that opens the scope and the owner that decides
// the verdicts must both be able to name it.
type StrandingBatch = departure.StrandingBatch

var (
	ContextWithStrandingBatch = departure.ContextWithStrandingBatch
	StrandingBatchFromContext = departure.StrandingBatchFromContext
)

// HasCompanionNote reports whether the free-text "mit wem" note carries the
// detail. A blank note is no answer.
func HasCompanionNote(note *string) bool {
	return note != nil && strings.TrimSpace(*note) != ""
}

// ErrLockNotAvailable reports a row another transaction already holds. The
// persistence adapter recognizes the driver's code and reports it as this.
var ErrLockNotAvailable = errors.New("row lock is not available")

// ValidateStudentRecord checks a child before it is written.
//
// supplied is the plan as the caller sent it and resolved is the plan that will
// actually be persisted. Both are checked: a caller that names a weekday the
// week does not have must be told so, even though resolving the plan would
// quietly drop it, and the "mit wem" invariant only makes sense against what
// ends up stored.
//
// covered names the weekdays whose "mit wem" a structured companion link
// answers; it is additive, so a day it covers needs no free-text note.
func ValidateStudentRecord(
	record StudentRecord,
	supplied, resolved DeparturePlan,
	note *string,
	covered map[string]bool,
) error {
	if record.PersonID <= 0 {
		return invalidStudentRecord("person ID is required")
	}
	if strings.TrimSpace(record.SchoolClass) == "" {
		return invalidStudentRecord("school class is required")
	}
	if err := validateStudentContact(record.GuardianEmail, record.GuardianPhone); err != nil {
		return err
	}
	for _, plan := range []DeparturePlan{supplied, resolved} {
		if err := validateDeparturePlan(plan); err != nil {
			return err
		}
	}
	_, err := departure.NormalizeCompanionNote(
		resolved.DepartureDays, resolved.AllowedDepartureModes, note, covered)
	return err
}

func validateDeparturePlan(plan DeparturePlan) error {
	if err := plan.DepartureDays.Validate(); err != nil {
		return err
	}
	if err := plan.AllowedDepartureModes.Validate(); err != nil {
		return err
	}
	if err := plan.BusDays.Validate(); err != nil {
		return err
	}
	return plan.PickupDays.Validate()
}

// validateStudentContact keeps the two retained guardian contact columns
// well-formed. They predate the guardian tables and are still imported into.
func validateStudentContact(email, phone *string) error {
	if email != nil {
		if trimmed := strings.TrimSpace(*email); trimmed != "" && !validEmail(trimmed) {
			return invalidStudentRecord("invalid guardian email format")
		}
	}
	if phone != nil {
		if trimmed := strings.TrimSpace(*phone); trimmed != "" && !validPhone(trimmed) {
			return invalidStudentRecord("invalid guardian phone format")
		}
	}
	return nil
}

// validEmail is deliberately the shape check the retained model applied, not a
// deliverability test: the column also holds values imported from a school's
// own records.
func validEmail(value string) bool {
	at := strings.Index(value, "@")
	if at <= 0 || at == len(value)-1 {
		return false
	}
	domain := value[at+1:]
	dot := strings.LastIndex(domain, ".")
	return dot > 0 && dot < len(domain)-1 && !strings.Contains(domain, "@")
}

// validPhone accepts the digits, separators and leading + a written phone
// number uses; anything else is a typo rather than a number.
func validPhone(value string) bool {
	digits := 0
	for _, symbol := range value {
		switch {
		case symbol >= '0' && symbol <= '9':
			digits++
		case strings.ContainsRune("+-/() .", symbol):
		default:
			return false
		}
	}
	return digits >= 3
}

// InvalidStudentRecordError refuses a child whose own fields do not hold up.
type InvalidStudentRecordError struct{ Reason string }

func (e *InvalidStudentRecordError) Error() string { return e.Reason }
func (e *InvalidStudentRecordError) Unwrap() error { return ErrStudentInvalid }

// ErrStudentInvalid is the sentinel every record refusal unwraps to.
var ErrStudentInvalid = errors.New("invalid student")

func invalidStudentRecord(reason string) error { return &InvalidStudentRecordError{Reason: reason} }

// DepartureAccompanied is the "Mit anderem Kind" mode, re-exported so this
// package states the companion rules without reaching past itself.
const DepartureAccompanied = departure.DepartureAccompanied

// The two bounds of the enrolment interval, naming which one a due-for-status
// read compares against.
const (
	StudentBoundCareStart = "care_start"
	StudentBoundCareEnd   = "care_end"
)

// StudentStatusPending is a child enrolled for a start day still ahead.
const StudentStatusPending = "pending"
