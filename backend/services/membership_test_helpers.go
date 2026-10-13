package services

import (
	"fmt"
	"time"

	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"

	"github.com/uptrace/bun"
)

// NewMembershipTestRepositories composes the People Directory and School
// Membership repositories the repository test graphs read through, over
// unobserved owners (#2743).
func NewMembershipTestRepositories(db *bun.DB) (MembershipTestRepositories, error) {
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return MembershipTestRepositories{}, err
	}
	people, err := NewPeopleDirectory(db)
	if err != nil {
		return MembershipTestRepositories{}, err
	}
	rooms, err := facilitiesCompose.NewUnobserved(db)
	if err != nil {
		return MembershipTestRepositories{}, err
	}
	workTime, err := NewWorkforce(db, membership)
	if err != nil {
		return MembershipTestRepositories{}, err
	}
	groups, err := NewSchoolStructure(db)
	if err != nil {
		return MembershipTestRepositories{}, err
	}
	members := newMembershipRepositories(db, NewPersonRepository(db), newPeopleIdentityAccess(db, nil), membership, rooms, workTime)
	members.People = people
	members.Student = NewStudentRepository(db, membership, groups)
	members.GuardianProfile = NewGuardianProfileRepository(db)
	members.StudentGuardian = NewStudentGuardianRepository(db)
	members.GroupSubstitution = newGroupSubstitutionRepository(members.Group, membership, workTime)
	return members, nil
}

// MustNewMembershipTestRepositories is NewMembershipTestRepositories for test
// graphs that cannot return an error; composing only fails on missing
// dependencies, which is a programming error at wiring time.
func MustNewMembershipTestRepositories(db *bun.DB) MembershipTestRepositories {
	members, err := NewMembershipTestRepositories(db)
	if err != nil {
		panic(fmt.Sprintf("compose membership test repositories: %v", err))
	}
	return members
}

// authTestIdentityChain is the identity chain rows (persons, staff, teachers,
// students, guardian profiles and relationships) the auth test graphs bind.
func authTestIdentityChain(db *bun.DB) (lifecycleRepositories, error) {
	members, err := NewMembershipTestRepositories(db)
	if err != nil {
		return lifecycleRepositories{}, err
	}
	return identityChainOf(db, members), nil
}

func identityChainOf(db *bun.DB, members MembershipTestRepositories) lifecycleRepositories {
	return lifecycleRepositories{
		persons: members.Person, staff: members.Staff, teachers: members.Teacher, students: NewUnboundStudentRepository(db),
		guardianProfiles: members.GuardianProfile, studentGuardians: members.StudentGuardian,
	}
}

// The test graph vocabulary the root tests reach through api/testutil, which
// may not name the repository composition.
type (
	MembershipTestDependencies = MembershipTestRepositories
	TimetableTestDependencies  = TimetableDependencies
)

// NewStudentDirectory adapts the People Directory's directory read to the
// retained model-typed contract, for suites that may not import the owner's
// composition.
func NewStudentDirectory(directory peopleCompose.StudentDirectoryCapability) *peopleCompose.StudentDirectory {
	return peopleCompose.NewStudentDirectory(directory)
}

// NewWorkforce composes the work-time owner unobserved for the test graphs
// whose test roles may not name the Workforce composition. Production roots compose the
// module themselves (api/base.go) so runtime evidence is kept.
func NewWorkforce(db *bun.DB, membership schoolmembership.Capability) (workforce.Capability, error) {
	return NewWorkforceWithClock(db, membership, nil)
}

// NewWorkforceWithClock is NewWorkforce with a pinned clock for the live
// work-session window and the calendar day; nil means the wall clock.
func NewWorkforceWithClock(db *bun.DB, membership schoolmembership.Capability, now func() time.Time) (workforce.Capability, error) {
	return workforceCompose.New(workforceCompose.Dependencies{
		LockStaffAssignment: WorkforceStaffAssignmentLock(membership),
		DB:                  db,
		LiveStaffIDs:        WorkforceLiveStaffIDs(membership),
		Observe:             func(workforceCompose.Observation) {},
		Now:                 now,
	})
}
