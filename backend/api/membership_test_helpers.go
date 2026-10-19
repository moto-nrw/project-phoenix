//go:build testhooks

package api

import (
	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	educationRepo "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// authTestIdentityChain is the identity chain rows (persons, staff, teachers,
// students, guardian profiles and relationships) the auth test graphs bind.
func authTestIdentityChain(db transactionruntime.Database) (owners.LifecycleRepositories, error) {
	members, err := owners.NewMembershipTestRepositories(db)
	if err != nil {
		return owners.LifecycleRepositories{}, err
	}
	return identityChainOf(db, members), nil
}

func identityChainOf(db transactionruntime.Database, members owners.MembershipTestRepositories) owners.LifecycleRepositories {
	return owners.LifecycleRepositories{
		Persons: members.Person, Staff: members.Staff, Teachers: members.Teacher, Students: NewUnboundStudentRepository(db),
		GuardianProfiles: members.GuardianProfile, StudentGuardians: members.StudentGuardian,
		People: members.People, Membership: owners.MustNewSchoolMembership(db),
	}
}

// NewStudentDirectory adapts the People Directory's directory read to the
// retained model-typed contract, for suites that may not import the owner's
// composition.
func NewStudentDirectory(directory peopleCompose.StudentDirectoryCapability) *peopleCompose.StudentDirectory {
	return peopleCompose.NewStudentDirectory(directory)
}

// NewEducationGroupRepository is the retained group store on the tenant
// runtime of the School Structure owner over db, with only the given
// dependencies bound, for the test graphs whose test roles may not name the
// School Structure composition.
func NewEducationGroupRepository(db transactionruntime.Database, options ...educationRepo.GroupRepositoryDependencies) *owners.EducationGroupStore {
	return owners.NewEducationGroupStore(educationRepo.NewLegacyRepositoryRuntime(db.Pool()), options...)
}

// SchoolMembershipSuiteRepositories serve the School Membership owner the
// staff adapters and the class-list entries (#2382) read through.
type SchoolMembershipSuiteRepositories struct {
	Membership schoolmembership.Capability
}

// SchoolMembership is the owner capability.
func (r *SchoolMembershipSuiteRepositories) SchoolMembership() schoolmembership.Capability {
	return r.Membership
}

// NewSchoolMembershipSuiteRepositories composes the unobserved owner of db.
func NewSchoolMembershipSuiteRepositories(db transactionruntime.Database) *SchoolMembershipSuiteRepositories {
	return &SchoolMembershipSuiteRepositories{Membership: owners.MustNewSchoolMembership(db)}
}

// NewGroupTeacherRepository serves the retained teaching-assignment contract
// over membership, for the repository test graphs.
func NewGroupTeacherRepository(membership schoolmembership.Capability, groups owners.GroupsByID) owners.GroupTeacherRepository {
	return owners.ComposeGroupTeacherRepository(membership, groups)
}
