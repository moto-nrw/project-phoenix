//go:build testhooks

package api

import (
	"fmt"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// EnrollmentTestRepositories are the People Directory rows of the
// enrollment suites and the Enrollment booking projection. The Timetable
// rows, the Care Plan rows, the access log, the schools and School
// Membership come from NewEnrollmentTimetableRows, NewEnrollmentCarePlanRows,
// NewEnrollmentAuditRows, NewOrganizationTestRows and
// NewSchoolMembershipSuiteRepositories over the same database (#4080).
type EnrollmentTestRepositories struct {
	MembershipTestRepositories
	StudentGuardian     peoplePorts.StudentGuardianRepository
	GuardianProfile     peoplePorts.GuardianProfileRepository
	GuardianPhoneNumber peoplePorts.GuardianPhoneNumberRepository
}

// Enrollment is the Enrollment booking projection the suites read and write.
func (r EnrollmentTestRepositories) Enrollment() EnrollmentBookingProjection {
	return NewEnrollmentBookingProjection(newEnrollmentBookings(enrollmentCompose.New()))
}

func NewEnrollmentTestRepositoriesWithMembers(db transactionruntime.Database, members MembershipTestRepositories) (EnrollmentTestRepositories, error) {
	return EnrollmentTestRepositories{
		MembershipTestRepositories: members,
		StudentGuardian:            members.StudentGuardian, GuardianProfile: members.GuardianProfile,
		GuardianPhoneNumber: peopleCompose.NewGuardianPhoneNumberRepository(db.Pool()),
	}, nil
}

// NewEnrollmentTimetableRows are the Timetable rows of the enrollment
// suites, with the deviation protocol bound to the Audit Platform.
func NewEnrollmentTimetableRows(db transactionruntime.Database) TimetableTestRepositories {
	tt, err := NewTimetableTestRepositoriesWithMembers(db, MustNewMembershipTestRepositories(db))
	if err != nil {
		panic(fmt.Sprintf("enrollment test rows: compose timetable: %v", err))
	}
	return withTimetableTestAudit(db, tt)
}

// NewEnrollmentCarePlanRows are the Care Plan rows of the enrollment suites.
func NewEnrollmentCarePlanRows(db transactionruntime.Database) TimetableCarePlanRows {
	return NewEnrollmentFlowCarePlanRows(db)
}

// EnrollmentAuditRows are the Audit Platform access log the enrollment
// suites read back.
type EnrollmentAuditRows struct {
	DataAccessLog auditModels.DataAccessLogRepository
}

// NewEnrollmentAuditRows composes the access log over the test database with
// its writes routed through command.
func NewEnrollmentAuditRows(db transactionruntime.Database, command auditModels.Command) *EnrollmentAuditRows {
	return &EnrollmentAuditRows{DataAccessLog: NewTestAuditPersistence(db, command).DataAccessLog}
}
