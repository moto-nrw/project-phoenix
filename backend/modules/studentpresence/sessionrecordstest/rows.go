// Package sessionrecordstest maps the two Student Presence session tables,
// active.groups and active.group_supervisors, for test fixtures and for tests
// that arrange or assert stored rows directly. Student Presence's internal
// Postgres adapter owns the runtime persistence; production code exchanges the
// values of the public modules/studentpresence contract (#3422).
package sessionrecordstest

import (
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/uptrace/bun"
)

// ActiveGroupRow is one stored row of active.groups.
type ActiveGroupRow struct {
	bun.BaseModel  `bun:"table:active.groups,alias:group"`
	ID             int64      `bun:"id,pk,autoincrement" json:"id"`
	CreatedAt      time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt      time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
	TenantID       int64      `bun:"tenant_id,notnull" json:"tenant_id"`
	StartTime      time.Time  `bun:"start_time,notnull" json:"start_time"`
	EndTime        *time.Time `bun:"end_time" json:"end_time,omitempty"`
	LastActivity   time.Time  `bun:"last_activity,notnull" json:"last_activity"`
	TimeoutMinutes int        `bun:"timeout_minutes,nullzero" json:"timeout_minutes"`
	GroupID        *int64     `bun:"group_id" json:"group_id"`
	DeviceID       *int64     `bun:"device_id" json:"device_id,omitempty"`
	RoomID         int64      `bun:"room_id,notnull" json:"room_id"`
}

// GroupSupervisorRow is one stored row of active.group_supervisors.
type GroupSupervisorRow struct {
	bun.BaseModel `bun:"table:active.group_supervisors,alias:group_supervisor"`
	ID            int64          `bun:"id,pk,autoincrement" json:"id"`
	CreatedAt     time.Time      `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt     time.Time      `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
	TenantID      int64          `bun:"tenant_id,notnull" json:"tenant_id"`
	StaffID       int64          `bun:"staff_id,notnull" json:"staff_id"`
	GroupID       int64          `bun:"group_id,notnull" json:"group_id"`
	Role          string         `bun:"role,notnull" json:"role"`
	StartDate     timezone.Date  `bun:"start_date,notnull,type:date" json:"start_date"`
	EndDate       *timezone.Date `bun:"end_date,type:date" json:"end_date,omitempty"`
}
