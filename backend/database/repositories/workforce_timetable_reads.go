package repositories

import (
	"context"
	"fmt"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
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
	instances scheduleModels.ActivityInstanceRepository
}

// NewTimetableInstanceReads binds the instance reads to the retained
// presence-aware instance repository.
func NewTimetableInstanceReads(instances scheduleModels.ActivityInstanceRepository) TimetableInstanceReads {
	if instances == nil {
		panic("timetable instance reads: the activity instance repository is required")
	}
	return TimetableInstanceReads{instances: instances}
}

// FindByTenantAndDateRange returns the tenant's instances within the
// inclusive date range, ordered by date and time.
func (r TimetableInstanceReads) FindByTenantAndDateRange(ctx context.Context, from, to calendar.Date) ([]*timetable.ScheduledInstance, error) {
	rows, err := r.instances.FindByTenantAndDateRange(ctx, scheduleModels.Date(from), scheduleModels.Date(to))
	if err != nil {
		return nil, err
	}
	return scheduledInstances(rows), nil
}

// FindByIDs returns the instances with the given IDs.
func (r TimetableInstanceReads) FindByIDs(ctx context.Context, ids []int64) ([]*timetable.ScheduledInstance, error) {
	rows, err := r.instances.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
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
		instance := timetableCompose.ScheduledInstanceOf(row)
		result = append(result, &instance)
	}
	return result
}

// TimetableInstanceStaffReads serves the staff assignments of the activity
// instances and the sick-report provenance stamp on them (#1843).
type TimetableInstanceStaffReads struct {
	staff scheduleModels.InstanceStaffRepository
}

// NewTimetableInstanceStaffReads binds the assignment reads to the retained
// instance staff repository.
func NewTimetableInstanceStaffReads(staff scheduleModels.InstanceStaffRepository) TimetableInstanceStaffReads {
	if staff == nil {
		panic("timetable instance staff reads: the instance staff repository is required")
	}
	return TimetableInstanceStaffReads{staff: staff}
}

// FindByID returns one assignment; a missing row keeps the repository's
// no-rows error.
func (r TimetableInstanceStaffReads) FindByID(ctx context.Context, id int64) (*timetable.InstanceStaff, error) {
	row, err := r.staff.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return publicInstanceStaff(row), nil
}

// FindByInstanceID returns the assignments of one instance.
func (r TimetableInstanceStaffReads) FindByInstanceID(ctx context.Context, instanceID int64) ([]*timetable.InstanceStaff, error) {
	return instanceStaffRows(r.staff.FindByInstanceID(ctx, instanceID))
}

// FindByInstanceIDs returns the assignments of the given instances.
func (r TimetableInstanceStaffReads) FindByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*timetable.InstanceStaff, error) {
	return instanceStaffRows(r.staff.FindByInstanceIDs(ctx, instanceIDs))
}

// FindByStaffAndDate returns one staff member's assignments on a day.
func (r TimetableInstanceStaffReads) FindByStaffAndDate(ctx context.Context, staffID int64, date calendar.Date) ([]*timetable.InstanceStaff, error) {
	return instanceStaffRows(r.staff.FindByStaffAndDate(ctx, staffID, scheduleModels.Date(date)))
}

// FindByStaffAndDateRange returns one staff member's assignments in the
// inclusive date range.
func (r TimetableInstanceStaffReads) FindByStaffAndDateRange(ctx context.Context, staffID int64, from, to calendar.Date) ([]*timetable.InstanceStaff, error) {
	return instanceStaffRows(r.staff.FindByStaffAndDateRange(ctx, staffID, scheduleModels.Date(from), scheduleModels.Date(to)))
}

// instanceStaffFilterLister is the owner-filtered listing the retained
// instance staff repository runs its List through.
type instanceStaffFilterLister interface {
	list(ctx context.Context, filter timetable.InstanceStaffFilter, operation string) ([]*scheduleModels.InstanceStaff, error)
}

// ListBySickAbsence returns the assignments a sick report stamped, the way
// the retained repository's filtered List answers a sick_absence_id filter.
func (r TimetableInstanceStaffReads) ListBySickAbsence(ctx context.Context, absenceID int64) ([]*timetable.InstanceStaff, error) {
	lister, ok := r.staff.(instanceStaffFilterLister)
	if !ok {
		return nil, fmt.Errorf("legacy list capability is not configured for %T", r.staff)
	}
	return instanceStaffRows(lister.list(ctx, timetable.InstanceStaffFilter{SickAbsenceID: &absenceID}, "list with options"))
}

