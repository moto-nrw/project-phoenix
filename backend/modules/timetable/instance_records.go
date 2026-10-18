// Package timetable is the public Timetable & Activities capability. It owns
// activity categories, templates, target groups, schedules, rosters, and
// materialized activity instances.
package timetable

import (
	"context"
)

func (m *Module) FindActivityInstance(ctx context.Context, id int64) (ActivityInstance, error) {
	if id <= 0 {
		return ActivityInstance{}, m.reject("find_activity_instance", ErrInvalidActivityInstanceQuery)
	}
	return m.engine.FindActivityInstance(ctx, id)
}

func (m *Module) ListActivityInstances(ctx context.Context, filter ActivityInstanceFilter) ([]ActivityInstance, error) {
	if !validActivityInstanceFilter(filter) {
		return nil, m.reject("list_activity_instances", ErrInvalidActivityInstanceQuery)
	}
	return m.engine.ListActivityInstances(ctx, filter)
}

func (m *Module) MaxActivityInstanceID(ctx context.Context) (int64, error) {
	return m.engine.MaxActivityInstanceID(ctx)
}

func (m *Module) CountActivityInstances(ctx context.Context, before *string) (int, error) {
	if !validOptionalDate(before) {
		return 0, m.reject("count_activity_instances", ErrInvalidActivityInstanceQuery)
	}
	return m.engine.CountActivityInstances(ctx, before)
}

func (m *Module) OldestActivityInstanceBefore(ctx context.Context, before *string) (*string, error) {
	if !validOptionalDate(before) {
		return nil, m.reject("oldest_activity_instance", ErrInvalidActivityInstanceQuery)
	}
	return m.engine.OldestActivityInstanceBefore(ctx, before)
}

func (m *Module) CreateActivityInstance(ctx context.Context, input ActivityInstanceInput) (ActivityInstance, error) {
	input, ok := normalizeActivityInstance(input)
	if !ok {
		return ActivityInstance{}, m.reject("create_activity_instance", ErrInvalidActivityInstance)
	}
	return m.engine.CreateActivityInstance(ctx, input)
}

func (m *Module) CreateTemplateBackedActivityInstanceIfAbsent(ctx context.Context, input ActivityInstanceInput) (ActivityInstance, bool, error) {
	input, ok := normalizeActivityInstance(input)
	if !ok || input.ActivityGroupID == nil {
		return ActivityInstance{}, false, m.reject("create_template_backed_activity_instance", ErrInvalidActivityInstance)
	}
	return m.engine.CreateTemplateBackedActivityInstanceIfAbsent(ctx, input)
}

func (m *Module) CreateIdempotentActivityInstance(ctx context.Context, input ActivityInstanceInput) (ActivityInstance, bool, error) {
	input, ok := normalizeActivityInstance(input)
	if !ok || input.IdempotencyKey == nil {
		return ActivityInstance{}, false, m.reject("create_idempotent_activity_instance", ErrInvalidActivityInstance)
	}
	return m.engine.CreateIdempotentActivityInstance(ctx, input)
}

func (m *Module) UpdateActivityInstance(ctx context.Context, id int64, input ActivityInstanceInput) (ActivityInstance, error) {
	input, ok := normalizeActivityInstance(input)
	if id <= 0 || !ok {
		return ActivityInstance{}, m.reject("update_activity_instance", ErrInvalidActivityInstance)
	}
	return m.engine.UpdateActivityInstance(ctx, id, input)
}

func (m *Module) PatchActivityInstance(ctx context.Context, id int64, input ActivityInstanceInput, columns []string) (int64, error) {
	if id <= 0 || len(columns) == 0 || !validActivityInstanceColumns(columns) {
		return 0, m.reject("patch_activity_instance", ErrInvalidActivityInstance)
	}
	return m.engine.PatchActivityInstance(ctx, id, input, columns)
}

func (m *Module) DeleteActivityInstance(ctx context.Context, id int64) error {
	if id <= 0 {
		return m.reject("delete_activity_instance", ErrInvalidActivityInstance)
	}
	return m.engine.DeleteActivityInstance(ctx, id)
}

func (m *Module) DeletePlannedActivityInstances(ctx context.Context, from string, to *string, groupID *int64, preserveDeviations bool) (int64, error) {
	if !validDate(from) || !validOptionalDate(to) || (groupID != nil && *groupID <= 0) {
		return 0, m.reject("delete_planned_activity_instances", ErrInvalidActivityInstance)
	}
	return m.engine.DeletePlannedActivityInstances(ctx, from, to, groupID, preserveDeviations)
}

