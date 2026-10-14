package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceLegacy "github.com/moto-nrw/project-phoenix/modules/workforce/legacy"
)

// The retained staff, teacher, guest, teaching-assignment and group
// repositories are served over School Membership and School Structure
// (#2667, #2742) and built where they are used (#2743). The person half of a
// staff row comes from the People Directory repository, the account, role
// and permission half from Identity & Access, the employment-only writes from
// Workforce.

// MembershipTestRepositories are the People Directory and School Membership
// repositories the retained graphs read through. services composes them where
// it composes the owners; the test graphs build them through
// api.NewMembershipTestRepositories (#2743, #4005).
type MembershipTestRepositories struct {
	// People is the People Directory owner the student rows translate to.
	People          peopledirectory.Capability
	Person          userModels.PersonRepository
	Student         userModels.StudentRepository
	GuardianProfile userModels.GuardianProfileRepository
	StudentGuardian userModels.StudentGuardianRepository
	Staff           userModels.StaffRepository
	Teacher         userModels.TeacherRepository
	Guest           userModels.GuestRepository
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

// bindMembershipRepositories binds the staff, teacher and guest adapters over
// membership, with the group store and the teaching assignments they resolve
// through. members.People, Person, Student and the guardian fields stay
// unset; the caller composes them.
func bindMembershipRepositories(persons userModels.PersonRepository, identity staffIdentityQuery, membership schoolmembership.Capability, group *EducationGroupStore, employment workforce.StaffEmployments) MembershipTestRepositories {
	groupTeachers := NewGroupTeacherRepository(membership, group)
	deps := NewStaffMembershipDeps(persons, identity, employment)
	deps.GroupTeachers = func() GroupTeacherRepository { return groupTeachers }
	return MembershipTestRepositories{
		Person:       persons,
		Staff:        StaffMembershipRepository{Membership: membership, Deps: deps},
		Teacher:      TeacherMembershipRepository{Membership: membership, Deps: deps},
		Guest:        guestMembershipRepository{membership: membership},
		Group:        group,
		GroupTeacher: groupTeachers,
		ClassTeacher: newClassTeacherRepository(membership),
		Membership:   membership,
	}
}

// NewGroupSubstitutionRepository serves the retained group substitution
// contract over Workforce (#2688): the rows are Workforce's, the groups
// resolve through the group store and the staff through School Membership.
func NewGroupSubstitutionRepository(groups EducationGroupRepository, membership staffLookup, workTime workforce.Capability) GroupSubstitutionRepository {
	return newGroupSubstitutions(
		workforceLegacy.NewGroupSubstitutionRepository(workTime, func(ctx context.Context, ids []int64) (map[int64]string, error) {
			return NewGroupNames(groups).GroupNamesByID(ctx, ids)
		}),
		groups.FindByIDs,
		membership,
	)
}
