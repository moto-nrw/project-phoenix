package services

import (
	"context"

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

// bindMembershipRepositories binds the staff, teacher and guest adapters over
// membership, with the group store and the teaching assignments they resolve
// through. members.People, Person, Student and the guardian fields stay
// unset; the caller composes them.
func bindMembershipRepositories(persons userModels.PersonRepository, identity staffIdentityQuery, membership schoolmembership.Capability, group *educationGroupRepository, employment workforce.StaffEmployments) MembershipTestRepositories {
	groupTeachers := newGroupTeacherRepository(membership, group)
	deps := newStaffMembershipDeps(persons, identity, employment)
	deps.groupTeachers = func() GroupTeacherRepository { return groupTeachers }
	return MembershipTestRepositories{
		Person:       persons,
		Staff:        staffMembershipRepository{membership: membership, deps: deps},
		Teacher:      teacherMembershipRepository{membership: membership, deps: deps},
		Guest:        guestMembershipRepository{membership: membership},
		Group:        group,
		GroupTeacher: groupTeachers,
		ClassTeacher: newClassTeacherRepository(membership),
		Membership:   membership,
	}
}

// newGroupSubstitutionRepository serves the retained group substitution
// contract over Workforce (#2688): the rows are Workforce's, the groups
// resolve through the group store and the staff through School Membership.
func newGroupSubstitutionRepository(groups EducationGroupRepository, membership staffLookup, workTime workforce.Capability) GroupSubstitutionRepository {
	return newGroupSubstitutions(
		workforceLegacy.NewGroupSubstitutionRepository(workTime, func(ctx context.Context, ids []int64) (map[int64]string, error) {
			return NewGroupNames(groups).GroupNamesByID(ctx, ids)
		}),
		groups.FindByIDs,
		membership,
	)
}