func (m *Module) DeleteRemovedWeekendActivityInstances(ctx context.Context, groupID int64, weekdays []int) (int64, error) {
	if groupID <= 0 || invalidActivityInstanceWeekdays(weekdays) {
		return 0, m.reject("delete_removed_weekend_activity_instances", ErrInvalidActivityInstance)
	}
	return m.engine.DeleteRemovedWeekendActivityInstances(ctx, groupID, weekdays)
}

func (m *Module) PropagateActivityInstanceListKind(ctx context.Context, groupID int64, previousKind, newKind *string, after string) (int64, error) {
	if groupID <= 0 || !validDate(after) {
		return 0, m.reject("propagate_activity_instance_list_kind", ErrInvalidActivityInstance)
	}
	return m.engine.PropagateActivityInstanceListKind(ctx, groupID, previousKind, newKind, after)
}

func (m *Module) DeleteActivityInstancesBefore(ctx context.Context, before string) (int64, error) {
	if !validDate(before) {
		return 0, m.reject("delete_activity_instances_before", ErrInvalidActivityInstance)
	}
	return m.engine.DeleteActivityInstancesBefore(ctx, before)
}

func (m *Module) FindInstanceStaff(ctx context.Context, id int64) (InstanceStaff, error) {
	if id <= 0 {
		return InstanceStaff{}, m.reject("find_instance_staff", ErrInvalidInstanceStaffQuery)
	}
	return m.engine.FindInstanceStaff(ctx, id)
}

func (m *Module) ListInstanceStaff(ctx context.Context, filter InstanceStaffFilter) ([]InstanceStaff, error) {
	if !validInstanceStaffFilter(filter) {
		return nil, m.reject("list_instance_staff", ErrInvalidInstanceStaffQuery)
	}
	return m.engine.ListInstanceStaff(ctx, filter)
}

func (m *Module) CountNonAbsentInstanceStaff(ctx context.Context, instanceIDs []int64) (map[int64]int, error) {
	if hasInvalidID(instanceIDs) {
		return nil, m.reject("count_non_absent_instance_staff", ErrInvalidInstanceStaffQuery)
	}
	return m.engine.CountNonAbsentInstanceStaff(ctx, instanceIDs)
}

func (m *Module) CreateInstanceStaff(ctx context.Context, input InstanceStaffInput) (InstanceStaff, error) {
	if !validInstanceStaff(input) {
		return InstanceStaff{}, m.reject("create_instance_staff", ErrInvalidInstanceStaff)
	}
	return m.engine.CreateInstanceStaff(ctx, input)
}

func (m *Module) UpdateInstanceStaff(ctx context.Context, id int64, input InstanceStaffInput) (InstanceStaff, error) {
	if id <= 0 || !validInstanceStaff(input) {
		return InstanceStaff{}, m.reject("update_instance_staff", ErrInvalidInstanceStaff)
	}
	return m.engine.UpdateInstanceStaff(ctx, id, input)
}

func (m *Module) PatchInstanceStaff(ctx context.Context, id int64, input InstanceStaffInput, columns []string) (int64, error) {
	if id <= 0 || !validInstanceStaffColumns(columns) {
		return 0, m.reject("patch_instance_staff", ErrInvalidInstanceStaff)
	}
	return m.engine.PatchInstanceStaff(ctx, id, input, columns)
}

func (m *Module) DeleteInstanceStaff(ctx context.Context, id int64) error {
	if id <= 0 {
		return m.reject("delete_instance_staff", ErrInvalidInstanceStaff)
	}
	return m.engine.DeleteInstanceStaff(ctx, id)
}

func (m *Module) DeleteInstanceStaffByInstance(ctx context.Context, instanceID int64) error {
	if instanceID <= 0 {
		return m.reject("delete_instance_staff_by_instance", ErrInvalidInstanceStaff)
	}
	return m.engine.DeleteInstanceStaffByInstance(ctx, instanceID)
}

func (m *Module) DeleteUpcomingInstanceStaff(ctx context.Context, staffID int64, after string) (int64, error) {
	if staffID <= 0 || !validDate(after) {
		return 0, m.reject("delete_upcoming_instance_staff", ErrInvalidInstanceStaff)
	}
	return m.engine.DeleteUpcomingInstanceStaff(ctx, staffID, after)
}

func (m *Module) FindInstanceStudent(ctx context.Context, id int64) (InstanceStudent, error) {
	if id <= 0 {
		return InstanceStudent{}, m.reject("find_instance_student", ErrInvalidInstanceStudentQuery)
	}
	return m.engine.FindInstanceStudent(ctx, id)
}

