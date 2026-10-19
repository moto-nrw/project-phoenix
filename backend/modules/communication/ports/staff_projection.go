package ports

import (
	"context"
)

// StaffMember is one live staff membership: the school and the member's
// person.
type StaffMember struct {
	TenantID int64
	PersonID int64
}

// StaffMembers lists the live staff of the given schools; nil lists the staff
// of every school visible in the caller's transaction. School Membership owns
// those rows; the composition root binds it.
type StaffMembers func(ctx context.Context, tenantIDs []int64) ([]StaffMember, error)

// StaffPersons resolves the persons of staff members by ID; an unknown ID is
// absent. People Directory owns those rows; the composition root binds it.
type StaffPersonAccount struct{ AccountID *int64 }
type StaffPersons func(ctx context.Context, ids []int64) (map[int64]*StaffPersonAccount, error)

// GroupNames resolves group display names by ID; a group the caller cannot
// see is absent. School Structure owns the groups; the composition root binds
// it.
type GroupNames func(ctx context.Context, ids []int64) (map[int64]string, error)
