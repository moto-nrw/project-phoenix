package active

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Validation bounds for a staff vacation quota.
const (
	// minQuotaYear / maxQuotaYear bound the calendar year a quota may target.
	minQuotaYear = 2000
	maxQuotaYear = 2100
	// minQuotaDays / maxQuotaDays bound entitled and carryover day counts.
	minQuotaDays = 0
	maxQuotaDays = 366
)

type StaffVacationQuota struct {
	base.Model `bun:"schema:active,table:staff_vacation_quota"`
	base.TenantModel
	StaffID       int64   `bun:"staff_id,notnull" json:"staff_id"`
	Year          int     `bun:"year,notnull" json:"year"`
	EntitledDays  float64 `bun:"entitled_days,notnull,default:30" json:"entitled_days"`
	CarryoverDays float64 `bun:"carryover_days,notnull,default:0" json:"carryover_days"`
}

func (q *StaffVacationQuota) Validate() error {
	if q.StaffID <= 0 {
		return errors.New("staff_id is required")
	}
	if q.Year < minQuotaYear || q.Year > maxQuotaYear {
		return errors.New("year out of range")
	}
	if q.EntitledDays < minQuotaDays || q.EntitledDays > maxQuotaDays {
		return errors.New("entitled_days out of range")
	}
	if q.CarryoverDays < minQuotaDays || q.CarryoverDays > maxQuotaDays {
		return errors.New("carryover_days out of range")
	}
	if !hasOneDecimalPlace(q.EntitledDays) || !hasOneDecimalPlace(q.CarryoverDays) {
		return errors.New("vacation quota days must have at most one decimal place")
	}
	return nil
}
