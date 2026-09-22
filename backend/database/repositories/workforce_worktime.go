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

// NewWorkforceWithClock is NewWorkforce with a pinned clock for the live
// work-session window and the calendar day; nil means the wall clock.
func NewWorkforceWithClock(db *bun.DB, membership schoolmembership.Capability, now func() time.Time) (workforce.Capability, error) {
	return workforceCompose.New(workforceCompose.Dependencies{
		LockStaffAssignment: func(ctx context.Context, staffID int64) error {
			_, err := membership.FindStaffForMutation(ctx, staffID)
			return err
		},
		DB:           db,
		LiveStaffIDs: WorkforceLiveStaffIDs(membership),
		Observe:      func(workforceCompose.Observation) {},
		Now:          now,
	})
}
