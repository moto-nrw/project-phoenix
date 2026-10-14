package services

import (
	"context"
	"fmt"

	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/workforce"

	"github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
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
		SupervisingStaff:    supervisionStaffResolver(membership, workforceSubstitutedStaff(workTime)),
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
func (r *EducationGroupStore) ListGroupSupervisors(ctx context.Context, groupIDs []int64, on calendar.Date) ([]notifications.StaffGroupPair, error) {
	pairs, err := r.ListStaffIDsByEducationGroupIDs(ctx, groupIDs, on)
	if err != nil {
		return nil, err
	}
	result := make([]notifications.StaffGroupPair, 0, len(pairs))
	for _, pair := range pairs {
		result = append(result, notifications.StaffGroupPair{StaffID: pair.StaffID, GroupID: pair.GroupID})
	}
	return result, nil
}

// substitutedStaffQuery answers who substitutes in the given groups on a
// day. education.group_substitution belongs to Workforce (#2688); the root
// supplies the owner's query so the group repository never joins the table.
type substitutedStaffQuery = func(ctx context.Context, groupIDs []int64, on string) ([]educationRepo.StaffGroupID, error)

// supervisionStaffResolver resolves who supervises a group on a day: the
// assigned teachers through their live teacher and staff rows, plus the
// substituting staff that are still live. Offboarded teachers and staff drop
// out, as the replaced inner joins did.
func supervisionStaffResolver(membership staffLookup, substitutions substitutedStaffQuery) func(context.Context, educationRepo.GroupMembershipPairs) ([]educationRepo.StaffGroupID, error) {
	return func(ctx context.Context, raw educationRepo.GroupMembershipPairs) ([]educationRepo.StaffGroupID, error) {
		staffByTeacher, err := resolveTeacherStaff(ctx, membership, raw.Assigned)
		if err != nil {
			return nil, err
		}
		if substitutions == nil {
			return nil, fmt.Errorf("group supervision resolves substitutions through Workforce")
		}
		substituted, err := substitutions(ctx, raw.GroupIDs, raw.On.String())
		if err != nil {
			return nil, fmt.Errorf("load substitutions of group supervision: %w", err)
		}
		substituteIDs := make([]int64, 0, len(substituted))
		for _, pair := range substituted {
			substituteIDs = append(substituteIDs, pair.StaffID)
		}
		liveSubstitutes, err := staffByID(ctx, membership, substituteIDs, false)
		if err != nil {
			return nil, err
		}

		// Assignments first, then substitutions, each in query order.
		result := make([]educationRepo.StaffGroupID, 0, len(raw.Assigned)+len(substituted))
		seen := make(map[educationRepo.StaffGroupID]struct{}, cap(result))
		appendPair := func(pair educationRepo.StaffGroupID) {
			if _, dup := seen[pair]; dup {
				return
			}
			seen[pair] = struct{}{}
			result = append(result, pair)
		}
		for _, pair := range raw.Assigned {
			staffID, found := staffByTeacher[pair.TeacherID]
			if !found {
				continue
			}
			appendPair(educationRepo.StaffGroupID{StaffID: staffID, GroupID: pair.GroupID})
		}
		for _, pair := range substituted {
			if _, found := liveSubstitutes[pair.StaffID]; !found {
				continue
			}
			appendPair(pair)
		}
		return result, nil
	}
}

// workforceSubstitutedStaff adapts the Workforce capability to the
// supervision resolver's substitution query.
func workforceSubstitutedStaff(capability workforce.Capability) substitutedStaffQuery {
	if capability == nil {
		return nil
	}
	return func(ctx context.Context, groupIDs []int64, on string) ([]educationRepo.StaffGroupID, error) {
		if len(groupIDs) == 0 {
			return []educationRepo.StaffGroupID{}, nil
		}
		rows, err := capability.ListGroupSubstitutions(ctx, workforce.GroupSubstitutionFilter{GroupIDs: groupIDs, On: on})
		if err != nil {
			return nil, err
		}
		pairs := make([]educationRepo.StaffGroupID, 0, len(rows))
		for _, row := range rows {
			pairs = append(pairs, educationRepo.StaffGroupID{StaffID: row.SubstituteStaffID, GroupID: row.GroupID})
		}
		return pairs, nil
	}
}

// resolveTeacherStaff maps every live teacher of the assignments to their
// live staff member. A teacher or staff row that is gone yields no entry.
func resolveTeacherStaff(ctx context.Context, membership staffLookup, assigned []educationRepo.TeacherGroupID) (map[int64]int64, error) {
	result := make(map[int64]int64, len(assigned))
	ids := make([]int64, 0, len(assigned))
	for _, pair := range assigned {
		ids = append(ids, pair.TeacherID)
	}
	ids = uniqueIDs(ids)
	if len(ids) == 0 {
		return result, nil
	}
	teachers, err := membership.ListTeachers(ctx, schoolmembership.TeacherFilter{IDs: ids})
	if err != nil {
		return nil, fmt.Errorf("load teachers of group assignments: %w", err)
	}
	staffIDs := make([]int64, 0, len(teachers))
	for _, teacher := range teachers {
		staffIDs = append(staffIDs, teacher.StaffID)
	}
	live, err := staffByID(ctx, membership, staffIDs, false)
	if err != nil {
		return nil, err
	}
	for _, teacher := range teachers {
		if _, found := live[teacher.StaffID]; found {
			result[teacher.ID] = teacher.StaffID
		}
	}
	return result, nil
}
