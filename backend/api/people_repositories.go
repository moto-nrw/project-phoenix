package api

import (
	"context"

	studentDirectoryView "github.com/moto-nrw/project-phoenix/modules/studentdirectoryview/compose"

	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// The retained People Directory repositories are built where they are used
// (#2743): modules/peopledirectory/compose constructs them over the owner's
// legacy runtime, and this composition binds what other owners hold — the
// accounts and memberships of Identity & Access, the enrollment and teacher
// assignments of School Membership, the group names of School Structure.

// BindStudentRepository binds the retained student repository to the
// teacher assignments of School Membership and the group names of School
// Structure. repository carries the database, the directory and the
// enrollment owner the caller composed.
func BindStudentRepository(repository peopleCompose.StudentRepositoryDependencies, membership groupAssignmentLister, groups schoolstructure.Query) peoplePorts.StudentRepository {
	repository.TeacherGroupIDs = assignedTeacherGroupIDs(membership)
	repository.TeacherStaffGroupIDs = assignedTeacherStaffGroupIDs(membership)
	repository.Students = studentDirectoryView.NewReader()
	students := peopleCompose.NewStudentRepository(repository)
	return OverlappingRosterGroupNames{StudentRepository: groupStudentRepository{StudentRepository: students, groups: groups}, Groups: groups}
}

// groupAssignmentLister is the School Membership teacher-assignment read the
// student roster resolves teacher groups through.
type groupAssignmentLister interface {
	ListGroupAssignments(context.Context, schoolmembership.GroupAssignmentFilter) ([]schoolmembership.GroupAssignment, error)
}

// activeSchoolMemberships is the Identity & Access membership read the
// relationship store's permission checks filter through.
type activeSchoolMemberships interface {
	FindActiveSchoolMemberships(context.Context, []int64, []int64) (map[int64][]int64, error)
}

func assignedTeacherGroupIDs(membership groupAssignmentLister) func(context.Context, int64) ([]int64, error) {
	return func(ctx context.Context, teacherID int64) ([]int64, error) {
		assignments, err := membership.ListGroupAssignments(ctx, schoolmembership.GroupAssignmentFilter{TeacherIDs: []int64{teacherID}})
		if err != nil {
			return nil, err
		}
		ids := make([]int64, 0, len(assignments))
		for _, assignment := range assignments {
			ids = append(ids, assignment.GroupID)
		}
		return ids, nil
	}
}

func assignedTeacherStaffGroupIDs(membership groupAssignmentLister) func(context.Context, []int64) ([]int64, error) {
	return func(ctx context.Context, staffIDs []int64) ([]int64, error) {
		assignments, err := membership.ListGroupAssignments(ctx, schoolmembership.GroupAssignmentFilter{TeacherStaffIDs: staffIDs})
		if err != nil {
			return nil, err
		}
		ids := make([]int64, 0, len(assignments))
		for _, assignment := range assignments {
			ids = append(ids, assignment.GroupID)
		}
		return ids, nil
	}
}

// NewCaregiverBindingLocker serializes the caregiver capability re-check over
// the four owners whose tables can retain caregiver capability: School
// Membership, Workforce, Student Presence and Timetable & Activities.
func NewCaregiverBindingLocker(membership schoolmembership.Capability, workTime workforce.Capability, presence studentpresence.Capability, timetableCapability timetable.Capability) peoplePorts.CaregiverBindingLocker {
	return peopleCompose.NewCaregiverBindingLocker(caregiverBindingOwners{
		membership: membership, workforce: workTime, presence: presence, timetable: timetableCapability,
	})
}

// caregiverBindingOwners adapts the four binding owners to the People
// Directory port that serializes the caregiver capability re-check.
type caregiverBindingOwners struct {
	membership interface{ LockGroupAssignments(context.Context) error }
	workforce  interface{ LockGroupSubstitutions(context.Context) error }
	presence   interface{ LockGroupSupervisions(context.Context) error }
	timetable  interface{ LockPlannedSupervisors(context.Context) error }
}

func (o caregiverBindingOwners) LockGroupAssignments(ctx context.Context) error {
	return o.membership.LockGroupAssignments(ctx)
}

func (o caregiverBindingOwners) LockGroupSubstitutions(ctx context.Context) error {
	return o.workforce.LockGroupSubstitutions(ctx)
}

func (o caregiverBindingOwners) LockGroupSupervisions(ctx context.Context) error {
	return o.presence.LockGroupSupervisions(ctx)
}

func (o caregiverBindingOwners) LockPlannedSupervisors(ctx context.Context) error {
	return o.timetable.LockPlannedSupervisors(ctx)
}
