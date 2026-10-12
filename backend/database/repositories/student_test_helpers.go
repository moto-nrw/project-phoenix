package repositories

import (
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	"github.com/uptrace/bun"
)

// StudentTestRepositories is the student suites' repository graph. The audit
// repositories are filled by services.NewStudentTestRepositories, the
// composition that owns Audit persistence (ADR 0049).
type StudentTestRepositories struct {
	// CarePlan is the owner capability the legacy adapters below delegate to;
	// Care Plan workflows composed by the test root share the same instance.
	CarePlan           careplan.Capability
	ParentRequestShare usersModels.ParentRequestShareEventRepository
	EnrollmentTestRepositories
	CareScheduleChangeRequest    scheduleModels.CareScheduleChangeRequestRepository
	ExcusedAbsenceRequest        *ExcusedAbsenceRequestRepository
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

func NewStudentTestRepositories(db *bun.DB, members MembershipTestRepositories) (StudentTestRepositories, error) {
	enrollment, err := NewEnrollmentTestRepositories(db, members)
	if err != nil {
		return StudentTestRepositories{}, err
	}
	lifecycle, err := NewCareLifecycleTestRepositories(db, members)
	if err != nil {
		return StudentTestRepositories{}, err
	}
	care, err := NewCarePlan(db, members.People, enrollment.InstanceStudent)
	if err != nil {
		return StudentTestRepositories{}, err
	}
	r := &Factory{db: db}
	r.bindCarePlanAdapters(care)
	return StudentTestRepositories{
		CarePlan:                   care,
		ParentRequestShare:         usersRepo.NewParentRequestShareEventRepository(peopleRuntime(db)),
		CareScheduleChangeRequest:  r.CareScheduleChangeRequest,
		ExcusedAbsenceRequest:      r.ExcusedAbsenceRequest,
		ParentRequestEvent:         usersRepo.NewParentRequestEventRepository(peopleRuntime(db)),
		StudentDataChangeRequest:   r.StudentDataChangeRequest,
		FamilyProtection:           usersRepo.NewFamilyProtectionEventRepository(peopleRuntime(db)),
		EnrollmentTestRepositories: enrollment,
		CareWithdrawal:             lifecycle.CareWithdrawal,
		ParentMessageThread:        parentStore.NewParentMessageThreadRepository(db, NewMessageableGuardianRepository(db)),
		ParentMessage:              parentStore.NewParentMessageRepository(db),
	}, nil
}
