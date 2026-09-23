package repositories

import (
	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/uptrace/bun"
)

type EnrollmentTestRepositories struct {
	TimetableTestRepositories
	School organizationtenancy.Capability

	StudentGuardian     usersModels.StudentGuardianRepository
	GuardianProfile     usersModels.GuardianProfileRepository
	GuardianPhoneNumber usersModels.GuardianPhoneNumberRepository
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
	r := &Factory{db: db}
	r.BindPeopleDirectory(people)
	r.bindCarePlanAdapters(care)
	r.BindOrganizationTenancy(organizations)
	return EnrollmentTestRepositories{TimetableTestRepositories: tt,
		School:          r.School,
		StudentGuardian: NewStudentGuardianRepository(db), GuardianProfile: NewGuardianProfileRepository(db), GuardianPhoneNumber: usersRepo.NewGuardianPhoneNumberRepository(db),
		Membership:    members.Membership,
		DataAccessLog: dataAccessLogCommand{auditRepo.NewDataAccessLogRepository(newTestAuditRuntime(db)), command}}, nil
}
