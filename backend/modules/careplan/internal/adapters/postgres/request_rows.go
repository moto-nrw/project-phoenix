package postgres

import (
	"encoding/json"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/uptrace/bun"
)

type excusedAbsenceRequestRow struct {
	bun.BaseModel  `bun:"table:excused_absence_requests,alias:excused_absence_request"`
	ID             int64          `bun:"id,pk,autoincrement"`
	TenantID       int64          `bun:"tenant_id,notnull"`
	CreatedAt      time.Time      `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt      time.Time      `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	StudentID      int64          `bun:"student_id,notnull"`
	SubmittedBy    int64          `bun:"submitted_by,notnull"`
	Dates          []calendarDate `bun:"dates,type:jsonb,notnull"`
	Note           string         `bun:"note,notnull"`
	AbsenceStatus  string         `bun:"absence_status,notnull,default:'excused'"`
	Status         string         `bun:"status,notnull,default:'pending'"`
	DecisionReason *string        `bun:"decision_reason"`
	ReviewedBy     *int64         `bun:"reviewed_by"`
	ReviewedAt     *time.Time     `bun:"reviewed_at"`
	AppliedAt      *time.Time     `bun:"applied_at"`
}

type careScheduleRequestRow struct {
	bun.BaseModel    `bun:"table:care_schedule_change_requests,alias:care_schedule_change_request"`
	ID               int64           `bun:"id,pk,autoincrement"`
	TenantID         int64           `bun:"tenant_id,notnull"`
	CreatedAt        time.Time       `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt        time.Time       `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	StudentID        int64           `bun:"student_id,notnull"`
	SubmittedBy      int64           `bun:"submitted_by,notnull"`
	RequestKind      string          `bun:"request_kind,notnull"`
	Payload          json.RawMessage `bun:"payload,type:jsonb,notnull"`
	Status           string          `bun:"status,notnull,default:'pending'"`
	DecisionReason   *string         `bun:"decision_reason"`
	ReviewedBy       *int64          `bun:"reviewed_by"`
	ReviewedAt       *time.Time      `bun:"reviewed_at"`
	AppliedAt        *time.Time      `bun:"applied_at"`
	DecisionSnapshot json.RawMessage `bun:"decision_snapshot,type:jsonb"`
}

type studentDataRequestRow struct {
	bun.BaseModel `bun:"table:student_data_change_requests,alias:student_data_change_request"`
	ID            int64           `bun:"id,pk,autoincrement"`
	TenantID      int64           `bun:"tenant_id,notnull"`
	CreatedAt     time.Time       `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time       `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	StudentID     int64           `bun:"student_id,notnull"`
	SubmittedBy   int64           `bun:"submitted_by,notnull"`
	Target        string          `bun:"target,notnull"`
	TargetRefID   *int64          `bun:"target_ref_id"`
	FieldKey      string          `bun:"field_key,notnull"`
	OldValue      json.RawMessage `bun:"old_value,type:jsonb"`
	NewValue      json.RawMessage `bun:"new_value,type:jsonb,notnull"`
	Status        string          `bun:"status,notnull,default:'pending'"`
	ReviewReason  *string         `bun:"review_reason"`
	ReviewedBy    *int64          `bun:"reviewed_by"`
	ReviewedAt    *time.Time      `bun:"reviewed_at"`
	AppliedAt     *time.Time      `bun:"applied_at"`
}

func excusedRequestFromPublic(value careplan.ExcusedAbsenceRequest) *excusedAbsenceRequestRow {
	dates := make([]calendarDate, len(value.Dates))
	for i := range value.Dates {
		dates[i] = calendarDate(value.Dates[i])
	}
	row := &excusedAbsenceRequestRow{StudentID: value.StudentID, SubmittedBy: value.SubmittedBy, Dates: dates, Note: value.Note, AbsenceStatus: value.AbsenceStatus, Status: value.Status, DecisionReason: value.DecisionReason, ReviewedBy: value.ReviewedBy, ReviewedAt: value.ReviewedAt, AppliedAt: value.AppliedAt}
	row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	return row
}

