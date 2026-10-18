package api

import (
	"context"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/ports"
)

type ReminderStudentReader struct {
	Source interface {
		FindReadScopeByIDs(context.Context, []int64) (map[int64]*peopleValues.StudentState, error)
	}
}

func (r ReminderStudentReader) FindReadScopeByIDs(ctx context.Context, ids []int64) (map[int64]*ports.Student, error) {
	values, err := r.Source.FindReadScopeByIDs(ctx, ids)
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

type ReminderPersonReader struct {
	Source interface {
		FindByIDs(context.Context, []int64) (map[int64]*peopleValues.DirectoryPersonSnapshot, error)
	}
}

func (r ReminderPersonReader) FindByIDs(ctx context.Context, ids []int64) (map[int64]*ports.Person, error) {
	values, err := r.Source.FindByIDs(ctx, ids)
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

type ReminderSupervisionReader struct {
	Presence interface {
		QueryGroupSupervisions(context.Context, studentpresence.GroupSupervisionFilter) ([]studentpresence.GroupSupervision, error)
	}
	Source interface {
		GetActiveGroupsByIDs(context.Context, []int64) (map[int64]*studentpresence.SessionDetail, error)
	}
}

func (r ReminderSupervisionReader) GetStaffActiveSupervisions(ctx context.Context, id int64) ([]*ports.GroupSupervisor, error) {
	day := timezone.TodayDate().String()
	values, err := r.Presence.QueryGroupSupervisions(ctx, studentpresence.GroupSupervisionFilter{StaffID: &id, ActiveOn: &day})
	if err != nil {
		return nil, &studentpresence.OperationError{Op: "GetStaffActiveSupervisions", Err: studentpresence.ErrDatabaseOperation}
	}
	// ActiveOn already keeps the supervisions running today: started on or
	// before it and ending after it.
	result := make([]*ports.GroupSupervisor, 0, len(values))
	for _, value := range values {
		result = append(result, &ports.GroupSupervisor{StaffID: value.StaffID, GroupID: value.GroupID})
	}
	return result, nil
}

func (r ReminderSupervisionReader) GetActiveGroupsByIDs(ctx context.Context, ids []int64) (map[int64]*ports.Group, error) {
	values, err := r.Source.GetActiveGroupsByIDs(ctx, ids)
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

type ReminderBulkSupervisionReader struct {
	Source interface {
		ListActiveSupervisedRooms(context.Context) ([]studentpresence.StaffRoomSupervision, error)
	}
}

func (r ReminderBulkSupervisionReader) ListActiveSupervisedRooms(ctx context.Context) ([]ports.StaffRoomSupervision, error) {
	values, err := r.Source.ListActiveSupervisedRooms(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]ports.StaffRoomSupervision, 0, len(values))
	for _, value := range values {
		result = append(result, ports.StaffRoomSupervision{StaffID: value.StaffID, RoomID: value.RoomID})
	}
	return result, nil
}
