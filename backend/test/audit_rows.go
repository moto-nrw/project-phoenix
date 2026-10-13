package test

import (
	"encoding/json"
	"time"

	"github.com/uptrace/bun"
)

// DataDeletion is one audit.data_deletions row the fixtures write. The Audit
// Platform keeps its row mapping in models/audit, which the shared fixtures
// may not import (#2748), so test support maps the table itself.
type DataDeletion struct {
	bun.BaseModel  `bun:"table:audit.data_deletions,alias:data_deletion"`
	ID             int64          `bun:"id,pk,autoincrement"`
	TenantID       int64          `bun:"tenant_id,notnull"`
	StudentID      *int64         `bun:"student_id"`
	StaffID        *int64         `bun:"staff_id"`
	DeletionType   string         `bun:"deletion_type,notnull"`
	RecordsDeleted int            `bun:"records_deleted,notnull"`
	DeletionReason string         `bun:"deletion_reason"`
	DeletedBy      string         `bun:"deleted_by,notnull"`
	DeletedAt      time.Time      `bun:"deleted_at,notnull,default:now()"`
	Metadata       map[string]any `bun:"metadata,type:jsonb"`
}

// EnrollmentOfferingAdjustment is one audit.enrollment_offering_adjustments
// row the fixtures write.
type EnrollmentOfferingAdjustment struct {
	bun.BaseModel               `bun:"table:audit.enrollment_offering_adjustments,alias:enrollment_offering_adjustment"`
	ID                          int64           `bun:"id,pk,autoincrement"`
	TenantID                    int64           `bun:"tenant_id,notnull"`
	RequestID                   int64           `bun:"request_id,notnull"`
	RequestChildID              int64           `bun:"request_child_id,notnull"`
	StudentID                   int64           `bun:"student_id,notnull"`
	ActorAccountID              int64           `bun:"actor_account_id,notnull"`
	ActorRole                   string          `bun:"actor_role,notnull"`
	ActorNameSnapshot           *string         `bun:"actor_name_snapshot"`
	ActorEmailSnapshot          *string         `bun:"actor_email_snapshot"`
	Reason                      string          `bun:"reason,notnull"`
	Source                      string          `bun:"source,notnull"`
	Before                      json.RawMessage `bun:"before_json,type:jsonb,notnull"`
	After                       json.RawMessage `bun:"after_json,type:jsonb,notnull"`
	CompleteWithdrawalConfirmed bool            `bun:"complete_withdrawal_confirmed,notnull"`
	ChangedAt                   time.Time       `bun:"changed_at,notnull,default:now()"`
}

// AuditStammdatenSectionSchoolClasses is the stored audit.staff_master_data_changes
// section of a staff member's class assignments (#1772).
const AuditStammdatenSectionSchoolClasses = "school-classes"
