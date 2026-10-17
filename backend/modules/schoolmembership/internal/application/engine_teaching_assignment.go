package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
)

func (e Engine) ListClassAssignments(ctx context.Context, filter schoolmembership.ClassAssignmentFilter) ([]schoolmembership.ClassAssignment, error) {
	values, err := e.service.ListClassAssignments(ctx, domain.ClassAssignmentFilter{
		IDs: filter.IDs, StaffIDs: filter.StaffIDs, ClassKeys: filter.ClassKeys,
	})
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]schoolmembership.ClassAssignment, 0, len(values))
	for _, value := range values {
		result = append(result, classAssignmentToPublic(value))
	}
	return result, nil
}

func (e Engine) CreateClassAssignment(ctx context.Context, input schoolmembership.CreateClassAssignment) (schoolmembership.ClassAssignment, error) {
	value, err := e.service.CreateClassAssignment(ctx, input.StaffID, input.SchoolClass)
	return classAssignmentToPublic(value), MapError(err)
}

func (e Engine) UpdateClassAssignment(ctx context.Context, input schoolmembership.UpdateClassAssignment) (schoolmembership.ClassAssignment, error) {
	value, err := e.service.UpdateClassAssignment(ctx, input.ID, input.StaffID, input.SchoolClass)
	return classAssignmentToPublic(value), MapError(err)
}

func (e Engine) DeleteClassAssignment(ctx context.Context, id int64) error {
	return MapError(e.service.DeleteClassAssignment(ctx, id))
}

func (e Engine) DeleteClassAssignmentsByStaff(ctx context.Context, staffID int64) (int64, error) {
	rows, err := e.service.DeleteClassAssignmentsByStaff(ctx, staffID)
	return rows, MapError(err)
}

func (e Engine) ListGroupAssignments(ctx context.Context, filter schoolmembership.GroupAssignmentFilter) ([]schoolmembership.GroupAssignment, error) {
	values, err := e.service.ListGroupAssignments(ctx, domain.GroupAssignmentFilter{
		IDs: filter.IDs, GroupIDs: filter.GroupIDs, TeacherIDs: filter.TeacherIDs, TeacherStaffIDs: filter.TeacherStaffIDs,
	})
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]schoolmembership.GroupAssignment, 0, len(values))
	for _, value := range values {
		result = append(result, groupAssignmentToPublic(value))
	}
	return result, nil
}

func (e Engine) CreateGroupAssignment(ctx context.Context, input schoolmembership.CreateGroupAssignment) (schoolmembership.GroupAssignment, error) {
	value, err := e.service.CreateGroupAssignment(ctx, input.GroupID, input.TeacherID)
	return groupAssignmentToPublic(value), MapError(err)
}

func (e Engine) UpdateGroupAssignment(ctx context.Context, input schoolmembership.UpdateGroupAssignment) (schoolmembership.GroupAssignment, error) {
	value, err := e.service.UpdateGroupAssignment(ctx, input.ID, input.GroupID, input.TeacherID)
	return groupAssignmentToPublic(value), MapError(err)
}

func (e Engine) DeleteGroupAssignment(ctx context.Context, id int64) error {
	return MapError(e.service.DeleteGroupAssignment(ctx, id))
}

func (e Engine) DeleteGroupAssignmentsByTeacher(ctx context.Context, teacherID int64) (int64, error) {
	rows, err := e.service.DeleteGroupAssignmentsByTeacher(ctx, teacherID)
	return rows, MapError(err)
}

func (e Engine) LockGroupAssignments(ctx context.Context) error {
	return MapError(e.service.LockGroupAssignments(ctx))
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
