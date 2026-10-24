//go:build testhooks

package api

import (
	"time"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// The test graphs below are the repository test sets of the root with the
// Audit Platform persistence filled in. The repository package no longer
// constructs Audit persistence; the Audit composition does, and the root may
// reach it (ADR 0049, ADR 0053).
// Repositories routed through command append through the Audit command,
// exactly as the service graph's repositories do.

// NewRoutedTestAuditPersistence is NewTestAuditPersistence with every write
// routed through an Audit command over the test append store, the way the
// test service graph (NewServiceGraphForTests) routes its repositories.
func NewRoutedTestAuditPersistence(db transactionruntime.Database) (*auditCompose.Persistence, error) {
	command, err := auditCompose.NewCommand(owners.NewTestAuditStore(db), func(auditCompose.AppendObservation) {})
	if err != nil {
		return nil, err
	}
	return owners.NewTestAuditPersistence(db, command), nil
}

// newFileEventTestRepository retains the Audit command write boundary while
// composing only the file-event query contract.
func newFileEventTestRepository(db transactionruntime.Database, command auditModels.Command) auditModels.FileEventRepository {
	audits := owners.NewTestAuditPersistence(db, command)
	return audits.FileEvent
}

// NewDataDeletionTestRepository composes the deletion ledger with its writes
// routed through command.
func NewDataDeletionTestRepository(db transactionruntime.Database, command auditModels.Command) auditModels.DataDeletionRepository {
	audits := owners.NewTestAuditPersistence(db, command)
	return audits.DataDeletion
}

// NewTimetableTestRepositories composes the Timetable test graph with its
// deviation protocol.
func NewTimetableTestRepositories(db transactionruntime.Database, clocks ...func() time.Time) (owners.TimetableTestRepositories, error) {
	result, err := owners.NewTimetableTestRepositoriesWithMembers(db, owners.MustNewMembershipTestRepositories(db), clocks...)
	if err != nil {
		return owners.TimetableTestRepositories{}, err
	}
	return withTimetableTestAudit(db, result), nil
}

func withTimetableTestAudit(db transactionruntime.Database, timetable owners.TimetableTestRepositories) owners.TimetableTestRepositories {
	return timetable.WithDeviationAudit(owners.NewTestAuditPersistence(db, nil).DeviationEvent)
}

// NewWorkforceTestRepositories composes the Workforce test graph over the
// members of the test database.
func NewWorkforceTestRepositories(db transactionruntime.Database, clocks ...func() time.Time) (WorkforceTestRepositories, error) {
	return NewWorkforceTestRepositoriesWithMembers(db, owners.MustNewMembershipTestRepositories(db), clocks...)
}

// NewEnrollmentTestRepositories composes the enrollment suites' People
// Directory rows; the access log recording into command comes from
// NewEnrollmentAuditRows.
func NewEnrollmentTestRepositories(db transactionruntime.Database, _ auditModels.Command) (EnrollmentTestRepositories, error) {
	return NewEnrollmentTestRepositoriesWithMembers(db, owners.MustNewMembershipTestRepositories(db))
}

// NewCareLifecycleTestRepositories composes the care-lifecycle test graph.
func NewCareLifecycleTestRepositories(db transactionruntime.Database, _ auditModels.Command) (CareLifecycleTestRepositories, error) {
	return NewCareLifecycleTestRepositoriesWithMembers(db, owners.MustNewMembershipTestRepositories(db))
}

// NewStudentTestRepositories composes the student suites' Care Plan rows
// and returns them with the Enrollment graph. The audit trails the suites
// read back come from NewStudentAuditRows, the access log recording into
// the suite's command from NewEnrollmentAuditRows.
func NewStudentTestRepositories(db transactionruntime.Database, _ auditModels.Command) (StudentTestRepositories, EnrollmentTestRepositories, error) {
	result, enrollment, err := NewStudentTestRepositoriesWithMembers(db, owners.MustNewMembershipTestRepositories(db))
	if err != nil {
		return StudentTestRepositories{}, EnrollmentTestRepositories{}, err
	}
	return result, enrollment, nil
}

// NewStudentConsents composes unobserved owners behind the consent seam.
func NewStudentConsents(db transactionruntime.Database) *owners.StudentConsents {
	return owners.NewStudentConsentsFor(
		owners.MustNewPeopleDirectory(db),
		owners.NewTestAuditPersistence(db, nil).StudentConsentChange,
	)
}
