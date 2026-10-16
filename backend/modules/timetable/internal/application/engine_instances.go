package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (e Engine) FindActivityInstance(ctx context.Context, id int64) (timetable.ActivityInstance, error) {
	value, err := e.service.FindActivityInstance(ctx, id)
	return activityInstanceToPublic(value), MapError(err)
}

func (e Engine) ListActivityInstances(ctx context.Context, filter timetable.ActivityInstanceFilter) ([]timetable.ActivityInstance, error) {
	values, err := e.service.ListActivityInstances(ctx, domain.ActivityInstanceFilter(filter))
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]timetable.ActivityInstance, 0, len(values))
	for _, value := range values {
		result = append(result, activityInstanceToPublic(value))
	}
	return result, nil
}

func (e Engine) MaxActivityInstanceID(ctx context.Context) (int64, error) {
	result, err := e.service.MaxActivityInstanceID(ctx)
	return result, MapError(err)
}

func (e Engine) CountActivityInstances(ctx context.Context, before *string) (int, error) {
	result, err := e.service.CountActivityInstances(ctx, before)
	return result, MapError(err)
}

func (e Engine) OldestActivityInstanceBefore(ctx context.Context, before *string) (*string, error) {
	result, err := e.service.OldestActivityInstanceBefore(ctx, before)
	return result, MapError(err)
}

func (e Engine) CreateActivityInstance(ctx context.Context, input timetable.ActivityInstanceInput) (timetable.ActivityInstance, error) {
	value, err := e.service.CreateActivityInstance(ctx, domain.ActivityInstanceFields(input))
	return activityInstanceToPublic(value), MapError(err)
}

func (e Engine) CreateTemplateBackedActivityInstanceIfAbsent(ctx context.Context, input timetable.ActivityInstanceInput) (timetable.ActivityInstance, bool, error) {
	value, inserted, err := e.service.CreateTemplateBackedActivityInstanceIfAbsent(ctx, domain.ActivityInstanceFields(input))
	return activityInstanceToPublic(value), inserted, MapError(err)
}

func (e Engine) CreateIdempotentActivityInstance(ctx context.Context, input timetable.ActivityInstanceInput) (timetable.ActivityInstance, bool, error) {
	value, inserted, err := e.service.CreateIdempotentActivityInstance(ctx, domain.ActivityInstanceFields(input))
	return activityInstanceToPublic(value), inserted, MapError(err)
}

func (e Engine) UpdateActivityInstance(ctx context.Context, id int64, input timetable.ActivityInstanceInput) (timetable.ActivityInstance, error) {
	value, err := e.service.UpdateActivityInstance(ctx, id, domain.ActivityInstanceFields(input))
	return activityInstanceToPublic(value), MapError(err)
}

func (e Engine) PatchActivityInstance(ctx context.Context, id int64, input timetable.ActivityInstanceInput, columns []string) (int64, error) {
	rows, err := e.service.PatchActivityInstance(ctx, id, domain.ActivityInstanceFields(input), columns)
	return rows, MapError(err)
}

func (e Engine) DeleteActivityInstance(ctx context.Context, id int64) error {
	return MapError(e.service.DeleteActivityInstance(ctx, id))
}

func (e Engine) DeletePlannedActivityInstances(ctx context.Context, from string, to *string, groupID *int64, preserveDeviations bool) (int64, error) {
	rows, err := e.service.DeletePlannedActivityInstances(ctx, from, to, groupID, preserveDeviations)
	return rows, MapError(err)
}

func (e Engine) DeleteRemovedWeekendActivityInstances(ctx context.Context, groupID int64, weekdays []int) (int64, error) {
	rows, err := e.service.DeleteRemovedWeekendActivityInstances(ctx, groupID, weekdays)
	return rows, MapError(err)
}

func (e Engine) PropagateActivityInstanceListKind(ctx context.Context, groupID int64, previousKind, newKind *string, after string) (int64, error) {
	rows, err := e.service.PropagateActivityInstanceListKind(ctx, groupID, previousKind, newKind, after)
	return rows, MapError(err)
}

func (e Engine) DeleteActivityInstancesBefore(ctx context.Context, before string) (int64, error) {
	rows, err := e.service.DeleteActivityInstancesBefore(ctx, before)
	return rows, MapError(err)
}

func (e Engine) FindInstanceStaff(ctx context.Context, id int64) (timetable.InstanceStaff, error) {
	value, err := e.service.FindInstanceStaff(ctx, id)
	return instanceStaffToPublic(value), MapError(err)
}

