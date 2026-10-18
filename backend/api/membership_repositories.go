package api

import (
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	educationRepo "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// The retained staff, teacher, guest, teaching-assignment and group
// repositories are served over School Membership and School Structure
// (#2667, #2742) and built where they are used (#2743). The person half of a
// staff row comes from the People Directory repository, the account, role
// and permission half from Identity & Access, the employment-only writes from
// Workforce.

// bindMembershipAdapters binds the staff, teacher and guest adapters over
// membership, with the group store and the teaching assignments they resolve
// through.
func bindMembershipAdapters(persons staffPersonRows, identity staffIdentityQuery, membership schoolmembership.Capability, group *EducationGroupStore, employment workforce.StaffEmployments) (
	StaffMembershipRepository, TeacherMembershipRepository, guestMembershipRepository, *groupTeacherRepository, *classTeacherRepository,
) {
	groupTeachers := newGroupTeacherRepository(membership, group)
	deps := NewStaffMembershipDeps(persons, identity, employment)
	deps.GroupTeachers = func() GroupTeacherRepository { return groupTeachers }
	return StaffMembershipRepository{Membership: membership, Deps: deps}, TeacherMembershipRepository{Membership: membership, Deps: deps},
		guestMembershipRepository{membership: membership}, groupTeachers, &classTeacherRepository{membership: membership}
}

// newMembershipAdapters composes the staff, teacher, guest, group,
// teaching-assignment and class-teacher adapters over membership, with the
// group store over db and an unobserved Workforce employment owner.
func newMembershipAdapters(db transactionruntime.Database, persons staffPersonRows, identity staffIdentityQuery, membership schoolmembership.Capability, rooms facilitiesModule.Query, workTime workforce.Capability) (
	StaffMembershipRepository, TeacherMembershipRepository, guestMembershipRepository, *EducationGroupStore, *groupTeacherRepository, *classTeacherRepository,
) {
	group := NewGroupRepository(educationRepo.NewLegacyRepositoryRuntime(db.Pool()), rooms, membership, workTime)
	staff, teacher, guest, groupTeachers, classTeachers := bindMembershipAdapters(persons, identity, membership, group, workforceCompose.MustNewStaffEmployment(db.Pool()))
	return staff, teacher, guest, group, groupTeachers, classTeachers
}
