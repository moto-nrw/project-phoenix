package schoolmembership

import (
	"context"
	"slices"
)

// StaffAssignmentLockQuery is the membership query needed by assignment guards.
type StaffAssignmentLockQuery interface {
	ListStaff(context.Context, StaffFilter) ([]Staff, error)
}

// LockStaffAssignments locks and validates all requested live memberships in
// one ascending query in the caller's transaction, without loading employment.
func LockStaffAssignments(ctx context.Context, membership StaffAssignmentLockQuery, staffIDs []int64) error {
	for _, id := range staffIDs {
		if id <= 0 {
			return &InvalidMembershipError{Reason: "staff ID is required"}
		}
	}
	ids := slices.Compact(slices.Sorted(slices.Values(staffIDs)))
	if len(ids) == 0 {
		return nil
	}
	rows, err := membership.ListStaff(ctx, StaffFilter{IDs: ids, ForMutation: true, MembershipOnly: true})
	if err != nil {
		return err
	}
	if len(rows) != len(ids) {
		return ErrStaffNotFound
	}
	return nil
}
