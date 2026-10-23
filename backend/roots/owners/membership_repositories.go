package owners

import (
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	educationRepo "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// bindMembershipAdapters binds the staff, teacher and external caregiver record adapters over
// membership, with the group store and the teaching assignments they resolve
// through.
func bindMembershipAdapters(persons staffPersonRows, identity staffIdentityQuery, membership schoolmembership.Capability, group *EducationGroupStore, employment workforce.StaffEmployments) (
	StaffMembershipRepository, TeacherMembershipRepository, ExternalCaregiverRecordMembershipRepository, *GroupTeacherRepositoryBinding, *ClassTeacherRepositoryBinding,
) {
	groupTeachers := ComposeGroupTeacherRepository(membership, group)
	deps := NewStaffMembershipDeps(persons, identity, employment, groupTeachers)
	return StaffMembershipRepository{Membership: membership, Deps: deps}, TeacherMembershipRepository{Membership: membership, Deps: deps},
		ExternalCaregiverRecordMembershipRepository{Membership: membership}, groupTeachers, &ClassTeacherRepositoryBinding{Membership: membership}
}

// newMembershipAdapters composes the staff, teacher, external caregiver record, group,
// teaching-assignment and class-teacher adapters over membership, with the
// group store over db and an unobserved Workforce employment owner.
func newMembershipAdapters(db transactionruntime.Database, persons staffPersonRows, identity staffIdentityQuery, membership schoolmembership.Capability, rooms facilitiesModule.Query, workTime workforce.Capability) (
	StaffMembershipRepository, TeacherMembershipRepository, ExternalCaregiverRecordMembershipRepository, *EducationGroupStore, *GroupTeacherRepositoryBinding, *ClassTeacherRepositoryBinding,
) {
	group := NewGroupRepository(educationRepo.NewLegacyRepositoryRuntime(db.Pool()), rooms, membership, workTime)
	staff, teacher, records, groupTeachers, classTeachers := bindMembershipAdapters(persons, identity, membership, group, workforceCompose.MustNewStaffEmployment(db.Pool()))
	return staff, teacher, records, group, groupTeachers, classTeachers
}
