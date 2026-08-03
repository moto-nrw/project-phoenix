package active

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
)

// Validation bounds for a vacation opening. NUMERIC(5,1) caps the magnitude;
// the business range is far smaller, but the takeover derives taken_before
// from entitled + carryover − entered rest, so both signs are legitimate.
const (
	maxOpeningDays = 999.0
)

// StaffVacationOpening records vacation days a staff member had already taken
// before the moto introduction (#2132), per calendar year. The row itself is
// the audit record — Stichtag, note, decided_by, decided_at — mirroring
// StaffBalanceAdjustment. Remaining vacation subtracts TakenBeforeDays
// alongside in-app absences, so the Jahresanspruch stays untouched and later
// entitlement corrections shift the Resturlaub coherently.
//
// TakenBeforeDays is SIGNED: an entered Resturlaub above entitled + carryover
// yields a negative value (extra credit brought over from the old system).
// EnteredRemainingDays documents what the admin actually typed.
type StaffVacationOpening struct {
	base.Model `bun:"schema:active,table:staff_vacation_openings"`
	base.TenantModel
	StaffID              int64         `bun:"staff_id,notnull" json:"staff_id"`
	Year                 int           `bun:"year,notnull" json:"year"`
	EffectiveDate        timezone.Date `bun:"effective_date,notnull,type:date" json:"effective_date"`
	TakenBeforeDays      float64       `bun:"taken_before_days,notnull" json:"taken_before_days"`
	EnteredRemainingDays float64       `bun:"entered_remaining_days,notnull" json:"entered_remaining_days"`
	Note                 string        `bun:"note,notnull,default:''" json:"note"`
	DecidedBy            int64         `bun:"decided_by,notnull" json:"decided_by"`
	DecidedAt            time.Time     `bun:"decided_at,notnull,default:current_timestamp" json:"decided_at"`
}

func (o *StaffVacationOpening) Validate() error {
	if o.StaffID <= 0 {
		return errors.New("staff_id is required")
	}
	if o.Year < minQuotaYear || o.Year > maxQuotaYear {
		return errors.New("year out of range")
	}
	if o.EffectiveDate.IsZero() {
		return errors.New("effective_date is required")
	}
	if o.EffectiveDate.Year != o.Year {
		return errors.New("effective_date must lie in the opening year")
	}
	if o.TakenBeforeDays < -maxOpeningDays || o.TakenBeforeDays > maxOpeningDays {
		return errors.New("taken_before_days out of range")
	}
	if o.EnteredRemainingDays < -maxOpeningDays || o.EnteredRemainingDays > maxOpeningDays {
		return errors.New("entered_remaining_days out of range")
	}
	if o.DecidedBy <= 0 {
		return errors.New("decided_by is required")
	}
	return nil
}
