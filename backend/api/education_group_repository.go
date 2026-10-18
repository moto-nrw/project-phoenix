package api

import (
	"context"
	"fmt"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"

	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/workforce"

	educationRepo "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// NewGroupRepository is the retained group store of the School Structure
// owner (#2742) with what other owners hold bound: the rooms through
// Facilities, the teaching assignments through School Membership and the
// substitutions through Workforce.
func NewGroupRepository(runtime educationRepo.LegacyRepositoryRuntime, rooms facilitiesModule.Query, membership schoolmembership.Capability, workTime workforce.Capability) *EducationGroupStore {
	return NewEducationGroupStore(runtime, educationRepo.GroupRepositoryDependencies{
		Rooms:               func() educationRepo.GroupRoomLookup { return educationRoomDirectory{rooms: rooms} },
		TeachingAssignments: groupTeachingAssignments(membership),
		TeacherStaffIDs:     supervisionTeacherStaff(membership),
		LiveStaffIDs:        supervisionLiveStaff(membership),
		GroupSubstitutes:    supervisionSubstitutes(workTime),
	})
}

// NewEducationGroupStore is the retained group store on the tenant
// runtime of the School Structure owner, with only the given dependencies
// bound.
func NewEducationGroupStore(runtime educationRepo.LegacyRepositoryRuntime, options ...educationRepo.GroupRepositoryDependencies) *EducationGroupStore {
	var deps educationRepo.GroupRepositoryDependencies
	if len(options) > 0 {
		deps = options[0]
	}
	if deps.Rooms == nil {
		// No room owner bound: the room reads report the missing lookup.
		deps.Rooms = func() educationRepo.GroupRoomLookup { return nil }
	}
	return &EducationGroupStore{GroupRepository: educationRepo.NewGroupRepository(runtime, deps)}
}

type EducationGroupStore struct {
	*educationRepo.GroupRepository
}

func groupTeachingAssignments(membership groupAssignmentLister) func(context.Context, []int64, []int64) ([]educationRepo.TeacherGroupID, error) {
	return func(ctx context.Context, groupIDs, teacherIDs []int64) ([]educationRepo.TeacherGroupID, error) {
		assignments, err := membership.ListGroupAssignments(ctx, schoolmembership.GroupAssignmentFilter{GroupIDs: groupIDs, TeacherIDs: teacherIDs})
		if err != nil {
			return nil, err
		}
		result := make([]educationRepo.TeacherGroupID, 0, len(assignments))
		for _, assignment := range assignments {
			result = append(result, educationRepo.TeacherGroupID{TeacherID: assignment.TeacherID, GroupID: assignment.GroupID})
		}
		return result, nil
	}
}

type educationRoomDirectory struct{ rooms facilitiesModule.Query }

func (d educationRoomDirectory) ListRoomsByID(ctx context.Context, ids []int64) ([]educationRepo.DirectoryRoom, error) {
	if d.rooms == nil {
		return nil, fmt.Errorf("education repositories: room directory is not bound")
	}
	rooms, err := d.rooms.ListRoomsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]educationRepo.DirectoryRoom, 0, len(rooms))
	for _, room := range rooms {
		result = append(result, educationRepo.DirectoryRoom{
			ID: room.ID, TenantID: room.TenantID, CreatedAt: room.CreatedAt, UpdatedAt: room.UpdatedAt,
			Name: room.Name, Building: room.Building, Floor: room.Floor, Capacity: room.Capacity,
			Category: room.Category, Color: room.Color,
		})
	}
	return result, nil
}

// ListGroupSupervisors answers who supervises the groups on the day for the
// staff notification recipients.
func (r *EducationGroupStore) ListGroupSupervisors(ctx context.Context, groupIDs []int64, on calendar.Date) ([]deliveryCompose.StaffGroupPair, error) {
	pairs, err := r.ListStaffIDsByEducationGroupIDs(ctx, groupIDs, on)
	if err != nil {
		return nil, err
	}
	result := make([]deliveryCompose.StaffGroupPair, 0, len(pairs))
	for _, pair := range pairs {
		result = append(result, deliveryCompose.StaffGroupPair{StaffID: pair.StaffID, GroupID: pair.GroupID})
	}
	return result, nil
}
