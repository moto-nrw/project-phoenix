package active

import (
	"errors"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

const tableActiveStaffAbsences = "active.staff_absences"

// AbsenceType constants
const (
	AbsenceTypeSick     = "sick"
	AbsenceTypeVacation = "vacation"
	AbsenceTypeTraining = "training"
	AbsenceTypeOther    = "other"
)

// AbsenceStatus constants
const (
	AbsenceStatusReported = "reported"
	AbsenceStatusApproved = "approved"
	AbsenceStatusDeclined = "declined"
)

// ValidAbsenceTypes lists all valid absence types
var ValidAbsenceTypes = []string{
	AbsenceTypeSick,
	AbsenceTypeVacation,
	AbsenceTypeTraining,
	AbsenceTypeOther,
}

// ValidAbsenceStatuses lists all valid absence statuses
var ValidAbsenceStatuses = []string{
	AbsenceStatusReported,
	AbsenceStatusApproved,
	AbsenceStatusDeclined,
}

// StaffAbsence represents a staff absence record (sick, vacation, etc.)
type StaffAbsence struct {
	base.Model  `bun:"schema:active,table:staff_absences"`
	StaffID     int64      `bun:"staff_id,notnull" json:"staff_id"`
	AbsenceType string     `bun:"absence_type,notnull" json:"absence_type"`
	DateStart   time.Time  `bun:"date_start,notnull,type:date" json:"date_start"`
	DateEnd     time.Time  `bun:"date_end,notnull,type:date" json:"date_end"`
	HalfDay     bool       `bun:"half_day,notnull,default:false" json:"half_day"`
	Note        string     `bun:"note" json:"note,omitempty"`
	Status      string     `bun:"status,notnull,default:'reported'" json:"status"`
	ApprovedBy  *int64     `bun:"approved_by" json:"approved_by,omitempty"`
	ApprovedAt  *time.Time `bun:"approved_at" json:"approved_at,omitempty"`
	CreatedBy   int64      `bun:"created_by,notnull" json:"created_by"`

	Staff *users.Staff `bun:"rel:belongs-to,join:staff_id=id" json:"staff,omitempty"`
}

// BeforeAppendModel implements the model hook for schema-qualified queries
func (sa *StaffAbsence) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(tableActiveStaffAbsences)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(tableActiveStaffAbsences)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(tableActiveStaffAbsences)
	}
	if q, ok := query.(*bun.InsertQuery); ok {
		q.ModelTableExpr(tableActiveStaffAbsences)
	}
	return nil
}

func (sa *StaffAbsence) GetID() any              { return sa.ID }
func (sa *StaffAbsence) GetCreatedAt() time.Time { return sa.CreatedAt }
func (sa *StaffAbsence) GetUpdatedAt() time.Time { return sa.UpdatedAt }
func (sa *StaffAbsence) TableName() string       { return tableActiveStaffAbsences }

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
	days := int(sa.DateEnd.Sub(sa.DateStart).Hours()/24) + 1
	if days < 1 {
		return 1
	}
	return days
}

func isValidAbsenceType(t string) bool {
	return slices.Contains(ValidAbsenceTypes, t)
}

func isValidAbsenceStatus(s string) bool {
	return slices.Contains(ValidAbsenceStatuses, s)
}
