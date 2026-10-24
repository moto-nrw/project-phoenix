package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The readers below serve the timetable facts the Workforce Dienstplan and
// the shift-plan-sync workflow read, in the Timetable owner's public
// vocabulary, from the retained repositories (#3424). They run the same
// queries and return the same errors those repositories do; only the row
// type changes. The instance rows keep the Student Presence session state in
// their status (active, completed), which the owner's planning rows do not
// carry and which the Dienstplan and the sick cascade classify on.

// TimetableInstanceReads serves the activity instances with their session
// state.
type TimetableInstanceReads struct {
	instances ports.LegacyInstanceReads
}

// NewTimetableInstanceReads binds the instance reads to the retained
// presence-aware instance repository.
func NewTimetableInstanceReads(instances ports.LegacyInstanceReads) TimetableInstanceReads {
	if instances == nil {
		panic("timetable instance reads: the activity instance repository is required")
	}
	return TimetableInstanceReads{instances: instances}
}

// FindByTenantAndDateRange returns the tenant's instances within the
// inclusive date range, ordered by date and time.
func (r TimetableInstanceReads) FindByTenantAndDateRange(ctx context.Context, from, to calendar.Date) ([]*timetable.ScheduledInstance, error) {
	fromDay, toDay := from.String(), to.String()
	rows, err := r.instances.ListLegacyInstances(ctx, ports.LegacyInstanceFilter{FromDate: &fromDay, ToDate: &toDay, OrderByDateAndTime: true})
	if err != nil {
		return nil, WrapDatabaseError("find by tenant and date range", err)
	}
	return scheduledInstances(rows), nil
}

// FindByIDs returns the instances with the given IDs.
func (r TimetableInstanceReads) FindByIDs(ctx context.Context, ids []int64) ([]*timetable.ScheduledInstance, error) {
	if len(ids) == 0 {
		return scheduledInstances([]*scheduleModels.ActivityInstance{}), nil
	}
	rows, err := r.instances.ListLegacyInstances(ctx, ports.LegacyInstanceFilter{IDs: ids, OrderByDateAndTime: true})
	if err != nil {
		return nil, WrapDatabaseError("find by ids", err)
	}
	return scheduledInstances(rows), nil
}

// GetActivityInstancesByID indexes the instances with the given IDs.
func (r TimetableInstanceReads) GetActivityInstancesByID(ctx context.Context, ids []int64) (map[int64]*timetable.ScheduledInstance, error) {
	instances, err := r.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]*timetable.ScheduledInstance, len(instances))
	for _, instance := range instances {
		if instance != nil {
			byID[instance.ID] = instance
		}
	}
	return byID, nil
}

func scheduledInstances(rows []*scheduleModels.ActivityInstance) []*timetable.ScheduledInstance {
	result := make([]*timetable.ScheduledInstance, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			result = append(result, nil)
			continue
		}
		instance := ScheduledInstanceOf(row)
		result = append(result, &instance)
	}
	return result
}

// TimetableInstanceStaffReads serves the staff assignments of the activity
// instances and the sick-report provenance stamp on them (#1843).
type TimetableInstanceStaffReads struct {
	staff timetable.InstanceStaffCapability
}

// NewTimetableInstanceStaffReads binds the assignment reads to the owner's
// instance staff capability.
func NewTimetableInstanceStaffReads(staff timetable.InstanceStaffCapability) TimetableInstanceStaffReads {
	if staff == nil {
		panic("timetable instance staff reads: the instance staff capability is required")
	}
	return TimetableInstanceStaffReads{staff: staff}
}

// FindByID returns one assignment; a missing row keeps the repository's
// no-rows error.
func (r TimetableInstanceStaffReads) FindByID(ctx context.Context, id int64) (*timetable.InstanceStaff, error) {
	value, err := r.staff.FindInstanceStaff(ctx, id)
	if err != nil {
		return nil, legacyInstanceStaffError("find by id", err)
	}
	return &value, nil
}

// FindByInstanceID returns the assignments of one instance.
func (r TimetableInstanceStaffReads) FindByInstanceID(ctx context.Context, instanceID int64) ([]*timetable.InstanceStaff, error) {
	values, err := r.staff.ListInstanceStaff(ctx, timetable.InstanceStaffFilter{InstanceIDs: []int64{instanceID}, OrderByCreated: true})
	return instanceStaffPointers(values, err, "find by instance id")
}

// FindByInstanceIDs returns the assignments of the given instances.
func (r TimetableInstanceStaffReads) FindByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*timetable.InstanceStaff, error) {
	if len(instanceIDs) == 0 {
		return []*timetable.InstanceStaff{}, nil
	}
	values, err := r.staff.ListInstanceStaff(ctx, timetable.InstanceStaffFilter{InstanceIDs: instanceIDs, OrderByInstanceAndCreated: true})
	return instanceStaffPointers(values, err, "find by instance ids")
}

