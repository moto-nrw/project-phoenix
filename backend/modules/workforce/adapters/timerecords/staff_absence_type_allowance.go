package timerecords

import (
	"github.com/moto-nrw/project-phoenix/models/base"
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
