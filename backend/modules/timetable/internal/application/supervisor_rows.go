package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// The activity service, the template writes and split and the materializer
// still compute on the legacy planned supervisor row. They call the owner's
// planned supervisor capability through these reads and writes, which
// convert the rows and report failures under the operation names the
// retained repository used. The group reads name each supervisor's staff
// member and person through the directories, as the retained repository did.

func createLegacyPlannedSupervisor(ctx context.Context, supervisors timetable.PlannedSupervisorCommand, supervisor *activityContract.SupervisorPlanned) error {
	if supervisor == nil {
		return errors.New("supervisor cannot be nil or zero value")
	}
	if err := supervisor.Validate(); err != nil {
		return err
	}
	created, err := supervisors.CreatePlannedSupervisor(ctx, publicPlannedSupervisorInput(supervisor))
	if err != nil {
		return legacyDatabaseError("create", err)
	}
	*supervisor = *legacyPlannedSupervisor(created)
	return nil
}

func findLegacyPlannedSupervisor(ctx context.Context, supervisors timetable.PlannedSupervisorQuery, id int64) (*activityContract.SupervisorPlanned, error) {
	value, err := supervisors.FindPlannedSupervisor(ctx, id)
	if err != nil {
		return nil, legacyPlannedSupervisorError("find by id", err)
	}
	return legacyPlannedSupervisor(value), nil
}

func updateLegacyPlannedSupervisor(ctx context.Context, supervisors timetable.PlannedSupervisorCommand, supervisor *activityContract.SupervisorPlanned) error {
	if supervisor == nil {
		return errors.New("supervisor cannot be nil")
	}
	if err := supervisor.Validate(); err != nil {
		return err
	}
	updated, err := supervisors.UpdatePlannedSupervisor(ctx, supervisor.ID, publicPlannedSupervisorInput(supervisor))
	if err != nil {
		return legacyPlannedSupervisorError("update", err)
	}
	*supervisor = *legacyPlannedSupervisor(updated)
	return nil
}

func deleteLegacyPlannedSupervisor(ctx context.Context, supervisors timetable.PlannedSupervisorCommand, id int64) error {
	if err := supervisors.DeletePlannedSupervisor(ctx, id); err != nil {
		return legacyDatabaseError("delete", err)
	}
	return nil
}

// groupPlannedSupervisors lists a group's planned supervisors, named.
func groupPlannedSupervisors(ctx context.Context, supervisors timetable.PlannedSupervisorQuery, directories ports.RepositoryDirectories, groupID int64) ([]*activityContract.SupervisorPlanned, error) {
	return namedPlannedSupervisors(ctx, supervisors, directories, []int64{groupID}, "find by group ID")
}

// groupsPlannedSupervisors lists the planned supervisors of several groups,
// named; no groups list nobody.
func groupsPlannedSupervisors(ctx context.Context, supervisors timetable.PlannedSupervisorQuery, directories ports.RepositoryDirectories, groupIDs []int64) ([]*activityContract.SupervisorPlanned, error) {
	if len(groupIDs) == 0 {
		return namePlannedSupervisors(ctx, directories, []*activityContract.SupervisorPlanned{})
	}
	return namedPlannedSupervisors(ctx, supervisors, directories, groupIDs, "find by group IDs")
}

// namedPlannedSupervisors attaches the staff member to each row, offboarded
// ones included as the replaced LEFT JOIN did, and then the person's name.
func namedPlannedSupervisors(ctx context.Context, supervisors timetable.PlannedSupervisorQuery, directories ports.RepositoryDirectories, groupIDs []int64, operation string) ([]*activityContract.SupervisorPlanned, error) {
	values, err := supervisors.ListPlannedSupervisors(ctx, timetable.PlannedSupervisorFilter{GroupIDs: groupIDs})
	if err != nil {
		return nil, legacyPlannedSupervisorError(operation, err)
	}
	return namePlannedSupervisors(ctx, directories, legacyPlannedSupervisors(values))
}

// namePlannedSupervisors attaches the staff member's person and its name to
// each row.
func namePlannedSupervisors(ctx context.Context, directories ports.RepositoryDirectories, rows []*activityContract.SupervisorPlanned) ([]*activityContract.SupervisorPlanned, error) {
	if err := attachPlannedSupervisorStaff(ctx, directories.StaffPersons, rows, true); err != nil {
		return nil, err
	}
	return rows, attachPlannedSupervisorPersons(ctx, directories.PersonNames, rows)
}

func capLegacyPlannedSupervisors(ctx context.Context, supervisors timetable.PlannedSupervisorCommand, groupID int64, validUntil activityContract.Date) (int64, error) {
	rows, err := supervisors.CapActivePlannedSupervisors(ctx, groupID, validUntil.String())
	if err != nil {
		return rows, legacyDatabaseError("cap active supervisors by group", err)
	}
	return rows, nil
}

func setLegacyPlannedSupervisorValidUntil(ctx context.Context, supervisors timetable.PlannedSupervisorCommand, id int64, validUntil activityContract.Date) error {
	if err := supervisors.SetPlannedSupervisorValidUntil(ctx, id, validUntil.String()); err != nil {
		return legacyPlannedSupervisorError("set supervisor valid_until", err)
	}
	return nil
}

func closeOpenLegacyPlannedSupervisors(ctx context.Context, supervisors timetable.PlannedSupervisorCommand, groupID int64, periodID *int64, validFrom activityContract.Date) error {
	if err := supervisors.CloseOpenPlannedSupervisors(ctx, groupID, periodID, validFrom.String()); err != nil {
		return legacyDatabaseError("close open supervisors by group and period", err)
	}
	return nil
}

func attachPlannedSupervisorStaff(ctx context.Context, staffPersons func(context.Context, []int64, bool) (map[int64]int64, error), rows []*activityContract.SupervisorPlanned, includeDeleted bool) error {
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			ids = append(ids, row.StaffID)
		}
	}
	members, err := staffPersons(ctx, ids, includeDeleted)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row == nil {
			continue
		}
		if personID, found := members[row.StaffID]; found {
			row.StaffPersonID = personID
		}
	}
	return nil
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
