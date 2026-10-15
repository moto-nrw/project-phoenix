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

// membershipAdapters are the retained staff, teacher, guest, group and
// teaching-assignment adapters over School Membership and School Structure.
type membershipAdapters struct {
	Staff        StaffMembershipRepository
	Teacher      TeacherMembershipRepository
	Guest        guestMembershipRepository
	Group        *EducationGroupStore
	GroupTeacher *groupTeacherRepository
	ClassTeacher *classTeacherRepository
}

// bindMembershipAdapters binds the staff, teacher and guest adapters over
// membership, with the group store and the teaching assignments they resolve
// through.
func bindMembershipAdapters(persons staffPersonRows, identity staffIdentityQuery, membership schoolmembership.Capability, group *EducationGroupStore, employment workforce.StaffEmployments) membershipAdapters {
	groupTeachers := newGroupTeacherRepository(membership, group)
	deps := NewStaffMembershipDeps(persons, identity, employment)
	deps.GroupTeachers = func() GroupTeacherRepository { return groupTeachers }
	return membershipAdapters{
		Staff:        StaffMembershipRepository{Membership: membership, Deps: deps},
		Teacher:      TeacherMembershipRepository{Membership: membership, Deps: deps},
		Guest:        guestMembershipRepository{membership: membership},
		Group:        group,
		GroupTeacher: groupTeachers,
		ClassTeacher: &classTeacherRepository{membership: membership},
	}
}

// newMembershipAdapters composes the adapters over membership, with the group
// store over db and an unobserved Workforce employment owner.
func newMembershipAdapters(db transactionruntime.Database, persons staffPersonRows, identity staffIdentityQuery, membership schoolmembership.Capability, rooms facilitiesModule.Query, workTime workforce.Capability) membershipAdapters {
	return bindMembershipAdapters(persons, identity, membership,
		NewGroupRepository(educationRepo.NewLegacyRepositoryRuntime(db.Pool()), rooms, membership, workTime),
		workforceCompose.MustNewStaffEmployment(db.Pool()))
}
