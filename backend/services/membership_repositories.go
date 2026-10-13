package services

import (
	"context"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceLegacy "github.com/moto-nrw/project-phoenix/modules/workforce/legacy"
	"github.com/uptrace/bun"
)

// The retained staff, teacher, guest, teaching-assignment and group
// repositories are served over School Membership and School Structure
// (#2667, #2742) and built where they are used (#2743). The person half of a
// staff row comes from the People Directory repository, the account, role
// and permission half from Identity & Access, the employment-only writes from
// Workforce.

// newMembershipRepositories composes the staff, teacher and guest adapters
// over membership, with the group store and the teaching assignments they
// resolve through. members.People, Person, Student and the guardian fields
// stay unset; the caller composes them.
func newMembershipRepositories(db *bun.DB, persons userModels.PersonRepository, identity staffIdentityQuery, membership schoolmembership.Capability, rooms facilitiesModule.Query, workTime workforce.Capability) MembershipTestRepositories {
	group := NewGroupRepository(db, rooms, membership, workTime)
	groupTeachers := newGroupTeacherRepository(membership, group)
	deps := newStaffMembershipDeps(persons, identity, MustNewStaffEmployment(db))
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

// NewStaffRepository is the retained staff repository over School
// Membership, for the roots that read staff rows outside the service graph
// (the Worker's reminder recipients). observe records the Identity & Access
// operations the account half reads through; nil composes it unobserved.
func NewStaffRepository(db *bun.DB, membership schoolmembership.Capability, rooms facilitiesModule.Query, workTime workforce.Capability, observe IdentityAccessObserver) userModels.StaffRepository {
	return newMembershipRepositories(db, NewPersonRepository(db), newPeopleIdentityAccess(db, observe), membership, rooms, workTime).Staff
}
