package services

import (
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	"github.com/uptrace/bun"
)

// The test graphs below are the repository test sets of the legacy
// composition with the Audit Platform persistence filled in. The repository
// package no longer constructs Audit persistence; the Audit composition does,
// and this package is the legacy composition that may reach it (ADR 0049).
// Repositories routed through command append through the Audit command,
// exactly as the service graph's repositories do.

// NewTestAuditPersistence composes the Audit persistence over the test
// database: reads and appends go through the Audit handoff on the context
// with the database as the fallback. A non-nil command routes every write
// through it, as the service graph routes its repositories.
func NewTestAuditPersistence(db *bun.DB, command auditModels.Command) *auditCompose.Persistence {
	audits := auditCompose.MustNewPersistence(auditCompose.PersistenceDependencies{DB: db})
	if command != nil {
		audits = audits.WithRoutedWrites(command)
	}
	return audits
}

// NewRoutedTestAuditPersistence is NewTestAuditPersistence with every write
// routed through an Audit command over the test append store, the way the
// test service graph (NewFactoryForTests) routes its repositories.
func NewRoutedTestAuditPersistence(db *bun.DB) (*auditCompose.Persistence, error) {
	command, err := auditCompose.NewCommand(NewTestAuditStore(db), func(auditCompose.AppendObservation) {})
	if err != nil {
		return nil, err
	}
	return NewTestAuditPersistence(db, command), nil
}

// NewTestAuditStore composes the Audit append store over the test database
// for narrow test compositions.
func NewTestAuditStore(db *bun.DB) auditModels.AppendStore {
	return NewTestAuditPersistence(db, nil).AppendStore
}

// newFileEventTestRepository retains the Audit command write boundary while
// composing only the file-event query contract.
func newFileEventTestRepository(db *bun.DB, command auditModels.Command) auditModels.FileEventRepository {
	audits := NewTestAuditPersistence(db, command)
	return audits.FileEvent
}

// NewDataDeletionTestRepository composes the deletion ledger with its writes
// routed through command.
func NewDataDeletionTestRepository(db *bun.DB, command auditModels.Command) auditModels.DataDeletionRepository {
	audits := NewTestAuditPersistence(db, command)
	return audits.DataDeletion
}

// NewTimetableTestRepositories composes the Timetable test graph with its
// deviation protocol.
func NewTimetableTestRepositories(db *bun.DB, clocks ...func() time.Time) (TimetableTestRepositories, error) {
	result, err := NewTimetableTestRepositoriesWithMembers(db, MustNewMembershipTestRepositories(db), clocks...)
	if err != nil {
		return TimetableTestRepositories{}, err
	}
	return withTimetableTestAudit(db, result), nil
}

func withTimetableTestAudit(db *bun.DB, timetable TimetableTestRepositories) TimetableTestRepositories {
	timetable.DeviationEvent = NewTestAuditPersistence(db, nil).DeviationEvent
	return timetable
}

// NewWorkSessionTestRepositories composes the work-session test graph with its
// edit trail.
func NewWorkSessionTestRepositories(db *bun.DB, clocks ...func() time.Time) (WorkSessionTestRepositories, error) {
	result, err := NewWorkSessionTestRepositoriesWithMembers(db, MustNewMembershipTestRepositories(db), clocks...)
	if err != nil {
		return WorkSessionTestRepositories{}, err
	}
	return withWorkSessionTestAudit(db, result), nil
}

func withWorkSessionTestAudit(db *bun.DB, sessions WorkSessionTestRepositories) WorkSessionTestRepositories {
	sessions.TimetableTestRepositories = withTimetableTestAudit(db, sessions.TimetableTestRepositories)
	sessions.WorkSessionEdit = NewTestAuditPersistence(db, nil).WorkSessionEdit
	return sessions
}

// NewWorkforceTestRepositories composes the Workforce test graph with its
// audit trails.
func NewWorkforceTestRepositories(db *bun.DB, command auditModels.Command, clocks ...func() time.Time) (WorkforceTestRepositories, error) {
	result, err := NewWorkforceTestRepositoriesWithMembers(db, MustNewMembershipTestRepositories(db), clocks...)
	if err != nil {
		return WorkforceTestRepositories{}, err
	}
	result.WorkSessionTestRepositories = withWorkSessionTestAudit(db, result.WorkSessionTestRepositories)
	audits := NewTestAuditPersistence(db, command)
	result.TimeTrackingDeletion, result.TimeTrackingAuditLog = audits.TimeTrackingDeletion, audits.TimeTrackingAuditLog
	result.PersonnelNumberChange, result.StaffMasterDataChange = audits.PersonnelNumberChange, audits.StaffMasterDataChange
	result.DataAccessLog, result.DataDeletion = audits.DataAccessLog, audits.DataDeletion
	return result, nil
}

