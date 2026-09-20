package repositories

import (
	educationRepo "github.com/moto-nrw/project-phoenix/database/repositories/education"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	authModels "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/authmodels"
	authRepo "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/authpostgres"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/uptrace/bun"
)

// InvitationPersistence is the real persistence surface for invitation behavior
// tests. It excludes the unrelated capabilities built by the legacy factory.
type InvitationPersistence struct {
	Account       authModels.AccountRepository
	AccountTenant authModels.AccountTenantRepository
	Person        userModels.PersonRepository
	Staff         userModels.StaffRepository
	Teacher       userModels.TeacherRepository
	Student       userModels.StudentRepository
	School        organizationtenancy.Capability
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
		Account:       authRepo.NewAccountRepository(db),
		AccountTenant: authRepo.NewAccountTenantRepository(db),
		Person:        NewPersonRepository(db),
		Staff:         staff, Teacher: teachers,
		Student: NewStudentRepository(db),
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
	deps := newStaffMembershipDeps(NewPersonRepository(db), newIdentityAccess(db, nil))
	groupTeachers := newGroupTeacherRepository(membership, educationRepo.NewGroupRepository(db))
	deps.groupTeachers = func() educationModels.GroupTeacherRepository { return groupTeachers }
	return staffMembershipRepository{membership: membership, deps: deps}, teacherMembershipRepository{membership: membership, deps: deps}, nil
}
