package repositories

import (
	authRepo "github.com/moto-nrw/project-phoenix/database/repositories/auth"
	educationRepo "github.com/moto-nrw/project-phoenix/database/repositories/education"
	platformRepo "github.com/moto-nrw/project-phoenix/database/repositories/platform"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// InvitationPersistence is the real persistence surface for invitation behavior
// tests. It excludes the unrelated capabilities built by the legacy factory.
type InvitationPersistence struct {
	InvitationToken authModels.InvitationTokenRepository
	Account         authModels.AccountRepository
	AccountTenant   authModels.AccountTenantRepository
	Role            authModels.RoleRepository
	Permission      authModels.PermissionRepository
	AccountRole     authModels.AccountRoleRepository
	MFACredential   authModels.MFACredentialRepository
	Person          userModels.PersonRepository
	Staff           userModels.StaffRepository
	Teacher         userModels.TeacherRepository
	Student         userModels.StudentRepository
	School          platformModels.SchoolRepository
}

// NewInvitationPersistence constructs only invitation dependencies through the
// existing composition seam, keeping postgres adapters out of behavior tests.
func NewInvitationPersistence(db *bun.DB) (*InvitationPersistence, error) {
	staff, teachers, err := newInvitationMembershipRepositories(db)
	if err != nil {
		return nil, err
	}
	return &InvitationPersistence{
		InvitationToken: authRepo.NewInvitationTokenRepository(db),
		Account:         authRepo.NewAccountRepository(db),
		AccountTenant:   authRepo.NewAccountTenantRepository(db),
		Role:            authRepo.NewRoleRepository(db),
		Permission:      authRepo.NewPermissionRepository(db),
		AccountRole:     authRepo.NewAccountRoleRepository(db),
		MFACredential:   authRepo.NewMFACredentialRepository(db),
		Person:          usersRepo.NewPersonRepository(db),
		Staff:           staff, Teacher: teachers,
		Student: usersRepo.NewStudentRepository(db),
		School:  platformRepo.NewSchoolRepository(db),
	}, nil
}

// SessionValidationPersistence contains only the repositories consulted when
// validating an existing token pair; it does not build the login/MFA graph.
type SessionValidationPersistence struct {
	Account              authModels.AccountRepository
	AccountTenant        authModels.AccountTenantRepository
	Token                authModels.TokenRepository
	Operator             platformModels.OperatorRepository
	OperatorRefreshToken platformModels.OperatorRefreshTokenRepository
}

func NewSessionValidationPersistence(db *bun.DB) *SessionValidationPersistence {
	return &SessionValidationPersistence{
		Account:              authRepo.NewAccountRepository(db),
		AccountTenant:        authRepo.NewAccountTenantRepository(db),
		Token:                authRepo.NewTokenRepository(db),
		Operator:             platformRepo.NewOperatorRepository(db),
		OperatorRefreshToken: platformRepo.NewOperatorRefreshTokenRepository(db),
	}
}

// newInvitationMembershipRepositories composes only the staff and teacher adapters
// over School Membership, without constructing the legacy repository graph.
func newInvitationMembershipRepositories(db *bun.DB) (userModels.StaffRepository, userModels.TeacherRepository, error) {
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return nil, nil, err
	}
	deps := newStaffMembershipDeps(usersRepo.NewPersonRepository(db), authRepo.NewAccountRepository(db), authRepo.NewAccountTenantRepository(db), authRepo.NewPermissionRepository(db), authRepo.NewRoleRepository(db))
	groupTeachers := newGroupTeacherRepository(membership, educationRepo.NewGroupRepository(db))
	deps.groupTeachers = func() educationModels.GroupTeacherRepository { return groupTeachers }
	return staffMembershipRepository{membership: membership, deps: deps}, teacherMembershipRepository{membership: membership, deps: deps}, nil
}
