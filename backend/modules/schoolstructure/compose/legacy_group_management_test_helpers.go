package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/application/groups"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/domain"
)

type legacyGroupManagement struct {
	capability schoolstructure.GroupManagement
}

func LegacyGroupManagement(capability schoolstructure.GroupManagement) Service {
	return &legacyGroupManagement{capability: capability}
}

func (s *legacyGroupManagement) GetGroup(ctx context.Context, id int64) (*domain.Group, error) {
	value, err := s.capability.GetGroup(ctx, id)
	return legacyGroup(value), legacyGroupError(err)
}
func (s *legacyGroupManagement) FindGroupWithRoom(ctx context.Context, id int64) (*domain.Group, error) {
	value, err := s.capability.FindGroupWithRoom(ctx, id)
	return legacyGroup(value), legacyGroupError(err)
}
func (s *legacyGroupManagement) GetGroupsByIDs(ctx context.Context, ids []int64) (map[int64]*domain.Group, error) {
	values, err := s.capability.GetGroupsByIDs(ctx, ids)
	if values == nil {
		return nil, legacyGroupError(err)
	}
	result := make(map[int64]*domain.Group, len(values))
	for id, value := range values {
		result[id] = legacyGroup(value)
	}
	return result, legacyGroupError(err)
}
func (s *legacyGroupManagement) GetGroupsWithRoomsByIDs(ctx context.Context, ids []int64) (map[int64]*domain.Group, error) {
	values, err := s.capability.GetGroupsWithRoomsByIDs(ctx, ids)
	if values == nil {
		return nil, legacyGroupError(err)
	}
	result := make(map[int64]*domain.Group, len(values))
	for id, value := range values {
		result[id] = legacyGroup(value)
	}
	return result, legacyGroupError(err)
}
func (s *legacyGroupManagement) CreateGroup(ctx context.Context, row *domain.Group) error {
	value := publicGroup(row)
	err := s.capability.CreateGroup(ctx, value)
	if row != nil && value != nil {
		*row = *legacyGroup(value)
	}
	return legacyGroupError(err)
}
func (s *legacyGroupManagement) UpdateGroup(ctx context.Context, row *domain.Group) error {
	value := publicGroup(row)
	err := s.capability.UpdateGroup(ctx, value)
	if row != nil && value != nil {
		*row = *legacyGroup(value)
	}
	return legacyGroupError(err)
}
func (s *legacyGroupManagement) DeleteGroup(ctx context.Context, id int64) error {
	return legacyGroupError(s.capability.DeleteGroup(ctx, id))
}
func (s *legacyGroupManagement) ListGroups(ctx context.Context, query *domain.GroupListQuery) ([]*domain.Group, error) {
	values, err := s.capability.ListGroups(ctx, legacyGroupQuery(query))
	if values == nil {
		return nil, legacyGroupError(err)
	}
	result := make([]*domain.Group, len(values))
	for i, value := range values {
		result[i] = legacyGroup(value)
	}
	return result, legacyGroupError(err)
}
func (s *legacyGroupManagement) CountGroups(ctx context.Context, query *domain.GroupListQuery) (int, error) {
	count, err := s.capability.CountGroups(ctx, legacyGroupQuery(query))
	return count, legacyGroupError(err)
}
func (s *legacyGroupManagement) GetTeacherGroups(ctx context.Context, id int64) ([]*domain.Group, error) {
	values, err := s.capability.GetTeacherGroups(ctx, id)
	if values == nil {
		return nil, legacyGroupError(err)
	}
	result := make([]*domain.Group, len(values))
	for i, value := range values {
		result[i] = legacyGroup(value)
	}
	return result, legacyGroupError(err)
}
func (s *legacyGroupManagement) GetGroupTeachers(ctx context.Context, id int64) ([]*groups.Teacher, error) {
	values, err := s.capability.GetGroupTeachers(ctx, id)
	if values == nil {
		return nil, legacyGroupError(err)
	}
	result := make([]*groups.Teacher, len(values))
	for i, value := range values {
		result[i] = legacyTeacher(value)
	}
	return result, legacyGroupError(err)
}
func (s *legacyGroupManagement) GetTeachersForGroups(ctx context.Context, ids []int64) (map[int64][]*groups.Teacher, error) {
	values, err := s.capability.GetTeachersForGroups(ctx, ids)
	if values == nil {
		return nil, legacyGroupError(err)
	}
	result := make(map[int64][]*groups.Teacher, len(values))
	for id, teachers := range values {
		if teachers == nil {
			result[id] = nil
			continue
		}
		list := make([]*groups.Teacher, len(teachers))
		for i, value := range teachers {
			list[i] = legacyTeacher(value)
		}
		result[id] = list
	}
	return result, legacyGroupError(err)
}
func (s *legacyGroupManagement) RemoveTeacherFromGroup(ctx context.Context, id, teacherID int64) error {
	return legacyGroupError(s.capability.RemoveTeacherFromGroup(ctx, id, teacherID))
}
func (s *legacyGroupManagement) UpdateGroupTeachers(ctx context.Context, id int64, ids []int64) error {
	return legacyGroupError(s.capability.UpdateGroupTeachers(ctx, id, ids))
}
func (s *legacyGroupManagement) GetStaffSchoolClasses(ctx context.Context, id int64) ([]string, error) {
	values, err := s.capability.GetStaffSchoolClasses(ctx, id)
	return values, legacyGroupError(err)
}
func (s *legacyGroupManagement) SetStaffSchoolClasses(ctx context.Context, id int64, classes []string, changedBy int64) error {
	return legacyGroupError(s.capability.SetStaffSchoolClasses(ctx, id, classes, changedBy))
}

