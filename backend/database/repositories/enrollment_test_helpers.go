package repositories

import (
	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	authRepo "github.com/moto-nrw/project-phoenix/database/repositories/auth"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/uptrace/bun"
)

type EnrollmentTestRepositories struct {
	TimetableTestRepositories
	School              organizationtenancy.Capability
	Account             authModels.AccountRepository
	AccountTenant       authModels.AccountTenantRepository
	AccountRole         authModels.AccountRoleRepository
	Role                authModels.RoleRepository
	StudentGuardian     usersModels.StudentGuardianRepository
	GuardianProfile     usersModels.GuardianProfileRepository
	GuardianPhoneNumber usersModels.GuardianPhoneNumberRepository
	StudentCompanion    usersModels.StudentCompanionRepository
	Membership          schoolmembership.Capability
	DataAccessLog       auditModels.DataAccessLogRepository
}

func NewEnrollmentTestRepositories(db *bun.DB, command auditModels.Command) (EnrollmentTestRepositories, error) {
	tt, err := NewTimetableTestRepositories(db)
	if err != nil {
		return EnrollmentTestRepositories{}, err
	}
	people, err := NewPeopleDirectory(db)
	if err != nil {
		return EnrollmentTestRepositories{}, err
	}
	care, err := NewCarePlan(db, people, tt.InstanceStudent)
	if err != nil {
		return EnrollmentTestRepositories{}, err
	}
	organizations, err := NewOrganizationTenancy(db)
	if err != nil {
		return EnrollmentTestRepositories{}, err
	}
	members, err := NewMembershipTestRepositories(db)
	if err != nil {
		return EnrollmentTestRepositories{}, err
	}
	r := &Factory{db: db,
		Account: members.Account, AccountTenant: members.AccountTenant,
	}
	r.BindPeopleDirectory(people)
	r.bindCarePlanAdapters(care)
	r.BindOrganizationTenancy(organizations)
	return EnrollmentTestRepositories{TimetableTestRepositories: tt,
		School: r.School, Account: r.Account, AccountTenant: r.AccountTenant,
		AccountRole: authRepo.NewAccountRoleRepository(db), Role: authRepo.NewRoleRepository(db),
		StudentGuardian: NewStudentGuardianRepository(db), GuardianProfile: NewGuardianProfileRepository(db), GuardianPhoneNumber: usersRepo.NewGuardianPhoneNumberRepository(db),
		StudentCompanion: r.StudentCompanion, Membership: members.Membership,
		DataAccessLog: dataAccessLogCommand{auditRepo.NewDataAccessLogRepository(newTestAuditRuntime(db)), command}}, nil
}
