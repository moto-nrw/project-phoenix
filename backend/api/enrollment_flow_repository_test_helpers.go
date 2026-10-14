package api

import (
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	usersModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/services"
)

// EnrollmentFlowTestRepositories is the repository set the enrollment flow
// suites of the HTTP adapter run on (#3565): the Timetable test graph plus
// the school, guardian and audit repositories the intake, decision,
// change-request, rollover and deletion flows read and write, bound the way
// the repository factory binds them. The audit repositories are filled by the
// composition that owns Audit persistence
// (NewEnrollmentFlowTestRepositories).
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

// NewEnrollmentFlowTestRepositoriesWithMembers composes the enrollment flow suites'
// repositories over the test database.
func NewEnrollmentFlowTestRepositoriesWithMembers(db transactionruntime.Database, members services.MembershipTestRepositories) (*EnrollmentFlowTestRepositories, error) {
	tt, err := NewTimetableTestRepositoriesWithMembers(db, members)
	if err != nil {
		return nil, err
	}
	organizations, err := services.NewOrganizationTenancy(db.Pool())
	if err != nil {
		return nil, err
	}
	care := tt.CarePlan
	return &EnrollmentFlowTestRepositories{
		TimetableTestRepositories: tt,
		School:                    organizations,
		StudentGuardian:           members.StudentGuardian,
		GuardianProfile:           members.GuardianProfile,
		GuardianPhoneNumber:       peopleCompose.NewGuardianPhoneNumberRepository(db.Pool()),
		CareWithdrawal:            services.NewLateBoundCareWithdrawalCompletionRepository(func() careplan.Capability { return care }),
	}, nil
}

// CarePlan returns the Care Plan capability the schedule adapters delegate
// to, as the repository factory hands it out.
func (r *EnrollmentFlowTestRepositories) CarePlan() careplan.Capability {
	return r.TimetableTestRepositories.CarePlan
}
