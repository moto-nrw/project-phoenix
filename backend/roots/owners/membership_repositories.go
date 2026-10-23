package owners

import (
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	educationRepo "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// bindMembershipAdapters binds the staff, teacher and guest adapters over
// membership, with the group store and the teaching assignments they resolve
// through.
func bindMembershipAdapters(persons staffPersonRows, identity staffIdentityQuery, membership schoolmembership.Capability, group *EducationGroupStore) (
	StaffMembershipRepository, TeacherMembershipRepository, GuestMembershipRepository, *GroupTeacherRepositoryBinding, *ClassTeacherRepositoryBinding,
) {
	groupTeachers := ComposeGroupTeacherRepository(membership, group)
	deps := NewStaffMembershipDeps(persons, identity, groupTeachers)
	return StaffMembershipRepository{Membership: membership, Deps: deps}, TeacherMembershipRepository{Membership: membership, Deps: deps},
		GuestMembershipRepository{Membership: membership}, groupTeachers, &ClassTeacherRepositoryBinding{Membership: membership}
}

// newMembershipAdapters composes the staff, teacher, guest, group,
// teaching-assignment and class-teacher adapters over membership, with the
// group store over db.
func newMembershipAdapters(db transactionruntime.Database, persons staffPersonRows, identity staffIdentityQuery, membership schoolmembership.Capability, rooms facilitiesModule.Query, workTime workforce.Capability) (
	StaffMembershipRepository, TeacherMembershipRepository, GuestMembershipRepository, *EducationGroupStore, *GroupTeacherRepositoryBinding, *ClassTeacherRepositoryBinding,
) {
	group := NewGroupRepository(educationRepo.NewLegacyRepositoryRuntime(db.Pool()), rooms, membership, workTime)
	staff, teacher, guest, groupTeachers, classTeachers := bindMembershipAdapters(persons, identity, membership, group)
	return staff, teacher, guest, group, groupTeachers, classTeachers
}
