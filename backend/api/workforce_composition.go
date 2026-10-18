package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
)

// workforceLiveStaffIDs answers Workforce's one question to School
// Membership: which of the memberships bound to a work-time template are still
// live. Workforce never reads users.staff_school_memberships itself.
func workforceLiveStaffIDs(membership schoolmembership.Query) func(context.Context, []int64) ([]int64, error) {
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

// workforceStaffAssignmentLock answers Workforce's lock on a staff
// assignment: the School Membership row of the staff member is locked for
// the mutation the caller's unit of work is about to make.
func workforceStaffAssignmentLock(membership schoolmembership.Query) func(context.Context, int64) error {
	return func(ctx context.Context, staffID int64) error {
		_, err := membership.FindStaffForMutation(ctx, staffID)
		return err
	}
}

// NewUnobservedWorkforce composes the Workforce owner over an unobserved
// School Membership for the command roots, which record no runtime evidence.
func NewUnobservedWorkforce(db transactionruntime.Database) (workforceModule.Capability, error) {
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return nil, err
	}
	return workforceCompose.New(workforceCompose.Dependencies{
		LockStaffAssignment: workforceStaffAssignmentLock(membership),
		DB:                  db.Pool(),
		LiveStaffIDs:        workforceLiveStaffIDs(membership),
		Observe:             func(workforceCompose.Observation) {},
	})
}
