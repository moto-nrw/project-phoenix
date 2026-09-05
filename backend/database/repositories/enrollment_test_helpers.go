package repositories

import (
	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	authRepo "github.com/moto-nrw/project-phoenix/database/repositories/auth"
	enrollmentRepo "github.com/moto-nrw/project-phoenix/database/repositories/enrollment"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

type EnrollmentTestRepositories struct {
	TimetableTestRepositories
	Request              enrollmentModels.RequestRepository
	RequestChild         enrollmentModels.RequestChildRepository
	RequestGuardian      enrollmentModels.RequestGuardianRepository
	LateInvite           enrollmentModels.LateInviteRepository
	FormSchema           enrollmentModels.FormSchemaRepository
	ChangeRequest        enrollmentModels.ChangeRequestRepository
	ChangeRequestMessage enrollmentModels.ChangeRequestMessageRepository
	SubmissionRateLimit  enrollmentModels.SubmissionRateLimitRepository
	School               platformModels.SchoolRepository
	Account              authModels.AccountRepository
	AccountTenant        authModels.AccountTenantRepository
	AccountRole          authModels.AccountRoleRepository
	Role                 authModels.RoleRepository
	StudentGuardian      usersModels.StudentGuardianRepository
	GuardianProfile      usersModels.GuardianProfileRepository
	GuardianPhoneNumber  usersModels.GuardianPhoneNumberRepository
	StudentCompanion     usersModels.StudentCompanionRepository
	ClassListEntry       usersModels.ClassListEntryRepository
	DataAccessLog        auditModels.DataAccessLogRepository
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
	r := &Factory{db: db, Request: enrollmentRepo.NewRequestRepository(db), RequestChild: enrollmentRepo.NewRequestChildRepository(db),
		RequestGuardian: enrollmentRepo.NewRequestGuardianRepository(db), LateInvite: enrollmentRepo.NewLateInviteRepository(db),
		FormSchema: enrollmentRepo.NewFormSchemaRepository(db), ChangeRequest: enrollmentRepo.NewChangeRequestRepository(db),
		ChangeRequestMessage: enrollmentRepo.NewChangeRequestMessageRepository(db), SubmissionRateLimit: enrollmentRepo.NewSubmissionRateLimitRepository(db),
		Account: members.Account, AccountTenant: members.AccountTenant,
	}
	r.BindPeopleDirectory(people)
	r.bindCarePlanAdapters(care)
	r.BindOrganizationTenancy(organizations)
	return EnrollmentTestRepositories{TimetableTestRepositories: tt, Request: r.Request, RequestChild: r.RequestChild, RequestGuardian: r.RequestGuardian,
		LateInvite: r.LateInvite, FormSchema: r.FormSchema, ChangeRequest: r.ChangeRequest, ChangeRequestMessage: r.ChangeRequestMessage,
		SubmissionRateLimit: r.SubmissionRateLimit, School: r.School, Account: r.Account, AccountTenant: r.AccountTenant,
		AccountRole: authRepo.NewAccountRoleRepository(db), Role: authRepo.NewRoleRepository(db),
		StudentGuardian: usersRepo.NewStudentGuardianRepository(db), GuardianProfile: usersRepo.NewGuardianProfileRepository(db), GuardianPhoneNumber: usersRepo.NewGuardianPhoneNumberRepository(db),
		StudentCompanion: r.StudentCompanion, ClassListEntry: members.ClassListEntry,
		DataAccessLog: dataAccessLogCommand{auditRepo.NewDataAccessLogRepository(newTestAuditRuntime(db)), command}}, nil
}
