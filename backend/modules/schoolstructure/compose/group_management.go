package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/application/groups"
)

// NewGroupManagement builds the native facade over the existing group application.
func NewGroupManagement(groupRepo GroupRecords, groupTeachers GroupTeacherStore, classTeachers ClassTeacherStore, rooms RoomDirectory, teachers TeacherDirectory, staff StaffDirectory, students StudentCounter, handovers HandoverReader, runtime Runtime, options ...GroupServiceOptions) schoolstructure.GroupManagement {
	var option GroupServiceOptions
	if len(options) > 0 {
		option = options[0]
	}
	return &groupManagement{reads: groupReads{groupRepo: groupRepo}, service: groups.NewService(groupRepo, groupTeachers, classTeachers, rooms, teachers, staff, students, handovers, runtime, option.Broadcaster, option.Audit)}
}

type groupManagement struct {
	service groups.Service
	reads   groupReads
}

func (s *groupManagement) GetGroup(ctx context.Context, id int64) (*schoolstructure.Group, error) {
	row, err := s.reads.GetGroup(ctx, id)
	return publicGroup(row), publicGroupError(err)
}
func (s *groupManagement) FindGroupWithRoom(ctx context.Context, id int64) (*schoolstructure.Group, error) {
	row, err := s.reads.FindGroupWithRoom(ctx, id)
	return publicGroup(row), publicGroupError(err)
}
func (s *groupManagement) GetGroupsByIDs(ctx context.Context, ids []int64) (map[int64]*schoolstructure.Group, error) {
	rows, err := s.reads.GetGroupsByIDs(ctx, ids)
	if rows == nil {
		return nil, publicGroupError(err)
	}
	result := make(map[int64]*schoolstructure.Group, len(rows))
	for id, row := range rows {
		result[id] = publicGroup(row)
	}
	return result, publicGroupError(err)
}
func (s *groupManagement) GetGroupsWithRoomsByIDs(ctx context.Context, ids []int64) (map[int64]*schoolstructure.Group, error) {
	rows, err := s.reads.GetGroupsWithRoomsByIDs(ctx, ids)
	if rows == nil {
		return nil, publicGroupError(err)
	}
	result := make(map[int64]*schoolstructure.Group, len(rows))
	for id, row := range rows {
		result[id] = publicGroup(row)
	}
	return result, publicGroupError(err)
}
func (s *groupManagement) CreateGroup(ctx context.Context, value *schoolstructure.Group) error {
	row := legacyGroup(value)
	err := s.service.CreateGroup(ctx, row)
	if value != nil && row != nil {
		*value = *publicGroup(row)
	}
	return publicGroupError(err)
}
func (s *groupManagement) UpdateGroup(ctx context.Context, value *schoolstructure.Group) error {
	row := legacyGroup(value)
	err := s.service.UpdateGroup(ctx, row)
	if value != nil && row != nil {
		*value = *publicGroup(row)
	}
	return publicGroupError(err)
}
func (s *groupManagement) DeleteGroup(ctx context.Context, id int64) error {
	return publicGroupError(s.service.DeleteGroup(ctx, id))
}
func (s *groupManagement) ListGroups(ctx context.Context, query *schoolstructure.GroupListQuery) ([]*schoolstructure.Group, error) {
	rows, err := s.reads.ListGroups(ctx, publicGroupQuery(query))
	if rows == nil {
		return nil, publicGroupError(err)
	}
	result := make([]*schoolstructure.Group, len(rows))
	for i, row := range rows {
		result[i] = publicGroup(row)
	}
	return result, publicGroupError(err)
}
func (s *groupManagement) CountGroups(ctx context.Context, query *schoolstructure.GroupListQuery) (int, error) {
	count, err := s.reads.CountGroups(ctx, publicGroupQuery(query))
	return count, publicGroupError(err)
}
func (s *groupManagement) GetTeacherGroups(ctx context.Context, id int64) ([]*schoolstructure.Group, error) {
	rows, err := s.service.GetTeacherGroups(ctx, id)
	if rows == nil {
		return nil, publicGroupError(err)
	}
	result := make([]*schoolstructure.Group, len(rows))
	for i, row := range rows {
		result[i] = publicGroup(row)
	}
	return result, publicGroupError(err)
}
func (s *groupManagement) GetGroupTeachers(ctx context.Context, id int64) ([]*schoolstructure.Teacher, error) {
	rows, err := s.service.GetGroupTeachers(ctx, id)
	if rows == nil {
		return nil, publicGroupError(err)
	}
	result := make([]*schoolstructure.Teacher, len(rows))
	for i, row := range rows {
		result[i] = publicTeacher(row)
	}
	return result, publicGroupError(err)
}
func (s *groupManagement) GetTeachersForGroups(ctx context.Context, ids []int64) (map[int64][]*schoolstructure.Teacher, error) {
	rows, err := s.service.GetTeachersForGroups(ctx, ids)
	if rows == nil {
		return nil, publicGroupError(err)
	}
	result := make(map[int64][]*schoolstructure.Teacher, len(rows))
	for id, teachers := range rows {
		if teachers == nil {
			result[id] = nil
			continue
		}
		list := make([]*schoolstructure.Teacher, len(teachers))
		for i, row := range teachers {
			list[i] = publicTeacher(row)
		}
		result[id] = list
	}
	return result, publicGroupError(err)
}
func (s *groupManagement) RemoveTeacherFromGroup(ctx context.Context, groupID, teacherID int64) error {
	return publicGroupError(s.service.RemoveTeacherFromGroup(ctx, groupID, teacherID))
}
func (s *groupManagement) UpdateGroupTeachers(ctx context.Context, id int64, ids []int64) error {
	return publicGroupError(s.service.UpdateGroupTeachers(ctx, id, ids))
}
func (s *groupManagement) GetStaffSchoolClasses(ctx context.Context, id int64) ([]string, error) {
	values, err := s.service.GetStaffSchoolClasses(ctx, id)
	return values, publicGroupError(err)
}
func (s *groupManagement) SetStaffSchoolClasses(ctx context.Context, id int64, classes []string, changedBy int64) error {
	return publicGroupError(s.service.SetStaffSchoolClasses(ctx, id, classes, changedBy))
}

type GroupServiceOptions struct {
	Broadcaster groups.GroupPublisher
	Audit       ClassAssignmentAudit
}
