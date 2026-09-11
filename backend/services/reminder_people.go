package services

import (
	"context"

	active "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/ports"
)

type reminderStudentReader struct {
	source interface {
		FindReadScopeByIDs(context.Context, []int64) (map[int64]*users.Student, error)
	}
}

func (r reminderStudentReader) FindReadScopeByIDs(ctx context.Context, ids []int64) (map[int64]*ports.Student, error) {
	values, err := r.source.FindReadScopeByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]*ports.Student, len(values))
	for id, value := range values {
		if value != nil {
			result[id] = &ports.Student{ID: value.ID, PersonID: value.PersonID, SchoolClass: value.SchoolClass, GroupID: value.GroupID}
		}
	}
	return result, nil
}

type reminderPersonReader struct {
	source interface {
		FindByIDs(context.Context, []int64) (map[int64]*users.Person, error)
	}
}

func (r reminderPersonReader) FindByIDs(ctx context.Context, ids []int64) (map[int64]*ports.Person, error) {
	values, err := r.source.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]*ports.Person, len(values))
	for id, value := range values {
		if value != nil {
			result[id] = &ports.Person{ID: value.ID, Name: value.GetFullName()}
		}
	}
	return result, nil
}

type reminderSupervisionReader struct {
	source interface {
		GetStaffActiveSupervisions(context.Context, int64) ([]*active.GroupSupervisor, error)
		GetActiveGroupsByIDs(context.Context, []int64) (map[int64]*active.Group, error)
	}
}

func (r reminderSupervisionReader) GetStaffActiveSupervisions(ctx context.Context, id int64) ([]*ports.GroupSupervisor, error) {
	values, err := r.source.GetStaffActiveSupervisions(ctx, id)
	if err != nil {
		return nil, err
	}
	result := make([]*ports.GroupSupervisor, 0, len(values))
	for _, value := range values {
		if value != nil {
			result = append(result, &ports.GroupSupervisor{StaffID: value.StaffID, GroupID: value.GroupID})
		}
	}
	return result, nil
}

func (r reminderSupervisionReader) GetActiveGroupsByIDs(ctx context.Context, ids []int64) (map[int64]*ports.Group, error) {
	values, err := r.source.GetActiveGroupsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]*ports.Group, len(values))
	for id, value := range values {
		if value != nil {
			result[id] = &ports.Group{RoomID: value.RoomID}
		}
	}
	return result, nil
}

type reminderBulkSupervisionReader struct {
	source interface {
		ListActiveSupervisedRooms(context.Context) ([]active.StaffRoomSupervision, error)
	}
}

func (r reminderBulkSupervisionReader) ListActiveSupervisedRooms(ctx context.Context) ([]ports.StaffRoomSupervision, error) {
	values, err := r.source.ListActiveSupervisedRooms(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]ports.StaffRoomSupervision, 0, len(values))
	for _, value := range values {
		result = append(result, ports.StaffRoomSupervision{StaffID: value.StaffID, RoomID: value.RoomID})
	}
	return result, nil
}
