package api

import (
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	usersModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/services"
)

// StudentTestRepositories is the student suites' repository graph. The audit
// repositories are filled by NewStudentTestRepositories, the
// composition that owns Audit persistence (ADR 0049).
type StudentTestRepositories struct {
	// CarePlan is the owner capability the legacy adapters below delegate to;
	// Care Plan workflows composed by the test root share the same instance.
	CarePlan           careplan.Capability
	ParentRequestShare usersModels.ParentRequestShareEventRepository
	EnrollmentTestRepositories
	CareScheduleChangeRequest    timetableCompose.CareScheduleRequestRows
	ExcusedAbsenceRequest        *services.ExcusedAbsenceRequestRepository
	ParentRequestEvent           usersModels.ParentRequestEventRepository
	StudentDataChangeRequest     usersModels.StudentDataChangeRequestRepository
	FamilyProtection             usersModels.FamilyProtectionEventRepository
	EnrollmentOfferingAdjustment auditModels.EnrollmentOfferingAdjustmentRepository
	EnrollmentRestorationAudit   auditModels.EnrollmentRestorationRepository
	GuardianFinancialChange      auditModels.GuardianFinancialChangeCreator
	SubstitutionChange           auditModels.SubstitutionChangeCreator
	CareWithdrawal               usersModels.CareWithdrawalCompletionRepository
	StudentDeletionAudit         auditModels.StudentDeletionRepository
	StudentFieldEdit             auditModels.StudentFieldEditRepository
	StudentConsentChange         auditModels.StudentConsentChangeRepository
	DataDeletion                 auditModels.DataDeletionRepository
	ParentMessageThread          usersModels.ParentMessageThreadRepository
	ParentMessage                usersModels.ParentMessageRepository
}

func NewStudentTestRepositoriesWithMembers(db transactionruntime.Database, members services.MembershipTestRepositories) (StudentTestRepositories, error) {
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
		CareScheduleChangeRequest:  services.NewCareScheduleChangeRequestRepository(care),
		ExcusedAbsenceRequest:      services.NewExcusedAbsenceRequestRepository(care),
		ParentRequestEvent:         peopleCompose.NewParentRequestEventRepository(db.Pool()),
		StudentDataChangeRequest:   services.NewStudentDataChangeRequestRepository(care),
		FamilyProtection:           peopleCompose.NewFamilyProtectionEventRepository(db.Pool()),
		EnrollmentTestRepositories: enrollment,
		CareWithdrawal:             lifecycle.CareWithdrawal,
		ParentMessageThread:        parentStore.NewParentMessageThreadRepository(db.Pool(), NewMessageableGuardianRepository(db)),
		ParentMessage:              parentStore.NewParentMessageRepository(db.Pool()),
	}, nil
}
