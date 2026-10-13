package services

import (
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	usersModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// MembershipTestRepositories are the People Directory and School Membership
// repositories the test graphs of this package read through. services
// composes them where it composes the owners (services.NewMembershipTestRepositories,
// #2743).
type MembershipTestRepositories struct {
	// People is the People Directory owner the student rows translate to.
	People          peopledirectory.Capability
	Person          usersModels.PersonRepository
	Student         usersModels.StudentRepository
	GuardianProfile usersModels.GuardianProfileRepository
	StudentGuardian usersModels.StudentGuardianRepository
	Staff           usersModels.StaffRepository
	Teacher         usersModels.TeacherRepository
	Guest           usersModels.GuestRepository
	Group           EducationGroupRepository
	GroupTeacher    GroupTeacherRepository
	ClassTeacher    ClassTeacherRepository
	// GroupSubstitution is the Workforce group substitution contract over the
	// group store and School Membership.
	GroupSubstitution GroupSubstitutionRepository
	// Membership is the owner capability itself; the class-list entries
	// (#2382) are read and written through it.
	Membership schoolmembership.Capability
}
