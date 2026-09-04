package repositories

import (
	"context"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	activitiesRepo "github.com/moto-nrw/project-phoenix/modules/timetable/compose/repositoryadapter"
)

// staffActivityGroupRepository attaches the staff member to the supervisors
// a group is loaded with. The replaced lookup was a soft-delete-aware model
// select, so an offboarded supervisor keeps a nil Staff here.
type staffActivityGroupRepository struct {
	activitiesModels.GroupRepository
	membership staffLookup
}

// BindStudentDirectory forwards the owner binding to the raw activity-group
// repository. The staff projection remains outside it so supervisor and
// student ownership can both be rebound at the service root.
func (r staffActivityGroupRepository) BindStudentDirectory(students activitiesRepo.StudentDirectory) {
	if repo, ok := r.GroupRepository.(interface {
		BindStudentDirectory(activitiesRepo.StudentDirectory)
	}); ok {
		repo.BindStudentDirectory(students)
	}
}

func (r staffActivityGroupRepository) FindWithSupervisors(ctx context.Context, groupID int64) (*activitiesModels.Group, []*activitiesModels.SupervisorPlanned, error) {
	group, supervisors, err := r.GroupRepository.FindWithSupervisors(ctx, groupID)
	if err != nil {
		return nil, nil, err
	}
	if err := attachPlannedSupervisorStaff(ctx, r.membership, supervisors, false); err != nil {
		return nil, nil, err
	}
	return group, supervisors, nil
}

// staffActivityGroupTargetRepository keeps the optional target seam the
// timetable services type-assert on reachable through the staff wrapper.
type staffActivityGroupTargetRepository struct {
	staffActivityGroupRepository
	activitiesModels.GroupTargetRepository
}

var _ activityGroupTargets = staffActivityGroupTargetRepository{}

func newStaffActivityGroupRepository(inner activitiesModels.GroupRepository, membership staffLookup) activitiesModels.GroupRepository {
	wrapped := staffActivityGroupRepository{GroupRepository: inner, membership: membership}
	if targets, ok := inner.(activitiesModels.GroupTargetRepository); ok {
		return staffActivityGroupTargetRepository{staffActivityGroupRepository: wrapped, GroupTargetRepository: targets}
	}
	return wrapped
}

// staffSupervisorPlannedRepository attaches the staff member to planned
// supervisor rows. The replaced LEFT JOIN carried no soft-delete filter, so
// an offboarded supervisor still resolves.
type staffSupervisorPlannedRepository struct {
	activitiesModels.SupervisorPlannedRepository
	membership staffLookup
}

func (r staffSupervisorPlannedRepository) FindByGroupID(ctx context.Context, groupID int64) ([]*activitiesModels.SupervisorPlanned, error) {
	rows, err := r.SupervisorPlannedRepository.FindByGroupID(ctx, groupID)
	if err != nil {
		return nil, err
	}
	return rows, attachPlannedSupervisorStaff(ctx, r.membership, rows, true)
}

func (r staffSupervisorPlannedRepository) FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*activitiesModels.SupervisorPlanned, error) {
	rows, err := r.SupervisorPlannedRepository.FindByGroupIDs(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	return rows, attachPlannedSupervisorStaff(ctx, r.membership, rows, true)
}

func attachPlannedSupervisorStaff(ctx context.Context, query staffLookup, rows []*activitiesModels.SupervisorPlanned, includeDeleted bool) error {
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			ids = append(ids, row.StaffID)
		}
	}
	members, err := staffByID(ctx, query, ids, includeDeleted)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row == nil {
			continue
		}
		if member, found := members[row.StaffID]; found {
			row.Staff = toLegacyStaff(member)
		}
	}
	return nil
}
