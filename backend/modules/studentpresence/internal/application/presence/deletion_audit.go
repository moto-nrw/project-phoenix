package presence

import (
	"context"
	"time"
)

// DeletionAudit writes retention evidence in the deletion transaction: the
// events of many students at once, all or nothing.
type DeletionAudit interface {
	CreateAll(context.Context, []*DeletionEvent) error
}

// DeletionEvent carries the subject and evidence of a retention operation.
type DeletionEvent struct {
	TenantID       int64
	StudentID      *int64
	StaffID        *int64
	DeletionType   string
	RecordsDeleted int
	DeletionReason string
	DeletedBy      string
	DeletedAt      time.Time
	Metadata       map[string]interface{}
}
