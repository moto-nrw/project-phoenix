package postgres

import (
	"encoding/json"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/uptrace/bun"
)

type requestChildRow struct {
	bun.BaseModel         `bun:"table:enrollment.request_children,alias:request_child"`
	ID                    int64            `bun:"id,pk,autoincrement"`
	TenantID              int64            `bun:"tenant_id,notnull"`
	CreatedAt             time.Time        `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt             time.Time        `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	RequestID             int64            `bun:"request_id,notnull"`
	FirstName             string           `bun:"first_name,notnull"`
	LastName              string           `bun:"last_name,notnull"`
	DateOfBirth           enrollment.Date  `bun:"date_of_birth,notnull,type:date"`
	TargetGradeLevel      *int16           `bun:"target_grade_level"`
	TargetSchoolClass     *string          `bun:"target_school_class"`
	CustomData            json.RawMessage  `bun:"custom_data,type:jsonb,notnull,default:'{}'"`
	Status                string           `bun:"status,notnull,default:'submitted'"`
	StatusReason          *string          `bun:"status_reason"`
	ActivationMode        string           `bun:"activation_mode,notnull,default:'scheduled'"`
	ActivateOn            *enrollment.Date `bun:"activate_on,type:date"`
	ReviewedAt            *time.Time       `bun:"reviewed_at"`
	ReviewedBy            *int64           `bun:"reviewed_by"`
	CreatedStudentID      *int64           `bun:"created_student_id"`
	MatchedStudentID      *int64           `bun:"matched_student_id"`
	SortOrder             int              `bun:"sort_order,notnull,default:0"`
	RolloverSourceChildID *int64           `bun:"rollover_source_child_id"`
	ReviewReason          *string          `bun:"review_reason"`
}

func (r requestChildRow) value() *enrollment.RequestChild {
	return &enrollment.RequestChild{ID: r.ID, TenantID: r.TenantID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, RequestID: r.RequestID, FirstName: r.FirstName, LastName: r.LastName, DateOfBirth: r.DateOfBirth, TargetGradeLevel: r.TargetGradeLevel, TargetSchoolClass: r.TargetSchoolClass, CustomData: r.CustomData, Status: r.Status, StatusReason: r.StatusReason, ActivationMode: r.ActivationMode, ActivateOn: r.ActivateOn, ReviewedAt: r.ReviewedAt, ReviewedBy: r.ReviewedBy, CreatedStudentID: r.CreatedStudentID, MatchedStudentID: r.MatchedStudentID, SortOrder: r.SortOrder, RolloverSourceChildID: r.RolloverSourceChildID, ReviewReason: r.ReviewReason}
}
func childStorage(r *enrollment.RequestChild) *requestChildRow {
	return &requestChildRow{ID: r.ID, TenantID: r.TenantID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, RequestID: r.RequestID, FirstName: r.FirstName, LastName: r.LastName, DateOfBirth: r.DateOfBirth, TargetGradeLevel: r.TargetGradeLevel, TargetSchoolClass: r.TargetSchoolClass, CustomData: r.CustomData, Status: r.Status, StatusReason: r.StatusReason, ActivationMode: r.ActivationMode, ActivateOn: r.ActivateOn, ReviewedAt: r.ReviewedAt, ReviewedBy: r.ReviewedBy, CreatedStudentID: r.CreatedStudentID, MatchedStudentID: r.MatchedStudentID, SortOrder: r.SortOrder, RolloverSourceChildID: r.RolloverSourceChildID, ReviewReason: r.ReviewReason}
}
