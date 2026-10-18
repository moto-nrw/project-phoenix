//go:build testhooks

package api

import (
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
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
	StudentGuardian              peoplePorts.StudentGuardianRepository
	GuardianProfile              peoplePorts.GuardianProfileRepository
	GuardianPhoneNumber          peoplePorts.GuardianPhoneNumberRepository
	EnrollmentOfferingAdjustment auditModels.EnrollmentOfferingAdjustmentRepository
	EnrollmentRestorationAudit   auditModels.EnrollmentRestorationRepository
	EnrollmentDeletionAudit      auditModels.EnrollmentDeletionRepository
	StudentFieldEdit             auditModels.StudentFieldEditRepository
	CareWithdrawal               CareWithdrawalRecords
}

// NewEnrollmentFlowTestRepositoriesWithMembers composes the enrollment flow suites'
// repositories over the test database.
func NewEnrollmentFlowTestRepositoriesWithMembers(db transactionruntime.Database, members MembershipTestRepositories) (*EnrollmentFlowTestRepositories, error) {
	tt, err := NewTimetableTestRepositoriesWithMembers(db, members)
	if err != nil {
		return nil, err
	}
	organizations, err := NewOrganizationTenancy(db)
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
		CareWithdrawal:            NewLateBoundCareWithdrawalCompletionRepository(func() careplan.Capability { return care }),
	}, nil
}

// CarePlan returns the Care Plan capability the schedule adapters delegate
// to, as the repository factory hands it out.
func (r *EnrollmentFlowTestRepositories) CarePlan() careplan.Capability {
	return r.TimetableTestRepositories.CarePlan
}
