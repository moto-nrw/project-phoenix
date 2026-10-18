//go:build testhooks

package api

import (
	"time"

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

// NewTestAuditPersistence composes the Audit persistence over the test
// database: reads and appends go through the Audit handoff on the context
// with the database as the fallback. A non-nil command routes every write
// through it, as the service graph routes its repositories.
func NewTestAuditPersistence(db transactionruntime.Database, command auditModels.Command) *auditCompose.Persistence {
	audits := auditCompose.MustNewPersistence(auditCompose.PersistenceDependencies{DB: db.Pool()})
	if command != nil {
		audits = audits.WithRoutedWrites(command)
	}
	return audits
}

// NewRoutedTestAuditPersistence is NewTestAuditPersistence with every write
// routed through an Audit command over the test append store, the way the
// test service graph (NewServiceGraphForTests) routes its repositories.
func NewRoutedTestAuditPersistence(db transactionruntime.Database) (*auditCompose.Persistence, error) {
	command, err := auditCompose.NewCommand(NewTestAuditStore(db), func(auditCompose.AppendObservation) {})
	if err != nil {
		return nil, err
	}
	return NewTestAuditPersistence(db, command), nil
}

// NewTestAuditStore composes the Audit append store over the test database
// for narrow test compositions.
func NewTestAuditStore(db transactionruntime.Database) auditModels.AppendStore {
	return NewTestAuditPersistence(db, nil).AppendStore
}

// newFileEventTestRepository retains the Audit command write boundary while
// composing only the file-event query contract.
func newFileEventTestRepository(db transactionruntime.Database, command auditModels.Command) auditModels.FileEventRepository {
	audits := NewTestAuditPersistence(db, command)
	return audits.FileEvent
}

// NewDataDeletionTestRepository composes the deletion ledger with its writes
// routed through command.
func NewDataDeletionTestRepository(db transactionruntime.Database, command auditModels.Command) auditModels.DataDeletionRepository {
	audits := NewTestAuditPersistence(db, command)
	return audits.DataDeletion
}

// NewTimetableTestRepositories composes the Timetable test graph with its
// deviation protocol.
func NewTimetableTestRepositories(db transactionruntime.Database, clocks ...func() time.Time) (TimetableTestRepositories, error) {
	result, err := NewTimetableTestRepositoriesWithMembers(db, MustNewMembershipTestRepositories(db), clocks...)
	if err != nil {
		return TimetableTestRepositories{}, err
	}
	return withTimetableTestAudit(db, result), nil
}

func withTimetableTestAudit(db transactionruntime.Database, timetable TimetableTestRepositories) TimetableTestRepositories {
	return timetable.withDeviationAudit(NewTestAuditPersistence(db, nil).DeviationEvent)
}

// NewWorkforceTestRepositories composes the Workforce test graph over the
// members of the test database.
func NewWorkforceTestRepositories(db transactionruntime.Database, clocks ...func() time.Time) (WorkforceTestRepositories, error) {
	return NewWorkforceTestRepositoriesWithMembers(db, MustNewMembershipTestRepositories(db), clocks...)
}

// NewEnrollmentTestRepositories composes the enrollment test graph with its
// access log.
func NewEnrollmentTestRepositories(db transactionruntime.Database, command auditModels.Command) (EnrollmentTestRepositories, error) {
	result, err := NewEnrollmentTestRepositoriesWithMembers(db, MustNewMembershipTestRepositories(db))
	if err != nil {
		return EnrollmentTestRepositories{}, err
	}
	return withEnrollmentTestAudit(db, command, result), nil
}

func withEnrollmentTestAudit(db transactionruntime.Database, command auditModels.Command, enrollment EnrollmentTestRepositories) EnrollmentTestRepositories {
	enrollment.TimetableTestRepositories = withTimetableTestAudit(db, enrollment.TimetableTestRepositories)
	enrollment.TimetableDeviationAudit = NewTimetableDeviationAudit(db)
	enrollment.DataAccessLog = NewTestAuditPersistence(db, command).DataAccessLog
	return enrollment
}

// NewEnrollmentFlowTestRepositories composes the enrollment flow suites'
// repositories with the audit repositories the flows read and write.
func NewEnrollmentFlowTestRepositories(db transactionruntime.Database) (*EnrollmentFlowTestRepositories, error) {
	return NewEnrollmentFlowTestRepositoriesWithMembers(db, MustNewMembershipTestRepositories(db))
}

// NewCareLifecycleTestRepositories composes the care-lifecycle test graph with
// its field-edit trail.
func NewCareLifecycleTestRepositories(db transactionruntime.Database, command auditModels.Command) (CareLifecycleTestRepositories, error) {
	result, err := NewCareLifecycleTestRepositoriesWithMembers(db, MustNewMembershipTestRepositories(db))
	if err != nil {
		return CareLifecycleTestRepositories{}, err
	}
	result.TimetableTestRepositories = withTimetableTestAudit(db, result.TimetableTestRepositories)
	result.TimetableDeviationAudit = NewTimetableDeviationAudit(db)
	audits := NewTestAuditPersistence(db, command)
	result.StudentFieldEdit = audits.StudentFieldEdit
	return result, nil
}

// NewStudentTestRepositories composes the student suites' Care Plan rows
// and returns them with the Enrollment graph and its access log. The audit
// trails the suites read back come from NewStudentAuditRows.
func NewStudentTestRepositories(db transactionruntime.Database, command auditModels.Command) (StudentTestRepositories, EnrollmentTestRepositories, error) {
	result, enrollment, err := NewStudentTestRepositoriesWithMembers(db, MustNewMembershipTestRepositories(db))
	if err != nil {
		return StudentTestRepositories{}, EnrollmentTestRepositories{}, err
	}
	return result, withEnrollmentTestAudit(db, command, enrollment), nil
}

// NewStudentConsents composes unobserved owners behind the consent seam.
func NewStudentConsents(db transactionruntime.Database) *StudentConsents {
	return NewStudentConsentsFor(
		MustNewPeopleDirectory(db),
		NewTestAuditPersistence(db, nil).StudentConsentChange,
	)
}
