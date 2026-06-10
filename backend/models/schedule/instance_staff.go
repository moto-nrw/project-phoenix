package schedule

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// tableInstanceStaff is the schema-qualified table name.
const tableInstanceStaff = "schedule.instance_staff"

// InstanceStaff assigns a staff member to a materialized activity instance.
// A row's optional RoomID (E3 multi-room override) is nil when the staff
// member stays in the instance's primary room; a non-nil value splits the
// instance across multiple rooms (the Lernzeit split pattern).
type InstanceStaff struct {
	base.Model `bun:"schema:schedule,table:instance_staff"`
	base.TenantModel

	InstanceID   int64  `bun:"instance_id,notnull" json:"instance_id"`
	StaffID      int64  `bun:"staff_id,notnull" json:"staff_id"`
	RoomID       *int64 `bun:"room_id" json:"room_id,omitempty"`
	IsPrimary    bool   `bun:"is_primary,notnull,default:false" json:"is_primary"`
	IsSubstitute bool   `bun:"is_substitute,notnull,default:false" json:"is_substitute"`
	IsAbsent     bool   `bun:"is_absent,notnull,default:false" json:"is_absent"`
}

func (s *InstanceStaff) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(`schedule.instance_staff AS "instance_staff"`)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(`schedule.instance_staff AS "instance_staff"`)
	}
	return nil
}

// TableName returns the database table name.
func (s *InstanceStaff) TableName() string { return tableInstanceStaff }

// GetID implements the Entity interface.
func (s *InstanceStaff) GetID() any { return s.ID }

// GetCreatedAt implements the Entity interface.
func (s *InstanceStaff) GetCreatedAt() time.Time { return s.CreatedAt }

// GetUpdatedAt implements the Entity interface.
func (s *InstanceStaff) GetUpdatedAt() time.Time { return s.UpdatedAt }

// Validate ensures the staff assignment is well-formed.
func (s *InstanceStaff) Validate() error {
	if s.InstanceID <= 0 {
		return errors.New("instance_id is required")
	}
	if s.StaffID <= 0 {
		return errors.New("staff_id is required")
	}
	if s.RoomID != nil && *s.RoomID <= 0 {
		return errors.New("room_id must be positive when set")
	}
	return nil
}

// InstanceStaffRepository defines operations for managing staff assignments to
// materialized activity instances.
type InstanceStaffRepository interface {
	base.Repository[*InstanceStaff]

	// FindByInstanceID returns all staff assignments for an instance.
	FindByInstanceID(ctx context.Context, instanceID int64) ([]*InstanceStaff, error)

	// FindByStaffAndDate returns all staff assignments for the given staff
	// member across all instances on a given date. Used by the one-click
	// substitute flow (E6) and gap detection.
	FindByStaffAndDate(ctx context.Context, staffID int64, date timezone.Date) ([]*InstanceStaff, error)

	// CountNonAbsentByInstanceIDs returns, for each instance id, the number of
	// instance_staff rows with is_absent=false. Single GROUP BY query; callers
	// must treat absent instance ids in the returned map as zero. Empty input
	// returns an empty map without hitting the DB.
	CountNonAbsentByInstanceIDs(ctx context.Context, instanceIDs []int64) (map[int64]int, error)

	// DeleteByInstanceID removes all staff assignments for an instance. The
	// CASCADE on the FK also does this on instance deletion; this method exists
	// for the "re-plan week" flow where the instance is kept but repopulated.
	DeleteByInstanceID(ctx context.Context, instanceID int64) error

	// DeleteFutureByStaffID removes the staff member's assignments on instances
	// dated strictly after the given date (staff offboarding cleanup). Past and
	// same-day assignments stay as history.
	DeleteFutureByStaffID(ctx context.Context, staffID int64, after timezone.Date) (int64, error)
}
