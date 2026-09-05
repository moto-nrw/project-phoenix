package schedule

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
)

// InstanceStaff assigns a staff member to a materialized activity instance.
// A row's optional RoomID (E3 multi-room override) is nil when the staff
// member stays in the instance's primary room; a non-nil value splits the
// instance across multiple rooms (the Lernzeit split pattern).
type InstanceStaff struct {
	Model `bun:"schema:schedule,table:instance_staff"`
	TenantModel

	InstanceID   int64  `bun:"instance_id,notnull" json:"instance_id"`
	StaffID      int64  `bun:"staff_id,notnull" json:"staff_id"`
	RoomID       *int64 `bun:"room_id" json:"room_id,omitempty"`
	IsPrimary    bool   `bun:"is_primary,notnull,default:false" json:"is_primary"`
	IsSubstitute bool   `bun:"is_substitute,notnull,default:false" json:"is_substitute"`
	IsAbsent     bool   `bun:"is_absent,notnull,default:false" json:"is_absent"`
	// AbsenceReason is an optional short "why" captured when the staff member
	// is marked absent (Vertretungsplan, issue #1840).
	AbsenceReason *string `bun:"absence_reason" json:"absence_reason,omitempty"`
	// SickAbsenceID identifies the active.staff_absences row whose #1843 sick
	// cascade marked this row absent. NULL for manual Vertretungsplan absences;
	// deleting the sick report clears only rows carrying its id.
	SickAbsenceID *int64 `bun:"sick_absence_id" json:"sick_absence_id,omitempty"`
}

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

	// UpdateColumns surfaces the embedded generic repository's single-column
	// update: the #1843 sick cascade stamps/releases sick_absence_id without
	// whole-model writes.
	UpdateColumns(ctx context.Context, row *InstanceStaff, columns ...string) (int64, error)

	// FindByInstanceID returns all staff assignments for an instance.
	FindByInstanceID(ctx context.Context, instanceID int64) ([]*InstanceStaff, error)

	// FindByInstanceIDs returns every instance_staff row for any of the
	// given instance IDs in a single IN query, tenant-scoped. Returns an
	// empty slice (not nil) when instanceIDs is empty, matching the sibling
	// bulk helpers (FindExpectedByInstanceIDs, CountNonAbsentByInstanceIDs).
	// Used by the planned-conflict probe to intersect requested staff with
	// staff already assigned to overlapping instances without N+1.
	FindByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*InstanceStaff, error)

	// FindByStaffAndDate returns all staff assignments for the given staff
	// member across all instances on a given date. Used by the one-click
	// substitute flow (E6) and gap detection.
	FindByStaffAndDate(ctx context.Context, staffID int64, date timezone.Date) ([]*InstanceStaff, error)

	// FindByStaffIDsAndDate batches the joined date lookup for several staff
	// members. Empty input returns an empty slice without querying.
	FindByStaffIDsAndDate(ctx context.Context, staffIDs []int64, date timezone.Date) ([]*InstanceStaff, error)

	// FindByStaffAndDateRange returns the staff member's assignments across
	// all instances dated within [from, to] inclusive, ordered by instance
	// date then start time. Used by the self-service assignment read (#1844)
	// so its cost stays proportional to one staff member's plan, never the
	// whole tenant window.
	FindByStaffAndDateRange(ctx context.Context, staffID int64, from, to timezone.Date) ([]*InstanceStaff, error)

	// CountNonAbsentByInstanceIDs returns, for each instance id, the number of
	// instance_staff rows with is_absent=false. Single GROUP BY query; callers
	// must treat absent instance ids in the returned map as zero. Empty input
	// returns an empty map without hitting the DB.
	CountNonAbsentByInstanceIDs(ctx context.Context, instanceIDs []int64) (map[int64]int, error)

	// DeleteByInstanceID removes all staff assignments for an instance. The
	// CASCADE on the FK also does this on instance deletion; this method exists
	// for the "re-plan week" flow where the instance is kept but repopulated.
	DeleteByInstanceID(ctx context.Context, instanceID int64) error

	// DeleteUpcomingByStaffID removes the staff member's assignments on
	// instances dated strictly after the given date, plus same-day instances
	// still in 'planned' status (staff offboarding cleanup). Past and same-day
	// already-started assignments stay as history.
	DeleteUpcomingByStaffID(ctx context.Context, staffID int64, after timezone.Date) (int64, error)
}
