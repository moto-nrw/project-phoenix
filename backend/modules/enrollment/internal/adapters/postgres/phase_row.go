package postgres

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/uptrace/bun"
)

type phaseRow struct {
	bun.BaseModel             `bun:"table:enrollment.phases,alias:phase"`
	ID                        int64           `bun:"id,pk,autoincrement"`
	TenantID                  int64           `bun:"tenant_id,notnull"`
	CreatedAt                 time.Time       `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt                 time.Time       `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	Name                      string          `bun:"name,notnull"`
	Kind                      string          `bun:"kind,notnull,default:'school_year'"`
	ServiceStartDate          enrollment.Date `bun:"service_start_date,notnull,type:date"`
	ServiceEndDate            enrollment.Date `bun:"service_end_date,notnull,type:date"`
	EnrollmentOpenAt          *time.Time      `bun:"enrollment_open_at"`
	EnrollmentCloseAt         *time.Time      `bun:"enrollment_close_at"`
	FormSchemaID              *int64          `bun:"form_schema_id"`
	CalendarPeriodID          *int64          `bun:"calendar_period_id"`
	ShowStatusReasonToParent  bool            `bun:"show_status_reason_to_parent,notnull"`
	CareOverflowMode          string          `bun:"care_overflow_mode,notnull"`
	CareOfferingSelectionMode string          `bun:"care_offering_selection_mode,notnull"`
	IsActive                  bool            `bun:"is_active,notnull"`
	RolloverSourcePhaseID     *int64          `bun:"rollover_source_phase_id"`
	RolloverMode              *string         `bun:"rollover_mode"`
	RolloverAutoApprove       bool            `bun:"rollover_auto_approve,notnull"`
	RolloverDeadline          *time.Time      `bun:"rollover_deadline"`
	RolloverBumpsGrade        bool            `bun:"rollover_bumps_grade,notnull"`
	AvailableSchoolClasses    []string        `bun:"available_school_classes,type:jsonb,notnull"`
	RequireSchoolClass        bool            `bun:"require_school_class,notnull"`
	Audience                  string          `bun:"audience,notnull,default:'open'"`
	EligibleSchoolClasses     []string        `bun:"eligible_school_classes,type:jsonb,notnull"`
	EligibleGradeLevels       []int           `bun:"eligible_grade_levels,type:jsonb,notnull"`
}

func (r *phaseRow) value() *enrollment.Phase {
	if r == nil {
		return nil
	}
	return &enrollment.Phase{
		ID:                        r.ID,
		TenantID:                  r.TenantID,
		CreatedAt:                 r.CreatedAt,
		UpdatedAt:                 r.UpdatedAt,
		Name:                      r.Name,
		Kind:                      r.Kind,
		ServiceStartDate:          r.ServiceStartDate,
		ServiceEndDate:            r.ServiceEndDate,
		EnrollmentOpenAt:          r.EnrollmentOpenAt,
		EnrollmentCloseAt:         r.EnrollmentCloseAt,
		FormSchemaID:              r.FormSchemaID,
		CalendarPeriodID:          r.CalendarPeriodID,
		ShowStatusReasonToParent:  r.ShowStatusReasonToParent,
		CareOverflowMode:          r.CareOverflowMode,
		CareOfferingSelectionMode: r.CareOfferingSelectionMode,
		IsActive:                  r.IsActive,
		RolloverSourcePhaseID:     r.RolloverSourcePhaseID,
		RolloverMode:              r.RolloverMode,
		RolloverAutoApprove:       r.RolloverAutoApprove,
		RolloverDeadline:          r.RolloverDeadline,
		RolloverBumpsGrade:        r.RolloverBumpsGrade,
		AvailableSchoolClasses:    r.AvailableSchoolClasses,
		RequireSchoolClass:        r.RequireSchoolClass,
		Audience:                  r.Audience,
		EligibleSchoolClasses:     r.EligibleSchoolClasses,
		EligibleGradeLevels:       r.EligibleGradeLevels,
	}
}
func phaseRecord(p *enrollment.Phase) *phaseRow {
	return &phaseRow{
		ID:                        p.ID,
		TenantID:                  p.TenantID,
		CreatedAt:                 p.CreatedAt,
		UpdatedAt:                 p.UpdatedAt,
		Name:                      p.Name,
		Kind:                      p.Kind,
		ServiceStartDate:          p.ServiceStartDate,
		ServiceEndDate:            p.ServiceEndDate,
		EnrollmentOpenAt:          p.EnrollmentOpenAt,
		EnrollmentCloseAt:         p.EnrollmentCloseAt,
		FormSchemaID:              p.FormSchemaID,
		CalendarPeriodID:          p.CalendarPeriodID,
		ShowStatusReasonToParent:  p.ShowStatusReasonToParent,
		CareOverflowMode:          p.CareOverflowMode,
		CareOfferingSelectionMode: p.CareOfferingSelectionMode,
		IsActive:                  p.IsActive,
		RolloverSourcePhaseID:     p.RolloverSourcePhaseID,
		RolloverMode:              p.RolloverMode,
		RolloverAutoApprove:       p.RolloverAutoApprove,
		RolloverDeadline:          p.RolloverDeadline,
		RolloverBumpsGrade:        p.RolloverBumpsGrade,
		AvailableSchoolClasses:    p.AvailableSchoolClasses,
		RequireSchoolClass:        p.RequireSchoolClass,
		Audience:                  p.Audience,
		EligibleSchoolClasses:     p.EligibleSchoolClasses,
		EligibleGradeLevels:       p.EligibleGradeLevels,
	}
}
func phaseValues(rows []*phaseRow) []*enrollment.Phase {
	if rows == nil {
		return nil
	}
	result := make([]*enrollment.Phase, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.value())
	}
	return result
}