func (m *Module) ListInstanceStudents(ctx context.Context, filter InstanceStudentFilter) ([]InstanceStudent, error) {
	if !validInstanceStudentFilter(filter) {
		return nil, m.reject("list_instance_students", ErrInvalidInstanceStudentQuery)
	}
	return m.engine.ListInstanceStudents(ctx, filter)
}

func (m *Module) CreateInstanceStudent(ctx context.Context, input InstanceStudentInput) (InstanceStudent, error) {
	if !validInstanceStudent(input) {
		return InstanceStudent{}, m.reject("create_instance_student", ErrInvalidInstanceStudent)
	}
	return m.engine.CreateInstanceStudent(ctx, input)
}

func (m *Module) UpdateInstanceStudent(ctx context.Context, id int64, input InstanceStudentInput) (InstanceStudent, error) {
	if id <= 0 || !validInstanceStudent(input) {
		return InstanceStudent{}, m.reject("update_instance_student", ErrInvalidInstanceStudent)
	}
	return m.engine.UpdateInstanceStudent(ctx, id, input)
}

func (m *Module) DeleteInstanceStudent(ctx context.Context, id int64) error {
	if id <= 0 {
		return m.reject("delete_instance_student", ErrInvalidInstanceStudent)
	}
	return m.engine.DeleteInstanceStudent(ctx, id)
}

func (m *Module) DeleteInstanceStudentsByInstance(ctx context.Context, instanceID int64) error {
	if instanceID <= 0 {
		return m.reject("delete_instance_students_by_instance", ErrInvalidInstanceStudent)
	}
	return m.engine.DeleteInstanceStudentsByInstance(ctx, instanceID)
}

func (m *Module) ListStudentInstanceRefsBefore(ctx context.Context, cutoff string) ([]StudentInstanceRef, error) {
	if !validDate(cutoff) {
		return nil, m.reject("list_student_instance_refs_before", ErrInvalidInstanceStudentQuery)
	}
	return m.engine.ListStudentInstanceRefsBefore(ctx, cutoff)
}

func (m *Module) ListPlannedStudentIDs(ctx context.Context, studentIDs []int64, date string) ([]int64, error) {
	if hasInvalidID(studentIDs) || !validDate(date) {
		return nil, m.reject("list_planned_student_ids", ErrInvalidInstanceStudentQuery)
	}
	return m.engine.ListPlannedStudentIDs(ctx, studentIDs, date)
}

func (m *Module) ListPlannedInstanceStudents(ctx context.Context, filter InstanceStudentFilter) ([]PlannedInstanceStudent, error) {
	if !validInstanceStudentFilter(filter) {
		return nil, m.reject("list_planned_instance_students", ErrInvalidInstanceStudentQuery)
	}
	return m.engine.ListPlannedInstanceStudents(ctx, filter)
}

func (m *Module) EnsureInstanceStudent(ctx context.Context, instanceID, studentID int64) (InstanceStudent, bool, error) {
	if instanceID <= 0 || studentID <= 0 {
		return InstanceStudent{}, false, m.reject("ensure_instance_student", ErrInvalidInstanceStudent)
	}
	return m.engine.EnsureInstanceStudent(ctx, instanceID, studentID)
}

func (m *Module) ArchivePlannedInstanceStudents(ctx context.Context, transitionID int64, entries []RosterArchiveEntry) (int, error) {
	if transitionID <= 0 || !validRosterArchiveEntries(entries) {
		return 0, m.reject("archive_planned_instance_students", ErrInvalidInstanceStudent)
	}
	return m.engine.ArchivePlannedInstanceStudents(ctx, transitionID, entries)
}

func (m *Module) RestoreArchivedInstanceStudents(ctx context.Context, transitionID int64, studentIDs []int64, from string) ([]RestoredInstanceStudent, error) {
	if transitionID <= 0 || hasInvalidID(studentIDs) || !validDate(from) {
		return nil, m.reject("restore_archived_instance_students", ErrInvalidInstanceStudent)
	}
	return m.engine.RestoreArchivedInstanceStudents(ctx, transitionID, studentIDs, from)
}

func (m *Module) LockInstanceStudentAssignments(ctx context.Context, instanceID int64) error {
	if instanceID <= 0 {
		return m.reject("lock_instance_student_assignments", ErrInvalidInstanceStudentQuery)
	}
	return m.engine.LockInstanceStudentAssignments(ctx, instanceID)
}
