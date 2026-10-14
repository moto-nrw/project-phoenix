package api

import (
	"fmt"
	"time"

	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	educationRepo "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/services"
)

// NewMembershipTestRepositories composes the People Directory and School
// Membership repositories the repository test graphs read through, over
// unobserved owners (#2743).
func NewMembershipTestRepositories(db transactionruntime.Database) (services.MembershipTestRepositories, error) {
	membership, err := services.NewSchoolMembership(db.Pool())
	if err != nil {
		return services.MembershipTestRepositories{}, err
	}
	people, err := services.NewPeopleDirectory(db.Pool())
	if err != nil {
		return services.MembershipTestRepositories{}, err
	}
	rooms, err := facilitiesCompose.NewUnobserved(db.Pool())
	if err != nil {
		return services.MembershipTestRepositories{}, err
	}
	workTime, err := NewWorkforce(db, membership)
	if err != nil {
		return services.MembershipTestRepositories{}, err
	}
	groups, err := NewSchoolStructure(db)
	if err != nil {
		return services.MembershipTestRepositories{}, err
	}
	members := services.NewMembershipRepositories(db.Pool(), services.NewPersonRepository(db.Pool()), services.NewPeopleIdentityAccess(db.Pool(), nil), membership, rooms, workTime)
	members.People = people
	members.Student = services.NewStudentRepository(db.Pool(), membership, groups)
	members.GuardianProfile = services.NewGuardianProfileRepository(db.Pool())
	members.StudentGuardian = services.NewStudentGuardianRepository(db.Pool())
	members.GroupSubstitution = services.NewGroupSubstitutionRepository(members.Group, membership, workTime)
	return members, nil
}

// MustNewMembershipTestRepositories is NewMembershipTestRepositories for test
// graphs that cannot return an error; composing only fails on missing
// dependencies, which is a programming error at wiring time.
func MustNewMembershipTestRepositories(db transactionruntime.Database) services.MembershipTestRepositories {
	members, err := NewMembershipTestRepositories(db)
	if err != nil {
		panic(fmt.Sprintf("compose membership test repositories: %v", err))
	}
	return members
}

// authTestIdentityChain is the identity chain rows (persons, staff, teachers,
// students, guardian profiles and relationships) the auth test graphs bind.
func authTestIdentityChain(db transactionruntime.Database) (LifecycleRepositories, error) {
	members, err := NewMembershipTestRepositories(db)
	if err != nil {
		return LifecycleRepositories{}, err
	}
	return identityChainOf(db, members), nil
}

func identityChainOf(db transactionruntime.Database, members services.MembershipTestRepositories) LifecycleRepositories {
	return LifecycleRepositories{
		Persons: members.Person, Staff: members.Staff, Teachers: members.Teacher, Students: NewUnboundStudentRepository(db),
		GuardianProfiles: members.GuardianProfile, StudentGuardians: members.StudentGuardian,
	}
}

// NewStudentDirectory adapts the People Directory's directory read to the
// retained model-typed contract, for suites that may not import the owner's
// composition.
func NewStudentDirectory(directory peopleCompose.StudentDirectoryCapability) *peopleCompose.StudentDirectory {
	return peopleCompose.NewStudentDirectory(directory)
}

// NewWorkforce composes the work-time owner unobserved for the test graphs
// whose test roles may not name the Workforce composition. Production roots compose the
// module themselves (api/base.go) so runtime evidence is kept.
func NewWorkforce(db transactionruntime.Database, membership schoolmembership.Capability) (workforce.Capability, error) {
	return NewWorkforceWithClock(db, membership, nil)
}

// NewWorkforceWithClock is NewWorkforce with a pinned clock for the live
// work-session window and the calendar day; nil means the wall clock.
func NewWorkforceWithClock(db transactionruntime.Database, membership schoolmembership.Capability, now func() time.Time) (workforce.Capability, error) {
	return workforceCompose.New(workforceCompose.Dependencies{
		LockStaffAssignment: workforceStaffAssignmentLock(membership),
		DB:                  db.Pool(),
		LiveStaffIDs:        workforceLiveStaffIDs(membership),
		Observe:             func(workforceCompose.Observation) {},
		Now:                 now,
	})
}

// NewEducationGroupRepository is the retained group store on the tenant
// runtime of the School Structure owner over db, with only the given
// dependencies bound, for the test graphs whose test roles may not name the
// School Structure composition.
func NewEducationGroupRepository(db transactionruntime.Database, options ...educationRepo.GroupRepositoryDependencies) *services.EducationGroupStore {
	return services.NewEducationGroupStore(educationRepo.NewLegacyRepositoryRuntime(db.Pool()), options...)
}
