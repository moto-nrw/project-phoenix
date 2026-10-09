package schoolstructurehttp

import (
	"context"

	education "github.com/moto-nrw/project-phoenix/modules/schoolstructure/contract"
)

// GroupService is the group read port owned by this consumer.
type GroupService interface {
	GetGroup(ctx context.Context, id int64) (*education.Group, error)
	GetGroupsByIDs(ctx context.Context, ids []int64) (map[int64]*education.Group, error)
	CreateGroup(ctx context.Context, group *education.Group) error
	UpdateGroup(ctx context.Context, group *education.Group) error
	DeleteGroup(ctx context.Context, id int64) error
	ListGroups(ctx context.Context, query *education.GroupListQuery) ([]*education.Group, error)
	CountGroups(ctx context.Context, query *education.GroupListQuery) (int, error)
	FindGroupWithRoom(ctx context.Context, groupID int64) (*education.Group, error)
	GetGroupsWithRoomsByIDs(ctx context.Context, ids []int64) (map[int64]*education.Group, error)
	RemoveTeacherFromGroup(ctx context.Context, groupID, teacherID int64) error
	UpdateGroupTeachers(ctx context.Context, groupID int64, teacherIDs []int64) error
	GetGroupTeachers(ctx context.Context, groupID int64) ([]*education.Teacher, error)
	GetTeachersForGroups(ctx context.Context, groupIDs []int64) (map[int64][]*education.Teacher, error)
	GetTeacherGroups(ctx context.Context, teacherID int64) ([]*education.Group, error)
	GetStaffSchoolClasses(ctx context.Context, staffID int64) ([]string, error)
	SetStaffSchoolClasses(ctx context.Context, staffID int64, classes []string, changedBy int64) error
}
