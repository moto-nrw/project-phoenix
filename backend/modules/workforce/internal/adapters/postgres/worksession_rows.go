package postgres

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/uptrace/bun"
)

type workSessionRow struct {
	bun.BaseModel  `bun:"table:active.work_sessions,alias:work_session"`
	ID             int64        `bun:"id,pk,autoincrement"`
	TenantID       int64        `bun:"tenant_id,notnull"`
	StaffID        int64        `bun:"staff_id,notnull"`
	Date           calendarDate `bun:"date,notnull,type:date"`
	Status         string       `bun:"status,notnull,default:'present'"`
	Source         string       `bun:"source,notnull,default:'app'"`
	CheckInTime    time.Time    `bun:"check_in_time,notnull"`
	CheckOutTime   *time.Time   `bun:"check_out_time"`
	ReopenedAt     *time.Time   `bun:"reopened_at"`
	BreakMinutes   int          `bun:"break_minutes,notnull,default:0"`
	Notes          string       `bun:"notes"`
	AutoCheckedOut bool         `bun:"auto_checked_out,notnull,default:false"`
	CreatedBy      int64        `bun:"created_by,notnull"`
	UpdatedBy      *int64       `bun:"updated_by"`
	CreatedAt      time.Time    `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt      time.Time    `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

type workSessionBreakRow struct {
	bun.BaseModel   `bun:"table:active.work_session_breaks,alias:work_session_break"`
	ID              int64      `bun:"id,pk,autoincrement"`
	TenantID        int64      `bun:"tenant_id,notnull"`
	SessionID       int64      `bun:"session_id,notnull"`
	StartedAt       time.Time  `bun:"started_at,notnull"`
	EndedAt         *time.Time `bun:"ended_at"`
	DurationMinutes int        `bun:"duration_minutes,notnull,default:0"`
	PlannedEndTime  *time.Time `bun:"planned_end_time"`
	CreatedAt       time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt       time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

type staffBalanceAdjustmentRow struct {
	bun.BaseModel `bun:"table:active.staff_balance_adjustments,alias:staff_balance_adjustment"`
	ID            int64        `bun:"id,pk,autoincrement"`
	TenantID      int64        `bun:"tenant_id,notnull"`
	StaffID       int64        `bun:"staff_id,notnull"`
	Type          string       `bun:"type,notnull"`
	MinutesDelta  int          `bun:"minutes_delta,notnull"`
	EffectiveDate calendarDate `bun:"effective_date,notnull,type:date"`
	Note          string       `bun:"note,notnull,default:''"`
	DecidedBy     int64        `bun:"decided_by,notnull"`
	DecidedAt     time.Time    `bun:"decided_at,nullzero,notnull,default:current_timestamp"`
	CreatedAt     time.Time    `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time    `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

type staffVacationOpeningRow struct {
	bun.BaseModel        `bun:"table:active.staff_vacation_openings,alias:staff_vacation_opening"`
	ID                   int64        `bun:"id,pk,autoincrement"`
	TenantID             int64        `bun:"tenant_id,notnull"`
	StaffID              int64        `bun:"staff_id,notnull"`
	Year                 int          `bun:"year,notnull"`
	EffectiveDate        calendarDate `bun:"effective_date,notnull,type:date"`
	TakenBeforeDays      float64      `bun:"taken_before_days,notnull"`
	EnteredRemainingDays float64      `bun:"entered_remaining_days,notnull"`
	Note                 string       `bun:"note,notnull,default:''"`
	DecidedBy            int64        `bun:"decided_by,notnull"`
	DecidedAt            time.Time    `bun:"decided_at,nullzero,notnull,default:current_timestamp"`
	CreatedAt            time.Time    `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt            time.Time    `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

type staffVacationQuotaRow struct {
	bun.BaseModel `bun:"table:active.staff_vacation_quota,alias:staff_vacation_quota"`
	ID            int64     `bun:"id,pk,autoincrement"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	StaffID       int64     `bun:"staff_id,notnull"`
	Year          int       `bun:"year,notnull"`
	EntitledDays  float64   `bun:"entitled_days,notnull,default:30"`
	CarryoverDays float64   `bun:"carryover_days,notnull,default:0"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

type staffVacationQuotaChangeRow struct {
	bun.BaseModel    `bun:"table:active.staff_vacation_quota_changes"`
	TenantID         int64    `bun:"tenant_id"`
	StaffID          int64    `bun:"staff_id"`
	Year             int      `bun:"year"`
	OldEntitledDays  *float64 `bun:"old_entitled_days"`
	NewEntitledDays  float64  `bun:"new_entitled_days"`
	OldCarryoverDays *float64 `bun:"old_carryover_days"`
	NewCarryoverDays float64  `bun:"new_carryover_days"`
	Reason           string   `bun:"reason"`
	ChangedBy        int64    `bun:"changed_by"`
}

// --- mapping ---

func workSessionFromDomain(value workforce.WorkSession) *workSessionRow {
	return &workSessionRow{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, Date: calendarDate(value.Date),
		Status: value.Status, Source: value.Source, CheckInTime: value.CheckInTime, CheckOutTime: value.CheckOutTime,
		ReopenedAt: value.ReopenedAt, BreakMinutes: value.BreakMinutes, Notes: value.Notes,
		AutoCheckedOut: value.AutoCheckedOut, CreatedBy: value.CreatedBy, UpdatedBy: value.UpdatedBy,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func workSessionToDomain(row workSessionRow) workforce.WorkSession {
	return workforce.WorkSession{
		ID: row.ID, TenantID: row.TenantID, StaffID: row.StaffID, Date: string(row.Date),
		Status: row.Status, Source: row.Source, CheckInTime: row.CheckInTime, CheckOutTime: row.CheckOutTime,
		ReopenedAt: row.ReopenedAt, BreakMinutes: row.BreakMinutes, Notes: row.Notes,
		AutoCheckedOut: row.AutoCheckedOut, CreatedBy: row.CreatedBy, UpdatedBy: row.UpdatedBy,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func workSessionsToDomain(rows []workSessionRow) []workforce.WorkSession {
	result := make([]workforce.WorkSession, 0, len(rows))
	for _, row := range rows {
		result = append(result, workSessionToDomain(row))
	}
	return result
}

func workSessionBreakFromDomain(value workforce.WorkSessionBreak) *workSessionBreakRow {
	return &workSessionBreakRow{
		ID: value.ID, TenantID: value.TenantID, SessionID: value.SessionID, StartedAt: value.StartedAt,
		EndedAt: value.EndedAt, DurationMinutes: value.DurationMinutes, PlannedEndTime: value.PlannedEndTime,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func workSessionBreakToDomain(row workSessionBreakRow) workforce.WorkSessionBreak {
	return workforce.WorkSessionBreak{
		ID: row.ID, TenantID: row.TenantID, SessionID: row.SessionID, StartedAt: row.StartedAt,
		EndedAt: row.EndedAt, DurationMinutes: row.DurationMinutes, PlannedEndTime: row.PlannedEndTime,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func workSessionBreaksToDomain(rows []workSessionBreakRow) []workforce.WorkSessionBreak {
	result := make([]workforce.WorkSessionBreak, 0, len(rows))
	for _, row := range rows {
		result = append(result, workSessionBreakToDomain(row))
	}
	return result
}

func staffBalanceAdjustmentFromDomain(value workforce.StaffBalanceAdjustment) *staffBalanceAdjustmentRow {
	return &staffBalanceAdjustmentRow{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, Type: value.Type, MinutesDelta: value.MinutesDelta,
		EffectiveDate: calendarDate(value.EffectiveDate), Note: value.Note, DecidedBy: value.DecidedBy, DecidedAt: value.DecidedAt,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func staffBalanceAdjustmentToDomain(row staffBalanceAdjustmentRow) workforce.StaffBalanceAdjustment {
	return workforce.StaffBalanceAdjustment{
		ID: row.ID, TenantID: row.TenantID, StaffID: row.StaffID, Type: row.Type, MinutesDelta: row.MinutesDelta,
		EffectiveDate: string(row.EffectiveDate), Note: row.Note, DecidedBy: row.DecidedBy, DecidedAt: row.DecidedAt,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func staffVacationOpeningFromDomain(value domain.StaffVacationOpening) *staffVacationOpeningRow {
	return &staffVacationOpeningRow{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, Year: value.Year,
		EffectiveDate: calendarDate(value.EffectiveDate), TakenBeforeDays: value.TakenBeforeDays,
		EnteredRemainingDays: value.EnteredRemainingDays, Note: value.Note, DecidedBy: value.DecidedBy,
		DecidedAt: value.DecidedAt, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func staffVacationOpeningToDomain(row staffVacationOpeningRow) domain.StaffVacationOpening {
	return domain.StaffVacationOpening{
		ID: row.ID, TenantID: row.TenantID, StaffID: row.StaffID, Year: row.Year,
		EffectiveDate: string(row.EffectiveDate), TakenBeforeDays: row.TakenBeforeDays,
		EnteredRemainingDays: row.EnteredRemainingDays, Note: row.Note, DecidedBy: row.DecidedBy,
		DecidedAt: row.DecidedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func staffVacationQuotaFromDomain(value workforce.StaffVacationQuota) *staffVacationQuotaRow {
	return &staffVacationQuotaRow{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, Year: value.Year,
		EntitledDays: value.EntitledDays, CarryoverDays: value.CarryoverDays,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func staffVacationQuotaToDomain(row staffVacationQuotaRow) workforce.StaffVacationQuota {
	return workforce.StaffVacationQuota{
		ID: row.ID, TenantID: row.TenantID, StaffID: row.StaffID, Year: row.Year,
		EntitledDays: row.EntitledDays, CarryoverDays: row.CarryoverDays,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}
