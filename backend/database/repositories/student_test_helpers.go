package repositories

import (
	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	activeModels "github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
	"github.com/uptrace/bun"
)

type StudentTestRepositories struct {
	// CarePlan is the owner capability the legacy adapters below delegate to;
	// Care Plan workflows composed by the test root share the same instance.
	CarePlan           careplan.Capability
	ParentRequestShare usersModels.ParentRequestShareEventRepository
	EnrollmentTestRepositories
	CareScheduleChangeRequest    scheduleModels.CareScheduleChangeRequestRepository
	ExcusedAbsenceRequest        activeModels.ExcusedAbsenceRequestRepository
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

func NewStudentTestRepositories(db *bun.DB, command auditModels.Command) (StudentTestRepositories, error) {
	enrollment, err := NewEnrollmentTestRepositories(db, command)
	if err != nil {
		return StudentTestRepositories{}, err
	}
	lifecycle, err := NewCareLifecycleTestRepositories(db, command)
	if err != nil {
		return StudentTestRepositories{}, err
	}
	people, err := NewPeopleDirectory(db)
	if err != nil {
		return StudentTestRepositories{}, err
	}
	care, err := NewCarePlan(db, people, enrollment.InstanceStudent)
	if err != nil {
		return StudentTestRepositories{}, err
	}
	appointments, err := NewAppointments(db)
	if err != nil {
		return StudentTestRepositories{}, err
	}
	r := &Factory{db: db,
		ParentRequestEvent:           usersRepo.NewParentRequestEventRepository(db),
		FamilyProtection:             usersRepo.NewFamilyProtectionEventRepository(db),
		EnrollmentOfferingAdjustment: auditRepo.NewEnrollmentOfferingAdjustmentRepository(newTestAuditRuntime(db)),
		EnrollmentRestorationAudit:   auditRepo.NewEnrollmentRestorationRepository(newTestAuditRuntime(db)),
		GuardianFinancialChange:      auditRepo.NewGuardianFinancialChangeRepository(newTestAuditRuntime(db)),
		SubstitutionChange:           auditRepo.NewSubstitutionChangeRepository(newTestAuditRuntime(db)),
		StudentDeletionAudit:         auditRepo.NewStudentDeletionRepository(newTestAuditRuntime(db)),
		StudentConsentChange:         auditRepo.NewStudentConsentChangeRepository(newTestAuditRuntime(db)),
		DataDeletion:                 auditRepo.NewDataDeletionRepository(newTestAuditRuntime(db)),
	}
	r.BindPeopleDirectory(people)
	r.bindCarePlanAdapters(care)
	r.BindAppointments(appointments)
	r.RouteAuditWrites(command)
	return StudentTestRepositories{
		CarePlan:                     care,
		ParentRequestShare:           usersRepo.NewParentRequestShareEventRepository(db),
		CareScheduleChangeRequest:    r.CareScheduleChangeRequest,
		ExcusedAbsenceRequest:        r.ExcusedAbsenceRequest,
		ParentRequestEvent:           r.ParentRequestEvent,
		StudentDataChangeRequest:     r.StudentDataChangeRequest,
		FamilyProtection:             r.FamilyProtection,
		EnrollmentOfferingAdjustment: r.EnrollmentOfferingAdjustment,
		EnrollmentRestorationAudit:   r.EnrollmentRestorationAudit,
		GuardianFinancialChange:      r.GuardianFinancialChange,
		SubstitutionChange:           r.SubstitutionChange,
		EnrollmentTestRepositories:   enrollment,
		CareWithdrawal:               lifecycle.CareWithdrawal,
		StudentDeletionAudit:         r.StudentDeletionAudit, StudentFieldEdit: lifecycle.StudentFieldEdit,
		StudentConsentChange: r.StudentConsentChange, DataDeletion: r.DataDeletion,
		ParentMessageThread: parentStore.NewParentMessageThreadRepository(db, NewMessageableGuardianRepository(db)),
		ParentMessage:       parentStore.NewParentMessageRepository(db),
	}, nil
}
