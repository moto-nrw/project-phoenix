package compose

import (
	"context"

	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/adapters/postgres/groups"
)

type DirectoryRoom = groups.DirectoryRoom
type GroupMembershipPairs = groups.GroupMembershipPairs
type TeacherGroupID = groups.TeacherGroupID
type GroupRoomLookup = groups.RoomDirectory

type GroupRepositoryDependencies struct {
	Rooms               func() GroupRoomLookup
	TeachingAssignments func(context.Context, []int64, []int64) ([]TeacherGroupID, error)
	SupervisingStaff    func(context.Context, GroupMembershipPairs) ([]educationModels.StaffGroupID, error)
}

// NewGroupRepository serves retained consumer-owned record ports from the
// School Structure owner. No implementation handle escapes.
func NewGroupRepository(runtime groups.Runtime, deps GroupRepositoryDependencies) *GroupRepository {
	store := groups.NewGroupRepository(runtime, deps.Rooms, deps.TeachingAssignments, deps.SupervisingStaff)
	return &GroupRepository{store: store}
}

type GroupRepository struct{ store *groups.GroupRepository }
