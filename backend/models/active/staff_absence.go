package active

import (
	"errors"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
)

// Absence duration constants.
const (
	// minAbsenceDays is the floor: an absence always spans at least one day.
	minAbsenceDays = 1
)

// AbsenceType constants
const (
	AbsenceTypeSick     = "sick"
	AbsenceTypeVacation = "vacation"
	AbsenceTypeTraining = "training"
	AbsenceTypeOther    = "other"
	// AbsenceTypeCompTime is Freizeitausgleich (#1420 5b): the day keeps its
	// Soll but is deliberately NOT credited, so approving it reduces the
	// Stundenkonto by the day's contractual target.
	AbsenceTypeCompTime = "comp_time"
)

// AbsenceStatus constants. `reported` = admin-direct entry (skips approval),
// `requested` = vacation request pending approval, `question` = Rückfrage from
// the Leitung awaiting the staff member's amended resubmit, `approved`/
// `declined` = post-decision terminal states, `canceled` = MA withdrew before
// decision.
const (
	AbsenceStatusReported  = "reported"
	AbsenceStatusRequested = "requested"
	AbsenceStatusQuestion  = "question"
	AbsenceStatusApproved  = "approved"
	AbsenceStatusDeclined  = "declined"
	AbsenceStatusCanceled  = "canceled"
)

// ValidAbsenceTypes lists all valid absence types
var ValidAbsenceTypes = []string{
	AbsenceTypeSick,
	AbsenceTypeVacation,
	AbsenceTypeTraining,
	AbsenceTypeOther,
	AbsenceTypeCompTime,
}

// ValidAbsenceStatuses lists all valid absence statuses
var ValidAbsenceStatuses = []string{
	AbsenceStatusReported,
	AbsenceStatusRequested,
	AbsenceStatusQuestion,
	AbsenceStatusApproved,
	AbsenceStatusDeclined,
	AbsenceStatusCanceled,
}

// StaffAbsence represents a staff absence record (sick, vacation, etc.)
type StaffAbsence struct {
	base.Model `bun:"schema:active,table:staff_absences"`
	base.TenantModel
	StaffID     int64  `bun:"staff_id,notnull" json:"staff_id"`
	AbsenceType string `bun:"absence_type,notnull" json:"absence_type"`
	// AbsenceTypeID optionally names this absence with a school-defined
	// Abwesenheitsart (#2403). It never changes the arithmetic: the row still
	// carries the canonical AbsenceType (always AbsenceTypeOther while a custom
	// art is attached, enforced by chk_sa_custom_type_is_other) and every
	// calculation keeps reading that column. NULL = one of the five standard
	// types.
	AbsenceTypeID *int64 `bun:"absence_type_id" json:"absence_type_id,omitempty"`
	// AbsenceTypeLabel is the resolved display name — the custom art's name
	// when AbsenceTypeID is set, otherwise empty so clients fall back to their
	// own label for the standard type. Not a column; filled by the service
	// layer on read paths.
	AbsenceTypeLabel  string        `bun:"-" json:"absence_type_label,omitempty"`
	DateStart         timezone.Date `bun:"date_start,notnull,type:date" json:"date_start"`
	DateEnd           timezone.Date `bun:"date_end,notnull,type:date" json:"date_end"`
	HalfDay           bool          `bun:"half_day,notnull,default:false" json:"half_day"`
	StartHalfDay      bool          `bun:"start_half_day,notnull,default:false" json:"start_half_day"`
	EndHalfDay        bool          `bun:"end_half_day,notnull,default:false" json:"end_half_day"`
	Note              string        `bun:"note" json:"note,omitempty"`
	Status            string        `bun:"status,notnull,default:'reported'" json:"status"`
	ApprovedBy        *int64        `bun:"approved_by" json:"approved_by,omitempty"`
	ApprovedAt        *time.Time    `bun:"approved_at" json:"approved_at,omitempty"`
	CreatedBy         int64         `bun:"created_by,notnull" json:"created_by"`
	WorkingDays       *float64      `bun:"working_days" json:"working_days,omitempty"`
	DecisionNote      string        `bun:"decision_note" json:"decision_note,omitempty"`
	RequestedAt       time.Time     `bun:"requested_at,notnull,default:current_timestamp" json:"requested_at"`
	SubstituteStaffID *int64        `bun:"substitute_staff_id" json:"substitute_staff_id,omitempty"`
}

// Validate validates the absence record
func (sa *StaffAbsence) Validate() error {
	if sa.StaffID <= 0 {
		return errors.New("staff ID is required")
	}
	if !isValidAbsenceType(sa.AbsenceType) {
		return errors.New("invalid absence type")
	}
	if !isValidAbsenceStatus(sa.Status) {
		return errors.New("invalid absence status")
	}
	if sa.DateStart.IsZero() {
		return errors.New("date_start is required")
	}
	if sa.DateEnd.IsZero() {
		return errors.New("date_end is required")
	}
	if sa.DateStart.After(sa.DateEnd) {
		return errors.New("date_start must be before or equal to date_end")
	}
	if sa.CreatedBy <= 0 {
		return errors.New("created_by is required")
	}
	return nil
}

// DurationDays returns the number of days this absence spans
func (sa *StaffAbsence) DurationDays() int {
	days := sa.DateStart.DaysUntil(sa.DateEnd) + minAbsenceDays
	if days < minAbsenceDays {
		return minAbsenceDays
	}
	return days
}

func isValidAbsenceType(t string) bool {
	return slices.Contains(ValidAbsenceTypes, t)
}

func isValidAbsenceStatus(s string) bool {
	return slices.Contains(ValidAbsenceStatuses, s)
}
