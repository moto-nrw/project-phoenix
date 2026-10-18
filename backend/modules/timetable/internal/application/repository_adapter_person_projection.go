package application

import (
	"context"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// personActivityGroupRepository attaches Staff.Person to the supervisors a
// group is loaded with.
type personActivityGroupRepository struct {
	activitiesModels.GroupRepository
	persons func(context.Context, []int64) (map[int64]ports.PersonName, error)
}

func (r personActivityGroupRepository) FindWithSupervisors(ctx context.Context, groupID int64) (*activityContract.Group, []*activityContract.SupervisorPlanned, error) {
	group, supervisors, err := r.GroupRepository.FindWithSupervisors(ctx, groupID)
	if err != nil {
		return nil, nil, err
	}
	if err := attachPlannedSupervisorPersons(ctx, r.persons, supervisors); err != nil {
		return nil, nil, err
	}
	return group, supervisors, nil
}

// personActivityGroupTargetRepository keeps the optional target seam the
// timetable services assert on reachable through the person wrapper.
type personActivityGroupTargetRepository struct {
	personActivityGroupRepository
	activitiesModels.GroupTargetRepository
}

func newPersonActivityGroupRepository(inner activitiesModels.GroupRepository, persons func(context.Context, []int64) (map[int64]ports.PersonName, error)) activitiesModels.GroupRepository {
	wrapped := personActivityGroupRepository{GroupRepository: inner, persons: persons}
	if targets, ok := inner.(activitiesModels.GroupTargetRepository); ok {
		return personActivityGroupTargetRepository{personActivityGroupRepository: wrapped, GroupTargetRepository: targets}
	}
	return wrapped
}

// personSupervisorPlannedRepository attaches Staff.Person to planned
// supervisor rows.
type personSupervisorPlannedRepository struct {
	activitiesModels.SupervisorPlannedRepository
	persons func(context.Context, []int64) (map[int64]ports.PersonName, error)
}

func (r personSupervisorPlannedRepository) FindByGroupID(ctx context.Context, groupID int64) ([]*activityContract.SupervisorPlanned, error) {
	rows, err := r.SupervisorPlannedRepository.FindByGroupID(ctx, groupID)
	if err != nil {
		return nil, err
	}
	return rows, attachPlannedSupervisorPersons(ctx, r.persons, rows)
}

func (r personSupervisorPlannedRepository) FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*activityContract.SupervisorPlanned, error) {
	rows, err := r.SupervisorPlannedRepository.FindByGroupIDs(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	return rows, attachPlannedSupervisorPersons(ctx, r.persons, rows)
}

func attachPlannedSupervisorPersons(ctx context.Context, personNames func(context.Context, []int64) (map[int64]ports.PersonName, error), rows []*activityContract.SupervisorPlanned) error {
	personIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row != nil && row.StaffPersonID > 0 {
			personIDs = append(personIDs, row.StaffPersonID)
		}
	}
	persons, err := personNames(ctx, personIDs)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row == nil {
			continue
		}
		if person, found := persons[row.StaffPersonID]; found {
			row.FirstName = person.FirstName
			row.LastName = person.LastName
		}
	}
	return nil
}
