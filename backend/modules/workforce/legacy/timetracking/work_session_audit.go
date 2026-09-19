package timetracking

import (
	"context"
	"time"
)

// WorkSessionAudit is the edit evidence consumed by work-session operations.
type WorkSessionAudit interface {
	CreateBatch(context.Context, []*WorkSessionEdit) error
	GetBySessionID(context.Context, int64) ([]*WorkSessionEdit, error)
	CountBySessionIDs(context.Context, []int64) (map[int64]int, error)
	CountManualBySessionIDs(context.Context, []int64) (map[int64]int, error)
}

// WorkSessionEdit is the change evidence shown in a session's edit history.
type WorkSessionEdit struct {
	ID        int64     `json:"id"`
	TenantID  int64     `json:"tenant_id"`
	SessionID int64     `json:"session_id"`
	StaffID   int64     `json:"staff_id"`
	EditedBy  int64     `json:"edited_by"`
	FieldName string    `json:"field_name"`
	OldValue  *string   `json:"old_value"`
	NewValue  *string   `json:"new_value"`
	Notes     *string   `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
}

func (e *WorkSessionEdit) SetTenantID(id int64) { e.TenantID = id }
