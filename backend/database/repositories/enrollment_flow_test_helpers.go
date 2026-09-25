package repositories

import (
	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/uptrace/bun"
)

// EnrollmentFlowTestRepositories is the repository set the enrollment flow
// suites of the HTTP adapter run on (#3565): the Timetable test graph plus
// the school, guardian and audit repositories the intake, decision,
// change-request, rollover and deletion flows read and write, bound the way
// the repository factory binds them.
type EnrollmentFlowTestRepositories struct {
	TimetableTestRepositories
	School                       organizationtenancy.Capability
	StudentGuardian              usersModels.StudentGuardianRepository
	GuardianProfile              usersModels.GuardianProfileRepository
	GuardianPhoneNumber          usersModels.GuardianPhoneNumberRepository
	EnrollmentOfferingAdjustment auditModels.EnrollmentOfferingAdjustmentRepository
	EnrollmentRestorationAudit   auditModels.EnrollmentRestorationRepository
	EnrollmentDeletionAudit      auditModels.EnrollmentDeletionRepository
	StudentFieldEdit             auditModels.StudentFieldEditRepository
	CareWithdrawal               usersModels.CareWithdrawalCompletionRepository
}

// NewEnrollmentFlowTestRepositories composes the enrollment flow suites'
// repositories over the test database.
func NewEnrollmentFlowTestRepositories(db *bun.DB) (*EnrollmentFlowTestRepositories, error) {
	tt, err := NewTimetableTestRepositories(db)
	if err != nil {
		return nil, err
	}
	organizations, err := NewOrganizationTenancy(db)
	if err != nil {
		return nil, err
	}
	care := tt.CarePlan
	audits := newTestAuditRuntime(db)
	return &EnrollmentFlowTestRepositories{
		TimetableTestRepositories:    tt,
		School:                       organizations,
		StudentGuardian:              NewStudentGuardianRepository(db),
		GuardianProfile:              NewGuardianProfileRepository(db),
		GuardianPhoneNumber:          usersRepo.NewGuardianPhoneNumberRepository(db),
		EnrollmentOfferingAdjustment: auditRepo.NewEnrollmentOfferingAdjustmentRepository(audits),
		EnrollmentRestorationAudit:   auditRepo.NewEnrollmentRestorationRepository(audits),
		EnrollmentDeletionAudit:      auditRepo.NewEnrollmentDeletionRepository(audits),
		StudentFieldEdit:             auditRepo.NewStudentFieldEditRepository(audits),
		CareWithdrawal:               newCareWithdrawalCompletionRepository(func() careplan.Capability { return care }),
	}, nil
}

// CarePlan returns the Care Plan capability the schedule adapters delegate
// to, as the repository factory hands it out.
func (r *EnrollmentFlowTestRepositories) CarePlan() careplan.Capability {
	return r.TimetableTestRepositories.CarePlan
}
