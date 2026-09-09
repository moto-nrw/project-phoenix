package repositories

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/uptrace/bun"
)

// NewWorkforce composes the work-time owner behind the legacy composition seam
// for graphs that do not record observations. Production roots compose the
// module themselves (api/base.go) so runtime evidence is kept.
func NewWorkforce(db *bun.DB, membership schoolmembership.Capability) (workforce.Capability, error) {
	return NewWorkforceWithClock(db, membership, nil)
}

// NewStaffDocumentCleanup is the narrow Workforce cleanup composition used by
// adapter tests and legacy roots; it does not construct a repository factory.
func NewStaffDocumentCleanup(db *bun.DB, now func() time.Time) (*workforce.DocumentCleanup, error) {
	return workforceCompose.NewDocumentCleanup(db, now)
}

// NewWorkforceWithClock is NewWorkforce with a pinned clock for the live
// work-session window and the calendar day; nil means the wall clock.
func NewWorkforceWithClock(db *bun.DB, membership schoolmembership.Capability, now func() time.Time) (workforce.Capability, error) {
	return workforceCompose.New(workforceCompose.Dependencies{
		LockStaffAssignment: func(ctx context.Context, staffID int64) error {
			_, err := membership.FindStaffForMutation(ctx, staffID)
			return err
		},
		DB:                db,
		AssignedStaffIDs:  WorkforceAssignedStaffIDs(membership),
		RebaseStaffAnchor: membership.RebaseWorkTimeModelAnchor,
		Observe:           func(workforceCompose.Observation) {},
		Now:               now,
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
