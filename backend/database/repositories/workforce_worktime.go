package repositories

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/uptrace/bun"
)

// NewWorkforce composes the work-time owner behind the legacy composition seam
// for graphs that do not record observations. Production roots compose the
// module themselves (api/base.go) so runtime evidence is kept.
func NewWorkforce(db *bun.DB, membership schoolmembership.Capability) (workforce.Capability, error) {
	return workforceCompose.New(workforceCompose.Dependencies{
		DB:                db,
		AssignedStaffIDs:  WorkforceAssignedStaffIDs(membership),
		RebaseStaffAnchor: membership.RebaseWorkTimeModelAnchor,
		Observe:           func(workforceCompose.Observation) {},
	})
}

// WorkforceAssignedStaffIDs resolves the staff members bound to a work-time
// template through School Membership. Workforce never joins users.staff
// itself (#2667).
func WorkforceAssignedStaffIDs(membership schoolmembership.Capability) func(context.Context, int64) ([]int64, error) {
	return func(ctx context.Context, workTimeModelID int64) ([]int64, error) {
		members, err := membership.ListStaff(ctx, schoolmembership.StaffFilter{WorkTimeModelID: &workTimeModelID})
		if err != nil {
			return nil, err
		}
		ids := make([]int64, 0, len(members))
		for _, member := range members {
			ids = append(ids, member.ID)
		}
		return ids, nil
	}
}