// SetSickAbsence writes the provenance stamp of one assignment; nil clears
// it. Only the stamp column is written.
func (r TimetableInstanceStaffReads) SetSickAbsence(ctx context.Context, assignmentID int64, absenceID *int64) (int64, error) {
	row := &scheduleModels.InstanceStaff{SickAbsenceID: absenceID}
	row.ID = assignmentID
	return r.staff.UpdateColumns(ctx, row, "sick_absence_id")
}

func instanceStaffRows(rows []*scheduleModels.InstanceStaff, err error) ([]*timetable.InstanceStaff, error) {
	if err != nil {
		return nil, err
	}
	result := make([]*timetable.InstanceStaff, 0, len(rows))
	for _, row := range rows {
		result = append(result, publicInstanceStaff(row))
	}
	return result, nil
}

func publicInstanceStaff(row *scheduleModels.InstanceStaff) *timetable.InstanceStaff {
	if row == nil {
		return nil
	}
	return &timetable.InstanceStaff{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		InstanceID: row.InstanceID, StaffID: row.StaffID, RoomID: row.RoomID,
		IsPrimary: row.IsPrimary, IsSubstitute: row.IsSubstitute, IsAbsent: row.IsAbsent,
		AbsenceReason: row.AbsenceReason, SickAbsenceID: row.SickAbsenceID,
	}
}

// TimetableGroupReads serves the activity groups (Angebote) an assignment
// belongs to.
type TimetableGroupReads struct {
	groups activitiesModels.GroupRepository
}

// NewTimetableGroupReads binds the group reads to the retained activity
// group repository.
func NewTimetableGroupReads(groups activitiesModels.GroupRepository) TimetableGroupReads {
	if groups == nil {
		panic("timetable group reads: the activity group repository is required")
	}
	return TimetableGroupReads{groups: groups}
}

// FindByIDs returns the groups with the given IDs.
func (r TimetableGroupReads) FindByIDs(ctx context.Context, ids []int64) ([]*timetable.Group, error) {
	rows, err := r.groups.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]*timetable.Group, 0, len(rows))
	for _, row := range rows {
		result = append(result, publicGroup(row))
	}
	return result, nil
}

func publicGroup(group *activitiesModels.Group) *timetable.Group {
	if group == nil {
		return nil
	}
	result := &timetable.Group{
		ID: group.ID, TenantID: group.TenantID, CreatedAt: group.CreatedAt, UpdatedAt: group.UpdatedAt,
		Name: group.Name, MaxParticipants: group.MaxParticipants, RequiredStaff: group.RequiredStaff,
		IsOpen: group.IsOpen, CategoryID: group.CategoryID, PlanningTrackID: group.PlanningTrackID,
		PlannedRoomID: group.PlannedRoomID, CreatedBy: group.CreatedBy, Type: group.Type,
		EducationGroupID: group.EducationGroupID, ListKind: group.ListKind, IsTemplate: group.IsTemplate,
		IsSystem: group.IsSystem, ArchivedAt: group.ArchivedAt, SeriesRootID: group.SeriesRootID,
		CalendarPeriodID: group.CalendarPeriodID, TargetGroupType: group.TargetGroupType,
		TargetGradeLevel: group.TargetGradeLevel, TargetSchoolClass: group.TargetSchoolClass,
		SourceCareOfferingIDs: group.SourceCareOfferingIDs, SourceGradeLevels: group.SourceGradeLevels,
		SourceSchoolClasses: group.SourceSchoolClasses, Notes: group.Notes,
	}
	if category := group.Category; category != nil {
		result.Category = &timetable.Category{
			ID: category.ID, TenantID: category.TenantID, CreatedAt: category.CreatedAt, UpdatedAt: category.UpdatedAt,
			Name: category.Name, Description: category.Description, Color: category.Color,
			IsSystem: category.IsSystem, ShiftTypeID: category.ShiftTypeID, ArchivedAt: category.ArchivedAt,
		}
	}
	return result
}
