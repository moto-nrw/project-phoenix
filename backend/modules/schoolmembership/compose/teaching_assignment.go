package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
)

func (e engine) ListClassAssignments(ctx context.Context, filter schoolmembership.ClassAssignmentFilter) ([]schoolmembership.ClassAssignment, error) {
	values, err := e.service.ListClassAssignments(ctx, domain.ClassAssignmentFilter{
		IDs: filter.IDs, StaffIDs: filter.StaffIDs, ClassKeys: filter.ClassKeys,
	})
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]schoolmembership.ClassAssignment, 0, len(values))
	for _, value := range values {
		result = append(result, classAssignmentToPublic(value))
	}
	return result, nil
}

func (e engine) CreateClassAssignment(ctx context.Context, input schoolmembership.CreateClassAssignment) (schoolmembership.ClassAssignment, error) {
	value, err := e.service.CreateClassAssignment(ctx, input.StaffID, input.SchoolClass)
	return classAssignmentToPublic(value), mapError(err)
}

func (e engine) UpdateClassAssignment(ctx context.Context, input schoolmembership.UpdateClassAssignment) (schoolmembership.ClassAssignment, error) {
	value, err := e.service.UpdateClassAssignment(ctx, input.ID, input.StaffID, input.SchoolClass)
	return classAssignmentToPublic(value), mapError(err)
}

func (e engine) DeleteClassAssignment(ctx context.Context, id int64) error {
	return mapError(e.service.DeleteClassAssignment(ctx, id))
}

func (e engine) DeleteClassAssignmentsByStaff(ctx context.Context, staffID int64) (int64, error) {
	rows, err := e.service.DeleteClassAssignmentsByStaff(ctx, staffID)
	return rows, mapError(err)
}

func (e engine) ListGroupAssignments(ctx context.Context, filter schoolmembership.GroupAssignmentFilter) ([]schoolmembership.GroupAssignment, error) {
	values, err := e.service.ListGroupAssignments(ctx, domain.GroupAssignmentFilter{
		IDs: filter.IDs, GroupIDs: filter.GroupIDs, TeacherIDs: filter.TeacherIDs, TeacherStaffIDs: filter.TeacherStaffIDs,
	})
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]schoolmembership.GroupAssignment, 0, len(values))
	for _, value := range values {
		result = append(result, groupAssignmentToPublic(value))
	}
	return result, nil
}

func (e engine) CreateGroupAssignment(ctx context.Context, input schoolmembership.CreateGroupAssignment) (schoolmembership.GroupAssignment, error) {
	value, err := e.service.CreateGroupAssignment(ctx, input.GroupID, input.TeacherID)
	return groupAssignmentToPublic(value), mapError(err)
}

func (e engine) UpdateGroupAssignment(ctx context.Context, input schoolmembership.UpdateGroupAssignment) (schoolmembership.GroupAssignment, error) {
	value, err := e.service.UpdateGroupAssignment(ctx, input.ID, input.GroupID, input.TeacherID)
	return groupAssignmentToPublic(value), mapError(err)
}

func (e engine) DeleteGroupAssignment(ctx context.Context, id int64) error {
	return mapError(e.service.DeleteGroupAssignment(ctx, id))
}

func (e engine) DeleteGroupAssignmentsByTeacher(ctx context.Context, teacherID int64) (int64, error) {
	rows, err := e.service.DeleteGroupAssignmentsByTeacher(ctx, teacherID)
	return rows, mapError(err)
}

func classAssignmentToPublic(value domain.ClassAssignment) schoolmembership.ClassAssignment {
	return schoolmembership.ClassAssignment{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		StaffID: value.StaffID, SchoolClass: value.SchoolClass,
	}
}

func groupAssignmentToPublic(value domain.GroupAssignment) schoolmembership.GroupAssignment {
	return schoolmembership.GroupAssignment{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		GroupID: value.GroupID, TeacherID: value.TeacherID,
	}
}
