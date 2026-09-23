package careplan

import (
	"context"
	"errors"
)

// Pickup permissions of a guardian (#2756). Whether a guardian may collect a
// child, and the note the school keeps about it, belong to Care Plan
// (users.student_guardian_pickup_permissions). The relationship they hang off
// belongs to People Directory; its unit of work creates both halves together
// and holds the relationship row lock while it writes this one.

// ErrGuardianPickupTenantMismatch refuses a command whose school differs from
// the school of the caller's tenant transaction.
var ErrGuardianPickupTenantMismatch = errors.New("care plan: pickup permission belongs to another school")

// GuardianPickupPermission is the pickup half of one relationship.
type GuardianPickupPermission struct {
	TenantID       int64
	RelationshipID int64
	CanPickup      bool
	PickupNotes    *string
}

// GuardianPickupPermissionChange writes only the supplied columns: a nil CanPickup is
// left alone, and PickupNotes is written only when SetPickupNotes is true
// (nil then clears it).
type GuardianPickupPermissionChange struct {
	TenantID       int64
	RelationshipID int64
	CanPickup      *bool
	SetPickupNotes bool
	PickupNotes    *string
}

// GuardianPickupPermissions is the command side of the pickup permission.
// Every command runs in the caller's transaction and names its school
// explicitly, so it also serves the administrative transactions that write
// across schools.
type GuardianPickupPermissions interface {
	// CreateGuardianPickupPermission records the permission of a new
	// relationship.
	CreateGuardianPickupPermission(context.Context, GuardianPickupPermission) error
	// ChangeGuardianPickupPermission writes the supplied columns and reports
	// whether the relationship has a pickup permission in that school.
	ChangeGuardianPickupPermission(context.Context, GuardianPickupPermissionChange) (bool, error)
}
