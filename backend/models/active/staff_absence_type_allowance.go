package active

import (
	"errors"
	"math"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/base"
)

const (
	minAllowanceYear = 2000
	maxAllowanceYear = 2100
	maxAllowanceDays = 366
)

// StaffAbsenceTypeAllowance is one person's yearly claim for one
// school-defined absence type. A missing row means a claim of zero days.
type StaffAbsenceTypeAllowance struct {
	base.Model `bun:"schema:active,table:staff_absence_type_allowances"`
	base.TenantModel
	StaffID       int64   `bun:"staff_id,notnull" json:"staff_id"`
	AbsenceTypeID int64   `bun:"absence_type_id,notnull" json:"absence_type_id"`
	Year          int     `bun:"year,notnull" json:"year"`
	EntitledDays  float64 `bun:"entitled_days,notnull" json:"entitled_days"`
}

func (a *StaffAbsenceTypeAllowance) Validate() error {
	if a.StaffID <= 0 {
		return errors.New("staff_id is required")
	}
	if a.AbsenceTypeID <= 0 {
		return errors.New("absence_type_id is required")
	}
	if a.Year < minAllowanceYear || a.Year > maxAllowanceYear {
		return errors.New("year out of range")
	}
	if a.EntitledDays < 0 || a.EntitledDays > maxAllowanceDays {
		return errors.New("entitled_days out of range")
	}
	if math.Abs(a.EntitledDays*2-math.Round(a.EntitledDays*2)) > 0.000001 {
		return errors.New("entitled_days must use whole or half days")
	}
	return nil
}

// StaffAbsenceTypeAllowanceChange is the append-only reason trail for a claim
// create or correction. OldEntitledDays is nil for the first assignment.
type StaffAbsenceTypeAllowanceChange struct {
	base.Model `bun:"schema:active,table:staff_absence_type_allowance_changes"`
	base.TenantModel
	StaffID         int64    `bun:"staff_id,notnull" json:"staff_id"`
	AbsenceTypeID   int64    `bun:"absence_type_id,notnull" json:"absence_type_id"`
	Year            int      `bun:"year,notnull" json:"year"`
	OldEntitledDays *float64 `bun:"old_entitled_days" json:"old_entitled_days,omitempty"`
	NewEntitledDays float64  `bun:"new_entitled_days,notnull" json:"new_entitled_days"`
	Reason          string   `bun:"reason,notnull" json:"reason"`
	ChangedBy       int64    `bun:"changed_by,notnull" json:"changed_by"`
}

func (c *StaffAbsenceTypeAllowanceChange) Validate() error {
	allowance := StaffAbsenceTypeAllowance{
		StaffID: c.StaffID, AbsenceTypeID: c.AbsenceTypeID,
		Year: c.Year, EntitledDays: c.NewEntitledDays,
	}
	if err := allowance.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(c.Reason) == "" {
		return errors.New("reason is required")
	}
	if c.ChangedBy <= 0 {
		return errors.New("changed_by is required")
	}
	return nil
}
