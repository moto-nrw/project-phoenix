package owners

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/schoolmembership"
)

// WorkforceLiveStaffIDs answers Workforce's one question to School
// Membership: which of the memberships bound to a work-time template are still
// live. Workforce never reads users.staff_school_memberships itself.
func WorkforceLiveStaffIDs(membership schoolmembership.Query) func(context.Context, []int64) ([]int64, error) {
	return func(ctx context.Context, ids []int64) ([]int64, error) {
		members, err := membership.ListStaff(ctx, schoolmembership.StaffFilter{IDs: ids, MembershipOnly: true})
		if err != nil {
			return nil, err
		}
		live := make([]int64, 0, len(members))
		for _, member := range members {
			live = append(live, member.ID)
		}
		return live, nil
	}
}

// SickReportStaffLock takes the staff rows a sick-report write locks after
// its gates and days (#6702). An offboarded member has no live row: nothing
// can offboard it again, so no cycle needs its row, and the write reports
// its own refusal.
func SickReportStaffLock(membership schoolmembership.Query) func(context.Context, []int64) error {
	return func(ctx context.Context, staffIDs []int64) error {
		_, err := membership.ListStaff(ctx, schoolmembership.StaffFilter{IDs: staffIDs, ForMutation: true, MembershipOnly: true})
		return err
	}
}

// WorkforceStaffAssignmentLock answers Workforce's lock on a staff
// assignment: the School Membership row of the staff member is locked for
// the mutation the caller's unit of work is about to make.
func WorkforceStaffAssignmentLock(membership schoolmembership.Query) func(context.Context, int64) error {
	return func(ctx context.Context, staffID int64) error {
		_, err := membership.FindStaffForMutation(ctx, staffID)
		return err
	}
}
