package repositories

import (
	"context"
	"errors"
	"fmt"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

type timetableActivitySupervisorRepository struct{ timetable timetable.Capability }

func (r timetableActivitySupervisorRepository) Create(ctx context.Context, supervisor *activitiesModels.SupervisorPlanned) error {
	if supervisor == nil {
		return errors.New("supervisor cannot be nil or zero value")
	}
	if err := supervisor.Validate(); err != nil {
		return err
	}
	created, err := r.timetable.CreatePlannedSupervisor(ctx, publicPlannedSupervisorInput(supervisor))
	if err != nil {
		return legacyDatabaseError("create", err)
	}
	*supervisor = *legacyPlannedSupervisor(created)
	return nil
}

func (r timetableActivitySupervisorRepository) FindByID(ctx context.Context, id any) (*activitiesModels.SupervisorPlanned, error) {
	supervisorID, ok := legacyGroupID(id)
	if !ok {
		return nil, legacyDatabaseError("find by id", fmt.Errorf("invalid planned supervisor id %T", id))
	}
	value, err := r.timetable.FindPlannedSupervisor(ctx, supervisorID)
	if err != nil {
		return nil, legacyPlannedSupervisorError("find by id", err)
	}
	return legacyPlannedSupervisor(value), nil
}

func (r timetableActivitySupervisorRepository) Update(ctx context.Context, supervisor *activitiesModels.SupervisorPlanned) error {
	if supervisor == nil {
		return errors.New("supervisor cannot be nil")
	}
	if err := supervisor.Validate(); err != nil {
		return err
	}
	updated, err := r.timetable.UpdatePlannedSupervisor(ctx, supervisor.ID, publicPlannedSupervisorInput(supervisor))
	if err != nil {
		return legacyPlannedSupervisorError("update", err)
	}
	*supervisor = *legacyPlannedSupervisor(updated)
	return nil
}

func (r timetableActivitySupervisorRepository) Delete(ctx context.Context, id any) error {
	supervisorID, ok := legacyGroupID(id)
	if !ok {
		return legacyDatabaseError("delete", fmt.Errorf("invalid planned supervisor id %T", id))
	}
	if err := r.timetable.DeletePlannedSupervisor(ctx, supervisorID); err != nil {
		return legacyDatabaseError("delete", err)
	}
	return nil
}

func (r timetableActivitySupervisorRepository) List(ctx context.Context, options *activitiesModels.SupervisorQueryOptions) ([]*activitiesModels.SupervisorPlanned, error) {
	if options != nil {
		return nil, legacyDatabaseError("list", errors.New("planned supervisor filters are unsupported"))
	}
	values, err := r.timetable.ListPlannedSupervisors(ctx, timetable.PlannedSupervisorFilter{})
	if err != nil {
		return nil, legacyPlannedSupervisorError("list", err)
	}
	return legacyPlannedSupervisors(values), nil
}

func (r timetableActivitySupervisorRepository) FindByStaffID(ctx context.Context, staffID int64) ([]*activitiesModels.SupervisorPlanned, error) {
	values, err := r.timetable.ListPlannedSupervisors(ctx, timetable.PlannedSupervisorFilter{StaffID: &staffID})
	if err != nil {
		return nil, legacyPlannedSupervisorError("find by staff ID", err)
	}
	result := legacyPlannedSupervisors(values)
	if len(result) == 0 {
		return result, nil
	}
	groupIDs := make([]int64, 0, len(result))
	for _, supervisor := range result {
		groupIDs = append(groupIDs, supervisor.GroupID)
	}
	groups, err := r.timetable.ListGroups(ctx, timetable.GroupFilter{IDs: groupIDs})
	if err != nil {
		return nil, legacyPlannedSupervisorError("find groups by supervisor staff ID", err)
	}
	groupsByID := make(map[int64]*activitiesModels.Group, len(groups))
	for _, group := range groups {
		groupsByID[group.ID] = legacyGroup(group)
	}
	for _, supervisor := range result {
		supervisor.Group = groupsByID[supervisor.GroupID]
	}
	return result, nil
}

func (r timetableActivitySupervisorRepository) FindByGroupID(ctx context.Context, groupID int64) ([]*activitiesModels.SupervisorPlanned, error) {
	return r.findByGroupIDs(ctx, []int64{groupID}, "find by group ID")
}

func (r timetableActivitySupervisorRepository) FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*activitiesModels.SupervisorPlanned, error) {
	if len(groupIDs) == 0 {
		return []*activitiesModels.SupervisorPlanned{}, nil
	}
	return r.findByGroupIDs(ctx, groupIDs, "find by group IDs")
}

