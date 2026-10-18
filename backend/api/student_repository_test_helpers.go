//go:build testhooks

package api

import (
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	owner0 "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// StudentTestRepositories are the student suites' Care Plan rows: the owner
// capability, which the Care Plan workflows composed by the test root share,
// and the request adapters over it. The Enrollment graph the suites read
// through comes beside it (NewStudentTestRepositories), the People
// Directory, Audit and Communication rows from NewStudentPeopleRows,
// NewStudentAuditRows and NewStudentMessageRows.
type StudentTestRepositories struct {
	CarePlan              careplan.Capability
	ExcusedAbsenceRequest *ExcusedAbsenceRequestRepository
}

// NewStudentTestRepositoriesWithMembers composes the student suites' Care
// Plan rows and returns them with the Enrollment graph over members they run
// beside.
func NewStudentTestRepositoriesWithMembers(db transactionruntime.Database, members MembershipTestRepositories) (StudentTestRepositories, EnrollmentTestRepositories, error) {
	enrollment, err := NewEnrollmentTestRepositoriesWithMembers(db, members)
	if err != nil {
		return StudentTestRepositories{}, EnrollmentTestRepositories{}, err
	}
	tt, err := NewTimetableTestRepositoriesWithMembers(db, members)
	if err != nil {
		return StudentTestRepositories{}, EnrollmentTestRepositories{}, err
	}
	care, err := NewCarePlan(db, members.People, tt.InstanceStudent)
	if err != nil {
		return StudentTestRepositories{}, EnrollmentTestRepositories{}, err
	}
	return StudentTestRepositories{
		CarePlan:              care,
		ExcusedAbsenceRequest: NewExcusedAbsenceRequestRepository(care),
	}, enrollment, nil
}

// StudentPeopleRows are the People Directory rows the student suites
// arrange and read back.
type StudentPeopleRows struct {
	ParentRequestShare peoplePorts.ParentRequestShareRecords
	ParentRequestEvent owner0.ParentRequestEvents
	FamilyProtection   peoplePorts.FamilyProtectionRecords
	CareWithdrawal     CareWithdrawalTestRows
}

// NewStudentPeopleRows composes the People Directory rows over db, with the
// care-withdrawal completions over the care lifecycle of members.
func NewStudentPeopleRows(db transactionruntime.Database, members MembershipTestRepositories) (StudentPeopleRows, error) {
	lifecycle, err := NewCareLifecycleTestRepositoriesWithMembers(db, members)
	if err != nil {
		return StudentPeopleRows{}, err
	}
	return StudentPeopleRows{
		ParentRequestShare: peopleCompose.NewParentRequestShareEventRepository(db.Pool()),
		ParentRequestEvent: peopleCompose.NewParentRequestEventRepository(db.Pool()),
		FamilyProtection:   peopleCompose.NewFamilyProtectionEventRepository(db.Pool()),
		CareWithdrawal:     lifecycle.CareWithdrawal(),
	}, nil
}

// StudentAuditRows are the Audit Platform trails the student suites write
// and read back.
type StudentAuditRows struct {
	EnrollmentOfferingAdjustment auditModels.EnrollmentOfferingAdjustmentRepository
	EnrollmentRestorationAudit   auditModels.EnrollmentRestorationRepository
	GuardianFinancialChange      auditModels.GuardianFinancialChangeCreator
	SubstitutionChange           auditModels.SubstitutionChangeCreator
	StudentDeletionAudit         auditModels.StudentDeletionRepository
	StudentFieldEdit             auditModels.StudentFieldEditRepository
	StudentConsentChange         auditModels.StudentConsentChangeRepository
	DataDeletion                 auditModels.DataDeletionRepository
}

// NewStudentAuditRows composes the trails over db with their writes routed
// through command, the composition that owns Audit persistence (ADR 0049).
func NewStudentAuditRows(db transactionruntime.Database, command auditModels.Command) StudentAuditRows {
	audits := NewTestAuditPersistence(db, command)
	return StudentAuditRows{
		EnrollmentOfferingAdjustment: audits.EnrollmentOfferingAdjustment,
		EnrollmentRestorationAudit:   audits.EnrollmentRestorationAudit,
		GuardianFinancialChange:      audits.GuardianFinancialChange,
		SubstitutionChange:           audits.SubstitutionChange,
		StudentDeletionAudit:         audits.StudentDeletionAudit,
		StudentFieldEdit:             audits.StudentFieldEdit,
		StudentConsentChange:         audits.StudentConsentChange,
		DataDeletion:                 audits.DataDeletion,
	}
}

// StudentMessageRows are the Communication owner's parent message rows the
// student suites arrange.
type StudentMessageRows struct {
	ParentMessageThread parentStore.ParentMessageThreadRepository
	ParentMessage       parentStore.ParentMessageRepository
}

// NewStudentMessageRows composes the parent message rows over db.
func NewStudentMessageRows(db transactionruntime.Database) StudentMessageRows {
	return StudentMessageRows{
		ParentMessageThread: parentStore.NewParentMessageThreadRepository(db.Pool(), newParentGuardianLister(db, NewPeopleIdentityAccess(db, nil).FindActiveSchoolMemberships)),
		ParentMessage:       parentStore.NewParentMessageRepository(db.Pool()),
	}
}
