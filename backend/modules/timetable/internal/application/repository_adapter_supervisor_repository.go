package application

import (
	"context"
	"errors"
	"fmt"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	activitiesModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/activityrecords"
)

type timetableActivitySupervisorRepository struct{ timetable timetable.Capability }

func (r timetableActivitySupervisorRepository) Create(ctx context.Context, supervisor *activityContract.SupervisorPlanned) error {
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

func (r timetableActivitySupervisorRepository) FindByID(ctx context.Context, id int64) (*activityContract.SupervisorPlanned, error) {
	supervisorID, ok := LegacyGroupID(id)
	if !ok {
		return nil, legacyDatabaseError("find by id", fmt.Errorf("invalid planned supervisor id %T", id))
	}
	value, err := r.timetable.FindPlannedSupervisor(ctx, supervisorID)
	if err != nil {
		return nil, legacyPlannedSupervisorError("find by id", err)
	}
	return legacyPlannedSupervisor(value), nil
}

func (r timetableActivitySupervisorRepository) Update(ctx context.Context, supervisor *activityContract.SupervisorPlanned) error {
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

func (r timetableActivitySupervisorRepository) Delete(ctx context.Context, id int64) error {
	supervisorID, ok := LegacyGroupID(id)
	if !ok {
		return legacyDatabaseError("delete", fmt.Errorf("invalid planned supervisor id %T", id))
	}
	if err := r.timetable.DeletePlannedSupervisor(ctx, supervisorID); err != nil {
		return legacyDatabaseError("delete", err)
	}
	return nil
}

func (r timetableActivitySupervisorRepository) FindByStaffID(ctx context.Context, staffID int64) ([]*activityContract.SupervisorPlanned, error) {
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
	groupsByID := make(map[int64]*activityContract.Group, len(groups))
	for _, group := range groups {
		groupsByID[group.ID] = legacyGroup(group)
	}
	for _, supervisor := range result {
		supervisor.Group = groupsByID[supervisor.GroupID]
	}
	return result, nil
}

func (r timetableActivitySupervisorRepository) FindByGroupID(ctx context.Context, groupID int64) ([]*activityContract.SupervisorPlanned, error) {
	return r.findByGroupIDs(ctx, []int64{groupID}, "find by group ID")
}

func (r timetableActivitySupervisorRepository) FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*activityContract.SupervisorPlanned, error) {
	if len(groupIDs) == 0 {
		return []*activityContract.SupervisorPlanned{}, nil
	}
	return r.findByGroupIDs(ctx, groupIDs, "find by group IDs")
}

func (r timetableActivitySupervisorRepository) findByGroupIDs(ctx context.Context, groupIDs []int64, operation string) ([]*activityContract.SupervisorPlanned, error) {
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

func (r timetableActivitySupervisorRepository) CapActiveByGroup(ctx context.Context, groupID int64, validUntil activityContract.Date) (int64, error) {
	rows, err := r.timetable.CapActivePlannedSupervisors(ctx, groupID, validUntil.String())
	if err != nil {
		return rows, legacyDatabaseError("cap active supervisors by group", err)
	}
	return rows, nil
}

func (r timetableActivitySupervisorRepository) SetValidUntilByID(ctx context.Context, id int64, validUntil activityContract.Date) error {
	if err := r.timetable.SetPlannedSupervisorValidUntil(ctx, id, validUntil.String()); err != nil {
		return legacyPlannedSupervisorError("set supervisor valid_until", err)
	}
	return nil
}

func (r timetableActivitySupervisorRepository) CloseOpenByGroupAndPeriod(ctx context.Context, groupID int64, periodID *int64, validFrom activityContract.Date) error {
	if err := r.timetable.CloseOpenPlannedSupervisors(ctx, groupID, periodID, validFrom.String()); err != nil {
		return legacyDatabaseError("close open supervisors by group and period", err)
	}
	return nil
}

func (r timetableActivitySupervisorRepository) ListPlannedSupervisionBlockers(ctx context.Context, staffID, tenantID int64) ([]activitiesModels.PlannedSupervisionBlocker, error) {
	values, err := r.timetable.ListPlannedSupervisionBlockers(ctx, staffID, tenantID)
	if err != nil {
		return nil, legacyDatabaseError("list planned supervision blockers", err)
	}
	result := make([]activitiesModels.PlannedSupervisionBlocker, 0, len(values))
	for _, value := range values {
		result = append(result, activitiesModels.PlannedSupervisionBlocker{ID: value.ID, ActivityID: value.ActivityID, ActivityName: value.ActivityName, IsPrimary: value.IsPrimary})
	}
	return result, nil
}

func legacyPlannedSupervisors(values []timetable.PlannedSupervisor) []*activityContract.SupervisorPlanned {
	result := make([]*activityContract.SupervisorPlanned, 0, len(values))
	for _, value := range values {
		result = append(result, legacyPlannedSupervisor(value))
	}
	return result
}

func legacyPlannedSupervisor(value timetable.PlannedSupervisor) *activityContract.SupervisorPlanned {
	result := &activityContract.SupervisorPlanned{StaffID: value.StaffID, GroupID: value.GroupID, IsPrimary: value.IsPrimary,
		CalendarPeriodID: value.CalendarPeriodID, Weekday: value.Weekday}
	result.ID, result.CreatedAt, result.UpdatedAt = value.ID, value.CreatedAt, value.UpdatedAt
	result.SetTenantID(value.TenantID)
	result.SetValidityDateStrings(value.ValidFrom, value.ValidUntil)
	return result
}

func publicPlannedSupervisorInput(value *activityContract.SupervisorPlanned) timetable.PlannedSupervisorInput {
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
