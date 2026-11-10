package postgres

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
)

type requestChildRow struct {
	bun.BaseModel         `bun:"table:enrollment.request_children,alias:request_child"`
	ID                    int64           `bun:"id,pk,autoincrement"`
	TenantID              int64           `bun:"tenant_id,notnull"`
	CreatedAt             time.Time       `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt             time.Time       `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	RequestID             int64           `bun:"request_id,notnull"`
	FirstName             string          `bun:"first_name,notnull"`
	LastName              string          `bun:"last_name,notnull"`
	DateOfBirth           calendar.Date   `bun:"date_of_birth,notnull,type:date"`
	TargetGradeLevel      *int16          `bun:"target_grade_level"`
	TargetSchoolClass     *string         `bun:"target_school_class"`
	HolidayCareWish       *bool           `bun:"holiday_care_wish"`
	CustomData            json.RawMessage `bun:"custom_data,type:jsonb,notnull,default:'{}'"`
	Status                string          `bun:"status,notnull,default:'submitted'"`
	StatusReason          *string         `bun:"status_reason"`
	ActivationMode        string          `bun:"activation_mode,notnull,default:'scheduled'"`
	ActivateOn            *calendar.Date  `bun:"activate_on,type:date"`
	ReviewedAt            *time.Time      `bun:"reviewed_at"`
	ReviewedBy            *int64          `bun:"reviewed_by"`
	CreatedStudentID      *int64          `bun:"created_student_id"`
	MatchedStudentID      *int64          `bun:"matched_student_id"`
	MatchedByOffice       bool            `bun:"matched_by_office,notnull,default:false"`
	SortOrder             int             `bun:"sort_order,notnull,default:0"`
	RolloverSourceChildID *int64          `bun:"rollover_source_child_id"`
	ReviewReason          *string         `bun:"review_reason"`
	// The planned class switch of an approved re-enrollment (#3917). It is
	// written and read only through the class-switch methods, never through
	// the RequestChild record.
	ClassSwitchFrom *string          `bun:"class_switch_from"`
	ClassSwitchTo   *string          `bun:"class_switch_to"`
	ClassSwitchOn   *enrollment.Date `bun:"class_switch_on,type:date"`
	// The day an approved future re-enrollment's weekly plan replaces the
	// running one (#6043). Written and read only through the weekly-plan
	// methods, never through the RequestChild record.
	WeeklyPlanOn       *calendar.Date `bun:"weekly_plan_on,type:date"`
	WeeklyPlanReplaces bool           `bun:"weekly_plan_replaces,notnull,default:false"`
	WeeklyPlanAuthor   *int64         `bun:"weekly_plan_author"`
}

// value decodes the row into the owner's request child. Its answers decode
// exactly here (ADR 0173); a decode error names the child.
func (r requestChildRow) value() (*enrollment.RequestChild, error) {
	value := &enrollment.RequestChild{ID: r.ID, TenantID: r.TenantID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, RequestID: r.RequestID, FirstName: r.FirstName, LastName: r.LastName, DateOfBirth: r.DateOfBirth, TargetGradeLevel: r.TargetGradeLevel, TargetSchoolClass: r.TargetSchoolClass, HolidayCareWish: r.HolidayCareWish, Status: r.Status, StatusReason: r.StatusReason, ActivationMode: r.ActivationMode, ActivateOn: r.ActivateOn, ReviewedAt: r.ReviewedAt, ReviewedBy: r.ReviewedBy, CreatedStudentID: r.CreatedStudentID, MatchedStudentID: r.MatchedStudentID, MatchedByOffice: r.MatchedByOffice, SortOrder: r.SortOrder, RolloverSourceChildID: r.RolloverSourceChildID, ReviewReason: r.ReviewReason, WeeklyPlanOn: r.WeeklyPlanOn}
	if len(r.CustomData) > 0 {
		if err := json.Unmarshal(r.CustomData, &value.CustomData); err != nil {
			return nil, fmt.Errorf("decode request child %d custom_data: %w", r.ID, err)
		}
	}
	return value, nil
}

// childStorage encodes the owner's request child into its row. Empty
// answers store {}, never null.
func childStorage(r *enrollment.RequestChild) (*requestChildRow, error) {
	customData, err := json.Marshal(r.CustomData)
	if err != nil {
		return nil, fmt.Errorf("encode request child custom_data: %w", err)
	}
	return &requestChildRow{ID: r.ID, TenantID: r.TenantID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, RequestID: r.RequestID, FirstName: r.FirstName, LastName: r.LastName, DateOfBirth: r.DateOfBirth, TargetGradeLevel: r.TargetGradeLevel, TargetSchoolClass: r.TargetSchoolClass, HolidayCareWish: r.HolidayCareWish, CustomData: customData, Status: r.Status, StatusReason: r.StatusReason, ActivationMode: r.ActivationMode, ActivateOn: r.ActivateOn, ReviewedAt: r.ReviewedAt, ReviewedBy: r.ReviewedBy, CreatedStudentID: r.CreatedStudentID, MatchedStudentID: r.MatchedStudentID, MatchedByOffice: r.MatchedByOffice, SortOrder: r.SortOrder, RolloverSourceChildID: r.RolloverSourceChildID, ReviewReason: r.ReviewReason}, nil
}

func childValues(rows []requestChildRow) ([]*enrollment.RequestChild, error) {
	var result []*enrollment.RequestChild
	for _, row := range rows {
		value, err := row.value()
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, nil
}
