package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/adapters/postgres/groups"
	supervision "github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/application/groups"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/domain"
)

type DirectoryRoom = groups.DirectoryRoom
type TeacherGroupID = domain.TeacherGroupID
type GroupRoomLookup = groups.RoomDirectory

type GroupRepositoryDependencies struct {
	Rooms               func() GroupRoomLookup
	TeachingAssignments func(context.Context, []int64, []int64) ([]TeacherGroupID, error)
	TeacherStaffIDs     func(context.Context, []int64) (map[int64]int64, error)
	LiveStaffIDs        func(context.Context, []int64) (map[int64]bool, error)
	GroupSubstitutes    func(context.Context, []int64, string) ([]domain.StaffGroupID, error)
}

// NewGroupRepository serves retained consumer-owned record ports from the
// School Structure owner. No implementation handle escapes.
func NewGroupRepository(runtime groups.Runtime, deps GroupRepositoryDependencies) *GroupRepository {
	store := groups.NewGroupRepository(runtime, deps.Rooms, deps.TeachingAssignments, supervision.SupervisingStaff(supervision.SupervisionSources{TeacherStaffIDs: deps.TeacherStaffIDs, LiveStaffIDs: deps.LiveStaffIDs, GroupSubstitutes: deps.GroupSubstitutes}))
	return &GroupRepository{store: store}
}

type GroupRepository struct{ store *groups.GroupRepository }