func (r timetableActivitySupervisorRepository) findByGroupIDs(ctx context.Context, groupIDs []int64, operation string) ([]*activitiesModels.SupervisorPlanned, error) {
	values, err := r.timetable.ListPlannedSupervisors(ctx, timetable.PlannedSupervisorFilter{GroupIDs: groupIDs})
	if err != nil {
		return nil, legacyPlannedSupervisorError(operation, err)
	}
	return legacyPlannedSupervisors(values), nil
}

func (r timetableActivitySupervisorRepository) SetPrimary(ctx context.Context, id int64) error {
	if err := r.timetable.SetPrimaryPlannedSupervisor(ctx, id); err != nil {
		return legacyPlannedSupervisorError("set primary", err)
	}
	return nil
}

func (r timetableActivitySupervisorRepository) DeleteByStaffID(ctx context.Context, staffID int64) (int64, error) {
	rows, err := r.timetable.DeletePlannedSupervisorsByStaff(ctx, staffID)
	if err != nil {
		return 0, legacyDatabaseError("delete by staff id", err)
	}
	return rows, nil
}

func (r timetableActivitySupervisorRepository) CapActiveByGroup(ctx context.Context, groupID int64, validUntil activitiesModels.SupervisorDate) (int64, error) {
	rows, err := r.timetable.CapActivePlannedSupervisors(ctx, groupID, validUntil.String())
	if err != nil {
		return rows, legacyDatabaseError("cap active supervisors by group", err)
	}
	return rows, nil
}

func (r timetableActivitySupervisorRepository) SetValidUntilByID(ctx context.Context, id int64, validUntil activitiesModels.SupervisorDate) error {
	if err := r.timetable.SetPlannedSupervisorValidUntil(ctx, id, validUntil.String()); err != nil {
		return legacyPlannedSupervisorError("set supervisor valid_until", err)
	}
	return nil
}

func (r timetableActivitySupervisorRepository) CloseOpenByGroupAndPeriod(ctx context.Context, groupID int64, periodID *int64, validFrom activitiesModels.SupervisorDate) error {
	if err := r.timetable.CloseOpenPlannedSupervisors(ctx, groupID, periodID, validFrom.String()); err != nil {
		return legacyDatabaseError("close open supervisors by group and period", err)
	}
	return nil
}

func (r timetableActivitySupervisorRepository) ListPlannedSupervisionBlockers(ctx context.Context, staffID, tenantID int64) ([]userModels.BlockerActivity, error) {
	values, err := r.timetable.ListPlannedSupervisionBlockers(ctx, staffID, tenantID)
	if err != nil {
		return nil, legacyDatabaseError("list planned supervision blockers", err)
	}
	result := make([]userModels.BlockerActivity, 0, len(values))
	for _, value := range values {
		result = append(result, userModels.BlockerActivity{ID: value.ID, ActivityID: value.ActivityID, ActivityName: value.ActivityName, IsPrimary: value.IsPrimary})
	}
	return result, nil
}

func legacyPlannedSupervisors(values []timetable.PlannedSupervisor) []*activitiesModels.SupervisorPlanned {
	result := make([]*activitiesModels.SupervisorPlanned, 0, len(values))
	for _, value := range values {
		result = append(result, legacyPlannedSupervisor(value))
	}
	return result
}

func legacyPlannedSupervisor(value timetable.PlannedSupervisor) *activitiesModels.SupervisorPlanned {
	result := &activitiesModels.SupervisorPlanned{StaffID: value.StaffID, GroupID: value.GroupID, IsPrimary: value.IsPrimary,
		CalendarPeriodID: value.CalendarPeriodID, Weekday: value.Weekday}
	result.ID, result.CreatedAt, result.UpdatedAt = value.ID, value.CreatedAt, value.UpdatedAt
	result.SetTenantID(value.TenantID)
	result.SetValidityDateStrings(value.ValidFrom, value.ValidUntil)
	return result
}

func publicPlannedSupervisorInput(value *activitiesModels.SupervisorPlanned) timetable.PlannedSupervisorInput {
	validFrom, validUntil := value.ValidityDateStrings()
	return timetable.PlannedSupervisorInput{StaffID: value.StaffID, GroupID: value.GroupID, IsPrimary: value.IsPrimary,
		ValidFrom: validFrom, ValidUntil: validUntil, CalendarPeriodID: value.CalendarPeriodID, Weekday: value.Weekday}
}

func legacyPlannedSupervisorError(operation string, err error) error {
	if errors.Is(err, timetable.ErrPlannedSupervisorNotFound) || errors.Is(err, timetable.ErrInvalidPlannedSupervisorQuery) {
		return legacyNotFoundError(operation)
	}
	return legacyDatabaseError(operation, err)
}