func excusedRequestToPublic(row *excusedAbsenceRequestRow) careplan.ExcusedAbsenceRequest {
	dates := make([]careplan.Date, len(row.Dates))
	for i := range row.Dates {
		dates[i] = careplan.Date(row.Dates[i])
	}
	return careplan.ExcusedAbsenceRequest{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, StudentID: row.StudentID, SubmittedBy: row.SubmittedBy, Dates: dates, Note: row.Note, AbsenceStatus: row.AbsenceStatus, Status: row.Status, DecisionReason: row.DecisionReason, ReviewedBy: row.ReviewedBy, ReviewedAt: row.ReviewedAt, AppliedAt: row.AppliedAt}
}

func excusedRequestsToPublic(rows []excusedAbsenceRequestRow) []careplan.ExcusedAbsenceRequest {
	result := make([]careplan.ExcusedAbsenceRequest, 0, len(rows))
	for i := range rows {
		result = append(result, excusedRequestToPublic(&rows[i]))
	}
	return result
}

func careScheduleRequestFromPublic(value careplan.CareScheduleChangeRequest) *careScheduleRequestRow {
	row := &careScheduleRequestRow{StudentID: value.StudentID, SubmittedBy: value.SubmittedBy, RequestKind: value.RequestKind, Payload: value.Payload, Status: value.Status, DecisionReason: value.DecisionReason, ReviewedBy: value.ReviewedBy, ReviewedAt: value.ReviewedAt, AppliedAt: value.AppliedAt, DecisionSnapshot: value.DecisionSnapshot}
	row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	return row
}

func careScheduleRequestToPublic(row *careScheduleRequestRow) careplan.CareScheduleChangeRequest {
	return careplan.CareScheduleChangeRequest{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, StudentID: row.StudentID, SubmittedBy: row.SubmittedBy, RequestKind: row.RequestKind, Payload: row.Payload, Status: row.Status, DecisionReason: row.DecisionReason, ReviewedBy: row.ReviewedBy, ReviewedAt: row.ReviewedAt, AppliedAt: row.AppliedAt, DecisionSnapshot: row.DecisionSnapshot}
}

func careScheduleRequestsToPublic(rows []careScheduleRequestRow) []careplan.CareScheduleChangeRequest {
	result := make([]careplan.CareScheduleChangeRequest, 0, len(rows))
	for i := range rows {
		result = append(result, careScheduleRequestToPublic(&rows[i]))
	}
	return result
}

func studentDataRequestFromPublic(value careplan.StudentDataChangeRequest) *studentDataRequestRow {
	row := &studentDataRequestRow{StudentID: value.StudentID, SubmittedBy: value.SubmittedBy, Target: value.Target, TargetRefID: value.TargetRefID, FieldKey: value.FieldKey, OldValue: value.OldValue, NewValue: value.NewValue, Status: value.Status, ReviewReason: value.ReviewReason, ReviewedBy: value.ReviewedBy, ReviewedAt: value.ReviewedAt, AppliedAt: value.AppliedAt}
	row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	return row
}

func studentDataRequestToPublic(row *studentDataRequestRow) careplan.StudentDataChangeRequest {
	return careplan.StudentDataChangeRequest{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, StudentID: row.StudentID, SubmittedBy: row.SubmittedBy, Target: row.Target, TargetRefID: row.TargetRefID, FieldKey: row.FieldKey, OldValue: row.OldValue, NewValue: row.NewValue, Status: row.Status, ReviewReason: row.ReviewReason, ReviewedBy: row.ReviewedBy, ReviewedAt: row.ReviewedAt, AppliedAt: row.AppliedAt}
}

func studentDataRequestsToPublic(rows []studentDataRequestRow) []careplan.StudentDataChangeRequest {
	result := make([]careplan.StudentDataChangeRequest, 0, len(rows))
	for i := range rows {
		result = append(result, studentDataRequestToPublic(&rows[i]))
	}
	return result
}