func (e Engine) ListInstanceStaff(ctx context.Context, filter timetable.InstanceStaffFilter) ([]timetable.InstanceStaff, error) {
	values, err := e.service.ListInstanceStaff(ctx, domain.InstanceStaffFilter(filter))
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]timetable.InstanceStaff, 0, len(values))
	for _, value := range values {
		result = append(result, instanceStaffToPublic(value))
	}
	return result, nil
}

func (e Engine) CountNonAbsentInstanceStaff(ctx context.Context, instanceIDs []int64) (map[int64]int, error) {
	result, err := e.service.CountNonAbsentInstanceStaff(ctx, instanceIDs)
	return result, MapError(err)
}

func (e Engine) CreateInstanceStaff(ctx context.Context, input timetable.InstanceStaffInput) (timetable.InstanceStaff, error) {
	value, err := e.service.CreateInstanceStaff(ctx, domain.InstanceStaffFields(input))
	return instanceStaffToPublic(value), MapError(err)
}

func (e Engine) UpdateInstanceStaff(ctx context.Context, id int64, input timetable.InstanceStaffInput) (timetable.InstanceStaff, error) {
	value, err := e.service.UpdateInstanceStaff(ctx, id, domain.InstanceStaffFields(input))
	return instanceStaffToPublic(value), MapError(err)
}

func (e Engine) PatchInstanceStaff(ctx context.Context, id int64, input timetable.InstanceStaffInput, columns []string) (int64, error) {
	rows, err := e.service.PatchInstanceStaff(ctx, id, domain.InstanceStaffFields(input), columns)
	return rows, MapError(err)
}

func (e Engine) DeleteInstanceStaff(ctx context.Context, id int64) error {
	return MapError(e.service.DeleteInstanceStaff(ctx, id))
}

func (e Engine) DeleteInstanceStaffByInstance(ctx context.Context, instanceID int64) error {
	return MapError(e.service.DeleteInstanceStaffByInstance(ctx, instanceID))
}

func (e Engine) DeleteUpcomingInstanceStaff(ctx context.Context, staffID int64, after string) (int64, error) {
	rows, err := e.service.DeleteUpcomingInstanceStaff(ctx, staffID, after)
	return rows, MapError(err)
}

func (e Engine) FindInstanceStudent(ctx context.Context, id int64) (timetable.InstanceStudent, error) {
	value, err := e.service.FindInstanceStudent(ctx, id)
	return instanceStudentToPublic(value), MapError(err)
}

func (e Engine) ListInstanceStudents(ctx context.Context, filter timetable.InstanceStudentFilter) ([]timetable.InstanceStudent, error) {
	values, err := e.service.ListInstanceStudents(ctx, domain.InstanceStudentFilter(filter))
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]timetable.InstanceStudent, 0, len(values))
	for _, value := range values {
		result = append(result, instanceStudentToPublic(value))
	}
	return result, nil
}

func (e Engine) CreateInstanceStudent(ctx context.Context, input timetable.InstanceStudentInput) (timetable.InstanceStudent, error) {
	value, err := e.service.CreateInstanceStudent(ctx, domain.InstanceStudentFields(input))
	return instanceStudentToPublic(value), MapError(err)
}

func (e Engine) UpdateInstanceStudent(ctx context.Context, id int64, input timetable.InstanceStudentInput) (timetable.InstanceStudent, error) {
	value, err := e.service.UpdateInstanceStudent(ctx, id, domain.InstanceStudentFields(input))
	return instanceStudentToPublic(value), MapError(err)
}

func (e Engine) DeleteInstanceStudent(ctx context.Context, id int64) error {
	return MapError(e.service.DeleteInstanceStudent(ctx, id))
}

func (e Engine) DeleteInstanceStudentsByInstance(ctx context.Context, instanceID int64) error {
	return MapError(e.service.DeleteInstanceStudentsByInstance(ctx, instanceID))
}

func (e Engine) ListStudentInstanceRefsBefore(ctx context.Context, cutoff string) ([]timetable.StudentInstanceRef, error) {
	values, err := e.service.ListStudentInstanceRefsBefore(ctx, cutoff)
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]timetable.StudentInstanceRef, 0, len(values))
	for _, value := range values {
		result = append(result, timetable.StudentInstanceRef(value))
	}
	return result, nil
}

func (e Engine) ListPlannedStudentIDs(ctx context.Context, studentIDs []int64, date string) ([]int64, error) {
	result, err := e.service.ListPlannedStudentIDs(ctx, studentIDs, date)
	return result, MapError(err)
}