// FindByStaffAndDate returns one staff member's assignments on a day.
func (r TimetableInstanceStaffReads) FindByStaffAndDate(ctx context.Context, staffID int64, date calendar.Date) ([]*timetable.InstanceStaff, error) {
	day := date.String()
	values, err := r.staff.ListInstanceStaff(ctx, timetable.InstanceStaffFilter{StaffIDs: []int64{staffID}, Date: &day, OrderByActivityTime: true})
	return instanceStaffPointers(values, err, "find by staff and date")
}

// FindByStaffAndDateRange returns one staff member's assignments in the
// inclusive date range.
func (r TimetableInstanceStaffReads) FindByStaffAndDateRange(ctx context.Context, staffID int64, from, to calendar.Date) ([]*timetable.InstanceStaff, error) {
	fromDay, toDay := from.String(), to.String()
	values, err := r.staff.ListInstanceStaff(ctx, timetable.InstanceStaffFilter{
		StaffIDs: []int64{staffID}, FromDate: &fromDay, ToDate: &toDay, OrderByActivityDateTime: true,
	})
	return instanceStaffPointers(values, err, "find by staff and date range")
}

// ListBySickAbsence returns the assignments a sick report stamped.
func (r TimetableInstanceStaffReads) ListBySickAbsence(ctx context.Context, absenceID int64) ([]*timetable.InstanceStaff, error) {
	values, err := r.staff.ListInstanceStaff(ctx, timetable.InstanceStaffFilter{SickAbsenceID: &absenceID})
	return instanceStaffPointers(values, err, "list with options")
}

// SetSickAbsence writes the provenance stamp of one assignment; nil clears
// it. Only the stamp column is written.
func (r TimetableInstanceStaffReads) SetSickAbsence(ctx context.Context, assignmentID int64, absenceID *int64) (int64, error) {
	rows, err := r.staff.PatchInstanceStaff(ctx, assignmentID, timetable.InstanceStaffInput{SickAbsenceID: absenceID}, []string{"sick_absence_id"})
	if err != nil {
		return 0, WrapDatabaseError("update columns", err)
	}
	return rows, nil
}

// instanceStaffPointers hands out the listed assignments, or reports a
// failed listing under the operation's name.
func instanceStaffPointers(values []timetable.InstanceStaff, err error, operation string) ([]*timetable.InstanceStaff, error) {
	if err != nil {
		return nil, WrapDatabaseError(operation, err)
	}
	result := make([]*timetable.InstanceStaff, len(values))
	for index := range values {
		result[index] = &values[index]
	}
	return result, nil
}

// TimetableGroupReads serves the activity groups (Angebote) an assignment
// belongs to.
type TimetableGroupReads struct {
	groups timetable.ActivityGroupQuery
}

// NewTimetableGroupReads binds the group reads to the owner's group
// capability.
func NewTimetableGroupReads(groups timetable.ActivityGroupQuery) TimetableGroupReads {
	if groups == nil {
		panic("timetable group reads: the activity group capability is required")
	}
	return TimetableGroupReads{groups: groups}
}

// FindByIDs returns the groups with the given IDs.
func (r TimetableGroupReads) FindByIDs(ctx context.Context, ids []int64) ([]*timetable.Group, error) {
	if len(ids) == 0 {
		return []*timetable.Group{}, nil
	}
	groups, err := r.groups.ListGroups(ctx, timetable.GroupFilter{IDs: ids})
	if err != nil {
		return nil, legacyGroupReadError("find groups by ids", err)
	}
	return groupPointers(groups), nil
}

// FindByID returns the group with the given ID.
func (r TimetableGroupReads) FindByID(ctx context.Context, id any) (*timetable.Group, error) {
	groupID, ok := LegacyGroupID(id)
	if !ok {
		return nil, legacyDatabaseError("find by id", fmt.Errorf("invalid activity group id %T", id))
	}
	group, err := r.groups.FindGroup(ctx, groupID)
	if err != nil {
		return nil, legacyGroupReadError("find by id", err)
	}
	return &group, nil
}

// ListWithCategory returns every group with its category.
func (r TimetableGroupReads) ListWithCategory(ctx context.Context) ([]*timetable.Group, error) {
	groups, err := r.groups.ListGroups(ctx, timetable.GroupFilter{})
	if err != nil {
		return nil, legacyGroupReadError("list", err)
	}
	return groupPointers(groups), nil
}

func groupPointers(groups []timetable.Group) []*timetable.Group {
	result := make([]*timetable.Group, len(groups))
	for index := range groups {
		result[index] = &groups[index]
	}
	return result
}
