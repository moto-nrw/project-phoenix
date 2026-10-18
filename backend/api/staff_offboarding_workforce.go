package api

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforcecompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/workflows/staffoffboarding"
	offboardingcompose "github.com/moto-nrw/project-phoenix/workflows/staffoffboarding/compose"
)

// NewStaffOffboardingWorkflow binds the workflow's Workforce and Delivery ports.
func NewStaffOffboardingWorkflow(deps offboardingcompose.Dependencies) (*staffoffboarding.Workflow, error) {
	membership := deps.Membership
	work, err := workforcecompose.NewOffboarding(workforcecompose.Dependencies{
		LockStaffAssignment: func(ctx context.Context, staffID int64) error {
			_, err := membership.FindStaffForMutation(ctx, staffID)
			return err
		},
		DB: deps.DB, Observe: func(workforcecompose.Observation) {},
		LiveStaffIDs: func(ctx context.Context, membershipIDs []int64) ([]int64, error) {
			rows, err := membership.ListStaff(ctx, schoolmembership.StaffFilter{IDs: membershipIDs, MembershipOnly: true})
			if err != nil {
				return nil, err
			}
			ids := make([]int64, 0, len(rows))
			for _, row := range rows {
				ids = append(ids, row.ID)
			}
			return ids, nil
		},
	}, func(ctx context.Context, absence workforce.StaffAbsence, actorID int64) error {
		record, err := deps.AbsenceDeletionRecord(absence, actorID)
		if err != nil {
			return err
		}
		return deps.Audit.Append(ctx, record)
	})
	if err != nil {
		return nil, err
	}

	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	events := NewGroupEvents(deps.Broadcaster, logger)
	return offboardingcompose.New(deps, work, func(ctx context.Context) {
		events.QueueGroupAccessChanged(ctx, "staff_offboarding")
	})
}
