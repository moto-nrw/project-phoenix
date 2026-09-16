package active

import (
	"errors"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
)

// BalanceAdjustmentType constants (#1420). Each row is a Stundenkonto
// correction transaction, never a normal time entry:
//   - payout:    plus hours paid out (future DATEV Lohnart "Überstunden-Auszahlung")
//   - comp_time: a lump-sum Freizeitausgleich grant not tied to a workday
//     (the day-based variant is the comp_time ABSENCE type, which reduces
//     the balance via its uncredited Soll — never both for the same grant)
//   - reset:     school-year reset; delta = carryover − closing balance
//   - opening:   go-live opening balance (#2132); delta = target − closing
//     balance as of the Stichtag. The only type whose resulting balance may
//     be negative — migrated accounts start where the old system left them.
const (
	BalanceAdjustmentTypePayout   = "payout"
	BalanceAdjustmentTypeCompTime = "comp_time"
	BalanceAdjustmentTypeReset    = "reset"
	BalanceAdjustmentTypeOpening  = "opening"
)

// ValidBalanceAdjustmentTypes lists all valid balance adjustment types.
var ValidBalanceAdjustmentTypes = []string{
	BalanceAdjustmentTypePayout,
	BalanceAdjustmentTypeCompTime,
	BalanceAdjustmentTypeReset,
	BalanceAdjustmentTypeOpening,
}

// StaffBalanceAdjustment is one Stundenkonto correction transaction (#1420).
// The row itself is the audit record: decided_by/decided_at/note document who
// granted what and why. MinutesDelta is signed — reductions (payout,
// comp-time grant, a reset of a positive balance) are negative.
type StaffBalanceAdjustment struct {
	base.Model `bun:"schema:active,table:staff_balance_adjustments"`
	base.TenantModel
	StaffID       int64         `bun:"staff_id,notnull" json:"staff_id"`
	Type          string        `bun:"type,notnull" json:"type"`
	MinutesDelta  int           `bun:"minutes_delta,notnull" json:"minutes_delta"`
	EffectiveDate timezone.Date `bun:"effective_date,notnull,type:date" json:"effective_date"`
	Note          string        `bun:"note,notnull,default:''" json:"note"`
	DecidedBy     int64         `bun:"decided_by,notnull" json:"decided_by"`
	DecidedAt     time.Time     `bun:"decided_at,notnull,default:current_timestamp" json:"decided_at"`
}

// Validate validates the adjustment record.
func (a *StaffBalanceAdjustment) Validate() error {
	if a.StaffID <= 0 {
		return errors.New("staff ID is required")
	}
	if !slices.Contains(ValidBalanceAdjustmentTypes, a.Type) {
		return errors.New("invalid balance adjustment type")
	}
	if a.EffectiveDate.IsZero() {
		return errors.New("effective_date is required")
	}
	if a.DecidedBy <= 0 {
		return errors.New("decided_by is required")
	}
	return nil
}
