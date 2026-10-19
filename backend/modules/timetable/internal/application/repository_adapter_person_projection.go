package application

import (
	"context"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// personActivityGroupRepository attaches Staff.Person to the supervisors a
// group is loaded with.
type personActivityGroupRepository struct {
	ports.ActivityGroupRecords
	persons func(context.Context, []int64) (map[int64]ports.PersonName, error)
}

func (r personActivityGroupRepository) FindWithSupervisors(ctx context.Context, groupID int64) (*activityContract.Group, []*activityContract.SupervisorPlanned, error) {
	group, supervisors, err := r.ActivityGroupRecords.FindWithSupervisors(ctx, groupID)
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
	ports.ActivityGroupTargets
}

func newPersonActivityGroupRepository(inner ports.ActivityGroupRecords, persons func(context.Context, []int64) (map[int64]ports.PersonName, error)) ports.ActivityGroupRecords {
	wrapped := personActivityGroupRepository{ActivityGroupRecords: inner, persons: persons}
	if targets, ok := inner.(ports.ActivityGroupTargets); ok {
		return personActivityGroupTargetRepository{personActivityGroupRepository: wrapped, ActivityGroupTargets: targets}
	}
	return wrapped
}

// personSupervisorPlannedRepository attaches Staff.Person to planned
// supervisor rows.
type personSupervisorPlannedRepository struct {
	ports.PlannedSupervisorRecords
	persons func(context.Context, []int64) (map[int64]ports.PersonName, error)
}

func (r personSupervisorPlannedRepository) FindByGroupID(ctx context.Context, groupID int64) ([]*activityContract.SupervisorPlanned, error) {
	rows, err := r.PlannedSupervisorRecords.FindByGroupID(ctx, groupID)
	if err != nil {
		return nil, err
	}
	return rows, attachPlannedSupervisorPersons(ctx, r.persons, rows)
}

func (r personSupervisorPlannedRepository) FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*activityContract.SupervisorPlanned, error) {
	rows, err := r.PlannedSupervisorRecords.FindByGroupIDs(ctx, groupIDs)
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
