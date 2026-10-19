package schedulerecords

import (
	"errors"
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

// ports.InstanceStaffRecords defines operations for managing staff assignments to
// materialized activity instances.
