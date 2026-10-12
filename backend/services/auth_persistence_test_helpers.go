package services

import (
	"github.com/moto-nrw/project-phoenix/database/repositories"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/uptrace/bun"
)

// InvitationPersistence is the real persistence surface for invitation behavior
// tests. It excludes the unrelated capabilities built by the legacy factory.
type InvitationPersistence struct {
	Person  userModels.PersonRepository
	Staff   userModels.StaffRepository
	Teacher userModels.TeacherRepository
	Student userModels.StudentRepository
	School  organizationtenancy.Capability
}

// NewInvitationPersistence constructs only invitation dependencies through the
// existing composition seam, keeping postgres adapters out of behavior tests.
func NewInvitationPersistence(db *bun.DB) (*InvitationPersistence, error) {
	staff, teachers, err := newInvitationMembershipRepositories(db)
	if err != nil {
		return nil, err
	}
	organizations, err := repositories.NewOrganizationTenancy(db)
	if err != nil {
		return nil, err
	}
	return &InvitationPersistence{
		Person: NewPersonRepository(db),
		Staff:  staff, Teacher: teachers,
		Student: NewUnboundStudentRepository(db),
		School:  organizations,
	}, nil
}

// newInvitationMembershipRepositories composes only the staff and teacher adapters
// over School Membership, without constructing the legacy repository graph.
func newInvitationMembershipRepositories(db *bun.DB) (userModels.StaffRepository, userModels.TeacherRepository, error) {
	membership, err := repositories.NewSchoolMembership(db)
	if err != nil {
		return nil, nil, err
	}
	deps := newStaffMembershipDeps(NewPersonRepository(db), newPeopleIdentityAccess(db, nil), repositories.MustNewStaffEmployment(db))
	groupTeachers := newGroupTeacherRepository(membership, NewEducationGroupRepository(db))
	deps.groupTeachers = func() repositories.GroupTeacherRepository { return groupTeachers }
	return staffMembershipRepository{membership: membership, deps: deps}, teacherMembershipRepository{membership: membership, deps: deps}, nil
}
