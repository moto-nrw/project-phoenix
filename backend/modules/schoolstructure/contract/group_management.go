package contract

import "context"

// GroupManagement composes independent structure capabilities.
type GroupManagement interface {
	GroupQuery
	GroupCommand
	GroupTeaching
	StaffSchoolClasses
}

// GroupRoomsQuery supplies room-enriched groups to live projections.
type GroupRoomsQuery interface {
	GetGroupsWithRoomsByIDs(context.Context, []int64) (map[int64]*Group, error)
}

// GroupOverviewQuery supplies group candidates and their rooms.
type GroupOverviewQuery interface {
	GroupRoomsQuery
	ListGroups(context.Context, *GroupListQuery) ([]*Group, error)
}

// GroupQuery provides group detail and overview reads.
type GroupQuery interface {
	GroupOverviewQuery
	GetGroup(context.Context, int64) (*Group, error)
	GetGroupsByIDs(context.Context, []int64) (map[int64]*Group, error)
	CountGroups(context.Context, *GroupListQuery) (int, error)
	FindGroupWithRoom(context.Context, int64) (*Group, error)
}

type GroupCommand interface {
	CreateGroup(context.Context, *Group) error
	UpdateGroup(context.Context, *Group) error
	DeleteGroup(context.Context, int64) error
}

// GroupTeaching manages teaching assignments and their display projections.
type GroupTeaching interface {
	RemoveTeacherFromGroup(context.Context, int64, int64) error
	UpdateGroupTeachers(context.Context, int64, []int64) error
	GetGroupTeachers(context.Context, int64) ([]*Teacher, error)
	GetTeachersForGroups(context.Context, []int64) (map[int64][]*Teacher, error)
	GetTeacherGroups(context.Context, int64) ([]*Group, error)
}

// StaffSchoolClasses manages free-text classes matched by normalized name.
// The last argument of SetStaffSchoolClasses is the authenticated audit actor.
type StaffSchoolClasses interface {
	GetStaffSchoolClasses(context.Context, int64) ([]string, error)
	SetStaffSchoolClasses(context.Context, int64, []string, int64) error
}
