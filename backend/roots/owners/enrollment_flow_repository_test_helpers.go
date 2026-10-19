//go:build testhooks

package owners

import (
	"fmt"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// EnrollmentFlowTestRepositories are the People Directory rows the
// enrollment flow suites of the HTTP adapter run on (#3565): the guardian
// and withdrawal rows the intake, decision, change-request, rollover
// and deletion flows read and write, bound the way the repository factory
// binds them. The Timetable rows, the Care Plan rows, the audit trails and
// the schools come from NewEnrollmentFlowTimetableRows,
// NewEnrollmentFlowCarePlanRows, NewEnrollmentFlowAuditRows and
// NewOrganizationTestRows over the same database (#4080).
type EnrollmentFlowTestRepositories struct {
	MembershipTestRepositories
	StudentGuardian     peoplePorts.StudentGuardianRepository
	GuardianProfile     peoplePorts.GuardianProfileRepository
	GuardianPhoneNumber peoplePorts.GuardianPhoneNumberRepository
	CareWithdrawal      CareWithdrawalTestRows
}

// Enrollment is the Enrollment booking projection the flows read and write.
func (r *EnrollmentFlowTestRepositories) Enrollment() EnrollmentBookingProjection {
	return NewEnrollmentBookingProjection(ComposeEnrollmentBookings(enrollmentCompose.New()))
}

// NewEnrollmentFlowTestRepositoriesWithMembers composes the enrollment flow
// suites' People Directory rows over the test database.
func NewEnrollmentFlowTestRepositoriesWithMembers(db transactionruntime.Database, members MembershipTestRepositories) (*EnrollmentFlowTestRepositories, error) {
	tt, err := NewTimetableTestRepositoriesWithMembers(db, members)
	if err != nil {
		return nil, err
	}
	carePlan, err := NewTimetableCarePlanRows(db, tt, members)
	if err != nil {
		return nil, err
	}
	care := carePlan.CarePlan
	return &EnrollmentFlowTestRepositories{
		MembershipTestRepositories: members,
		StudentGuardian:            members.StudentGuardian,
		GuardianProfile:            members.GuardianProfile,
		GuardianPhoneNumber:        peopleCompose.NewGuardianPhoneNumberRepository(db.Pool()),
		CareWithdrawal:             NewCareWithdrawalCompletionRepository(care),
	}, nil
}

// NewEnrollmentFlowCarePlanRows are the Care Plan rows of the enrollment flow
// suites.
func NewEnrollmentFlowCarePlanRows(db transactionruntime.Database) TimetableCarePlanRows {
	members := MustNewMembershipTestRepositories(db)
	tt, err := NewTimetableTestRepositoriesWithMembers(db, members)
	if err != nil {
		panic(fmt.Sprintf("enrollment flow test rows: compose timetable: %v", err))
	}
	rows, err := NewTimetableCarePlanRows(db, tt, members)
	if err != nil {
		panic(fmt.Sprintf("enrollment flow test rows: compose care plan: %v", err))
	}
	return rows
}

// EnrollmentFlowAuditRows are the Audit Platform trails the enrollment flows
// write.
type EnrollmentFlowAuditRows struct {
	EnrollmentOfferingAdjustment auditModels.EnrollmentOfferingAdjustmentRepository
	EnrollmentRestorationAudit   auditModels.EnrollmentRestorationRepository
	EnrollmentDeletionAudit      auditModels.EnrollmentDeletionRepository
	StudentFieldEdit             auditModels.StudentFieldEditRepository
}

// NewEnrollmentFlowAuditRows composes the trails over the test database, the
// composition that owns Audit persistence.
func NewEnrollmentFlowAuditRows(db transactionruntime.Database) *EnrollmentFlowAuditRows {
	audits := NewTestAuditPersistence(db, nil)
	return &EnrollmentFlowAuditRows{
		EnrollmentOfferingAdjustment: audits.EnrollmentOfferingAdjustment,
		EnrollmentRestorationAudit:   audits.EnrollmentRestorationAudit,
		EnrollmentDeletionAudit:      audits.EnrollmentDeletionAudit,
		StudentFieldEdit:             audits.StudentFieldEdit,
	}
}