// NewEnrollmentTestRepositories composes the enrollment test graph with its
// access log.
func NewEnrollmentTestRepositories(db *bun.DB, command auditModels.Command) (EnrollmentTestRepositories, error) {
	result, err := NewEnrollmentTestRepositoriesWithMembers(db, MustNewMembershipTestRepositories(db))
	if err != nil {
		return EnrollmentTestRepositories{}, err
	}
	return withEnrollmentTestAudit(db, command, result), nil
}

func withEnrollmentTestAudit(db *bun.DB, command auditModels.Command, enrollment EnrollmentTestRepositories) EnrollmentTestRepositories {
	enrollment.TimetableTestRepositories = withTimetableTestAudit(db, enrollment.TimetableTestRepositories)
	enrollment.DataAccessLog = NewTestAuditPersistence(db, command).DataAccessLog
	return enrollment
}

// NewEnrollmentFlowTestRepositories composes the enrollment flow suites'
// repositories with the audit repositories the flows read and write.
func NewEnrollmentFlowTestRepositories(db *bun.DB) (*EnrollmentFlowTestRepositories, error) {
	result, err := NewEnrollmentFlowTestRepositoriesWithMembers(db, MustNewMembershipTestRepositories(db))
	if err != nil {
		return nil, err
	}
	result.TimetableTestRepositories = withTimetableTestAudit(db, result.TimetableTestRepositories)
	audits := NewTestAuditPersistence(db, nil)
	result.EnrollmentOfferingAdjustment = audits.EnrollmentOfferingAdjustment
	result.EnrollmentRestorationAudit = audits.EnrollmentRestorationAudit
	result.EnrollmentDeletionAudit = audits.EnrollmentDeletionAudit
	result.StudentFieldEdit = audits.StudentFieldEdit
	return result, nil
}

// NewCareLifecycleTestRepositories composes the care-lifecycle test graph with
// its field-edit trail.
func NewCareLifecycleTestRepositories(db *bun.DB, command auditModels.Command) (CareLifecycleTestRepositories, error) {
	result, err := NewCareLifecycleTestRepositoriesWithMembers(db, MustNewMembershipTestRepositories(db))
	if err != nil {
		return CareLifecycleTestRepositories{}, err
	}
	result.TimetableTestRepositories = withTimetableTestAudit(db, result.TimetableTestRepositories)
	audits := NewTestAuditPersistence(db, command)
	result.StudentFieldEdit = audits.StudentFieldEdit
	return result, nil
}

// NewStudentTestRepositories composes the student test graph with its audit
// trails.
func NewStudentTestRepositories(db *bun.DB, command auditModels.Command) (StudentTestRepositories, error) {
	result, err := NewStudentTestRepositoriesWithMembers(db, MustNewMembershipTestRepositories(db))
	if err != nil {
		return StudentTestRepositories{}, err
	}
	result.EnrollmentTestRepositories = withEnrollmentTestAudit(db, command, result.EnrollmentTestRepositories)
	audits := NewTestAuditPersistence(db, command)
	result.EnrollmentOfferingAdjustment = audits.EnrollmentOfferingAdjustment
	result.EnrollmentRestorationAudit = audits.EnrollmentRestorationAudit
	result.GuardianFinancialChange = audits.GuardianFinancialChange
	result.SubstitutionChange = audits.SubstitutionChange
	result.StudentDeletionAudit = audits.StudentDeletionAudit
	result.StudentFieldEdit = audits.StudentFieldEdit
	result.StudentConsentChange = audits.StudentConsentChange
	result.DataDeletion = audits.DataDeletion
	return result, nil
}

// NewStatisticsTestRepositories composes the statistics test graph with its
// access log.
func NewStatisticsTestRepositories(db *bun.DB, command auditModels.Command, clocks ...func() time.Time) (StatisticsTestRepositories, error) {
	result, err := NewStatisticsTestRepositoriesWithMembers(db, MustNewMembershipTestRepositories(db), clocks...)
	if err != nil {
		return StatisticsTestRepositories{}, err
	}
	result.Timetable = withTimetableTestAudit(db, result.Timetable)
	audits := NewTestAuditPersistence(db, command)
	result.AccessLog = audits.DataAccessLog
	return result, nil
}

// NewGuardianTestRepositories composes the guardian test repositories with
// their financial-change and access trails.
func NewGuardianTestRepositories(db *bun.DB, command auditModels.Command) GuardianTestRepositories {
	result := NewGuardianTestRepositoriesWithoutAudit(db)
	audits := NewTestAuditPersistence(db, command)
	result.FinancialAudit, result.AccessLog = audits.GuardianFinancialChange, audits.DataAccessLog
	return result
}

// NewStudentConsents composes unobserved owners behind the consent seam.
func NewStudentConsents(db *bun.DB) *StudentConsents {
	return NewStudentConsentsFor(
		MustNewPeopleDirectory(db),
		NewTestAuditPersistence(db, nil).StudentConsentChange,
	)
}
