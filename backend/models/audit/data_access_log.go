package audit

import (
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// ResourceType constants for DataAccessLog.ResourceType.
const (
	ResourceTypeAttendanceHistory = "attendance_history"
)

// DataAccessLog is an append-only record of a staff member viewing sensitive
// tenant data (currently: per-student attendance history). Written for GDPR
// auditability. No retention/cleanup policy exists yet — this matches the
// existing convention for other tables in the audit schema.
type DataAccessLog struct {
	ID int64 `bun:"id,pk,autoincrement" json:"id"`
	base.TenantModel
	ActorAccountID int64     `bun:"actor_account_id,notnull" json:"actor_account_id"`
	ActorRole      string    `bun:"actor_role,notnull" json:"actor_role"`
	ResourceType   string    `bun:"resource_type,notnull" json:"resource_type"`
	StudentID      *int64    `bun:"student_id" json:"student_id,omitempty"`
	RangeStart     time.Time `bun:"range_start,notnull" json:"range_start"`
	RangeEnd       time.Time `bun:"range_end,notnull" json:"range_end"`
	AccessedAt     time.Time `bun:"accessed_at,notnull,default:now()" json:"accessed_at"`
}

// TableName returns the database table name.
func (d *DataAccessLog) TableName() string {
	return "audit.data_access_log"
}

// GetID implements the base.Entity interface.
func (d *DataAccessLog) GetID() interface{} {
	return d.ID
}

// GetCreatedAt implements the base.Entity interface.
func (d *DataAccessLog) GetCreatedAt() time.Time {
	return d.AccessedAt
}

// GetUpdatedAt implements the base.Entity interface. Access log rows are
// append-only, so updated_at mirrors accessed_at.
func (d *DataAccessLog) GetUpdatedAt() time.Time {
	return d.AccessedAt
}
