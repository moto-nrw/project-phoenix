//go:build testhooks

package api

import (
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	owner0 "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// StudentTestRepositories is the student suites' repository graph. The audit
// repositories are filled by NewStudentTestRepositories, the
// composition that owns Audit persistence (ADR 0049).
type StudentTestRepositories struct {
	// CarePlan is the owner capability the legacy adapters below delegate to;
	// Care Plan workflows composed by the test root share the same instance.
	CarePlan           careplan.Capability
	ParentRequestShare peoplePorts.ParentRequestShareRecords
	EnrollmentTestRepositories
	CareScheduleChangeRequest    timetablePorts.CareScheduleRequestRows
	ExcusedAbsenceRequest        *ExcusedAbsenceRequestRepository
	ParentRequestEvent           owner0.ParentRequestEvents
	StudentDataChangeRequest     studentDataRequestRepository
	FamilyProtection             peoplePorts.FamilyProtectionRecords
	EnrollmentOfferingAdjustment auditModels.EnrollmentOfferingAdjustmentRepository
	EnrollmentRestorationAudit   auditModels.EnrollmentRestorationRepository
	GuardianFinancialChange      auditModels.GuardianFinancialChangeCreator
	SubstitutionChange           auditModels.SubstitutionChangeCreator
	CareWithdrawal               CareWithdrawalRecords
	StudentDeletionAudit         auditModels.StudentDeletionRepository
	StudentFieldEdit             auditModels.StudentFieldEditRepository
	StudentConsentChange         auditModels.StudentConsentChangeRepository
	DataDeletion                 auditModels.DataDeletionRepository
	ParentMessageThread          parentStore.ParentMessageThreadRepository
	ParentMessage                parentStore.ParentMessageRepository
}

func NewStudentTestRepositoriesWithMembers(db transactionruntime.Database, members MembershipTestRepositories) (StudentTestRepositories, error) {
	enrollment, err := NewEnrollmentTestRepositoriesWithMembers(db, members)
	if err != nil {
		return StudentTestRepositories{}, err
	}
	lifecycle, err := NewCareLifecycleTestRepositoriesWithMembers(db, members)
	if err != nil {
		return StudentTestRepositories{}, err
	}
	care, err := NewCarePlan(db, members.People, enrollment.InstanceStudent)
	if err != nil {
		return StudentTestRepositories{}, err
	}
	return StudentTestRepositories{
		CarePlan:                   care,
		ParentRequestShare:         peopleCompose.NewParentRequestShareEventRepository(db.Pool()),
		CareScheduleChangeRequest:  NewCareScheduleChangeRequestRepository(care),
		ExcusedAbsenceRequest:      NewExcusedAbsenceRequestRepository(care),
		ParentRequestEvent:         peopleCompose.NewParentRequestEventRepository(db.Pool()),
		StudentDataChangeRequest:   NewStudentDataChangeRequestRepository(care),
		FamilyProtection:           peopleCompose.NewFamilyProtectionEventRepository(db.Pool()),
		EnrollmentTestRepositories: enrollment,
		CareWithdrawal:             lifecycle.CareWithdrawal,
		ParentMessageThread:        parentStore.NewParentMessageThreadRepository(db.Pool(), newParentGuardianLister(db, NewPeopleIdentityAccess(db, nil).FindActiveSchoolMemberships)),
		ParentMessage:              parentStore.NewParentMessageRepository(db.Pool()),
	}, nil
}
