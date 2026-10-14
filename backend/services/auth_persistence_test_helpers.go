package services

import (
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
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
	organizations, err := NewOrganizationTenancy(db)
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
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return nil, nil, err
	}
	deps := NewStaffMembershipDeps(NewPersonRepository(db), NewPeopleIdentityAccess(db, nil), workforceCompose.MustNewStaffEmployment(db))
	groupTeachers := NewGroupTeacherRepository(membership, NewEducationGroupRepository(db))
	deps.GroupTeachers = func() GroupTeacherRepository { return groupTeachers }
	return StaffMembershipRepository{Membership: membership, Deps: deps}, TeacherMembershipRepository{Membership: membership, Deps: deps}, nil
}
