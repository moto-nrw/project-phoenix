//go:build testhooks

package api

import (
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	usersModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
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
	// The other owners' rows beside the Timetable rows (#4080).
	MembershipTestRepositories
	TimetablePresenceRows
	TimetableCarePlanRows
	TimetableRoomRows
	TimetableDeviationAudit
	TimetableSchoolCalendar
	School                       organizationtenancy.Capability
	StudentGuardian              peoplePorts.StudentGuardianRepository
	GuardianProfile              peoplePorts.GuardianProfileRepository
	GuardianPhoneNumber          peoplePorts.GuardianPhoneNumberRepository
	EnrollmentOfferingAdjustment auditModels.EnrollmentOfferingAdjustmentRepository
	EnrollmentRestorationAudit   auditModels.EnrollmentRestorationRepository
	EnrollmentDeletionAudit      auditModels.EnrollmentDeletionRepository
	StudentFieldEdit             auditModels.StudentFieldEditRepository
	CareWithdrawal               usersModels.CareWithdrawalCompletionRepository
}

// NewEnrollmentFlowTestRepositoriesWithMembers composes the enrollment flow suites'
// repositories over the test database.
func NewEnrollmentFlowTestRepositoriesWithMembers(db transactionruntime.Database, members MembershipTestRepositories) (*EnrollmentFlowTestRepositories, error) {
	tt, err := NewTimetableTestRepositoriesWithMembers(db, members)
	if err != nil {
		return nil, err
	}
	presence, carePlan, rooms, calendar, err := newTimetableTestCompanions(db, tt, members)
	if err != nil {
		return nil, err
	}
	organizations, err := NewOrganizationTenancy(db)
	if err != nil {
		return nil, err
	}
	care := carePlan.CarePlan
	return &EnrollmentFlowTestRepositories{
		TimetableTestRepositories: tt, MembershipTestRepositories: members, TimetablePresenceRows: presence, TimetableCarePlanRows: carePlan, TimetableRoomRows: rooms, TimetableSchoolCalendar: calendar,
		School:              organizations,
		StudentGuardian:     members.StudentGuardian,
		GuardianProfile:     members.GuardianProfile,
		GuardianPhoneNumber: peopleCompose.NewGuardianPhoneNumberRepository(db.Pool()),
		CareWithdrawal:      NewCareWithdrawalCompletionRepository(care),
	}, nil
}

// CarePlan returns the Care Plan capability the schedule adapters delegate
// to, as the repository factory hands it out.
func (r *EnrollmentFlowTestRepositories) CarePlan() careplan.Capability {
	return r.TimetableCarePlanRows.CarePlan
}