func NewService(groupRepo GroupRecords, groupTeachers GroupTeacherStore, classTeachers ClassTeacherStore, rooms RoomDirectory, teachers TeacherDirectory, staff StaffDirectory, students StudentCounter, handovers HandoverReader, runtime Runtime, options ...GroupServiceOptions) Service {
	return LegacyGroupManagement(NewGroupManagement(groupRepo, groupTeachers, classTeachers, rooms, teachers, staff, students, handovers, runtime, options...))
}

type Service interface {
	// Group operations
	GetGroup(ctx context.Context, id int64) (*domain.Group, error)
	GetGroupsByIDs(ctx context.Context, ids []int64) (map[int64]*domain.Group, error)
	CreateGroup(ctx context.Context, group *domain.Group) error
	UpdateGroup(ctx context.Context, group *domain.Group) error
	DeleteGroup(ctx context.Context, id int64) error
	ListGroups(ctx context.Context, query *domain.GroupListQuery) ([]*domain.Group, error)
	CountGroups(ctx context.Context, query *domain.GroupListQuery) (int, error)
	FindGroupWithRoom(ctx context.Context, groupID int64) (*domain.Group, error)
	// GetGroupsWithRoomsByIDs bulk-loads groups with their room relation in
	// one query — use instead of per-group FindGroupWithRoom loops.
	GetGroupsWithRoomsByIDs(ctx context.Context, ids []int64) (map[int64]*domain.Group, error)

	// Group-Teacher operations
	RemoveTeacherFromGroup(ctx context.Context, groupID, teacherID int64) error
	UpdateGroupTeachers(ctx context.Context, groupID int64, teacherIDs []int64) error
	GetGroupTeachers(ctx context.Context, groupID int64) ([]*Teacher, error)
	GetTeachersForGroups(ctx context.Context, groupIDs []int64) (map[int64][]*Teacher, error)
	GetTeacherGroups(ctx context.Context, teacherID int64) ([]*domain.Group, error)

	// Class-Teacher operations (#1772): staff-to-school-class assignments
	// that scope the Lehrkraft day view. Classes are free-text strings
	// matched via schoolclass.Normalize; there is no class entity. changedBy
	// is the authenticated account ID for the audit trail.
	GetStaffSchoolClasses(ctx context.Context, staffID int64) ([]string, error)
	SetStaffSchoolClasses(ctx context.Context, staffID int64, classes []string, changedBy int64) error
}
