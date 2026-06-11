package active

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const tableStaffVacationQuota = "active.staff_vacation_quota"

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

func (q *StaffVacationQuota) BeforeAppendModel(query any) error {
	if qb, ok := query.(*bun.UpdateQuery); ok {
		qb.ModelTableExpr(tableStaffVacationQuota)
	}
	if qb, ok := query.(*bun.DeleteQuery); ok {
		qb.ModelTableExpr(tableStaffVacationQuota)
	}
	if qb, ok := query.(*bun.InsertQuery); ok {
		qb.ModelTableExpr(tableStaffVacationQuota)
	}
	return nil
}

func (q *StaffVacationQuota) GetID() any              { return q.ID }
func (q *StaffVacationQuota) GetCreatedAt() time.Time { return q.CreatedAt }
func (q *StaffVacationQuota) GetUpdatedAt() time.Time { return q.UpdatedAt }
func (q *StaffVacationQuota) TableName() string       { return tableStaffVacationQuota }

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
	return nil
}
