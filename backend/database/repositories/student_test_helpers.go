package repositories

import (
	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/uptrace/bun"
)

type StudentTestRepositories struct {
	ParentRequestShare usersModels.ParentRequestShareEventRepository
	EnrollmentTestRepositories
	CareScheduleChangeRequest    scheduleModels.CareScheduleChangeRequestRepository
	ExcusedAbsenceRequest        activeModels.ExcusedAbsenceRequestRepository
	OfferingChangeRequest        enrollmentModels.OfferingChangeRequestRepository
	ParentRequestEvent           usersModels.ParentRequestEventRepository
	StudentDataChangeRequest     usersModels.StudentDataChangeRequestRepository
	FamilyProtection             usersModels.FamilyProtectionEventRepository
	StudentDocument              usersModels.StudentDocumentRepository
	EnrollmentOfferingAdjustment auditModels.EnrollmentOfferingAdjustmentRepository
	EnrollmentRestorationAudit   auditModels.EnrollmentRestorationRepository
	GuardianFinancialChange      auditModels.GuardianFinancialChangeCreator
	SubstitutionChange           auditModels.SubstitutionChangeCreator
	CareExit                     usersModels.CareExitRepository
	CareExitCleanup              usersModels.CareExitCleanupRepository
	CareWithdrawal               usersModels.CareWithdrawalCompletionRepository
	GradeTransition              educationModels.GradeTransitionRepository
	PrivacyConsent               usersModels.PrivacyConsentRepository
	StudentDeletion              usersModels.StudentDeletionRepository
	StudentDeletionAudit         auditModels.StudentDeletionRepository
	StudentFieldEdit             auditModels.StudentFieldEditRepository
	StudentConsentChange         auditModels.StudentConsentChangeRepository
	DataDeletion                 auditModels.DataDeletionRepository
	ParentMessageThread          usersModels.ParentMessageThreadRepository
	ParentMessage                usersModels.ParentMessageRepository
	Attendance                   activeModels.AttendanceRepository
}

func (r StudentTestRepositories) BindTimetable(capability timetable.Capability) {
	r.CareExitCleanup.(*usersRepo.CareExitCleanupRepository).BindActivityBookings(activityBookingDirectory{capability: capability})
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
		PrivacyConsent:               activeRepo.NewPrivacyConsentRepository(db),
		StudentDeletionAudit:         auditRepo.NewStudentDeletionRepository(newTestAuditRuntime(db)),
		StudentConsentChange:         auditRepo.NewStudentConsentChangeRepository(newTestAuditRuntime(db)),
		DataDeletion:                 auditRepo.NewDataDeletionRepository(newTestAuditRuntime(db)),
		CareExitCleanup:              lifecycle.CareExitCleanup,
	}
	r.StudentDeletion = usersRepo.NewStudentDeletionRepository(db, r.StudentDeletionAudit.CountStudentReferences, r.countPrivacyConsents, enrollmentCompose.New().CountStudentReferences)
	r.BindPeopleDirectory(people)
	r.bindCarePlanAdapters(care)
	r.BindAppointments(appointments)
	r.CareExitCleanup.(*usersRepo.CareExitCleanupRepository).BindActivityBookings(activityBookingDirectory{capability: enrollment.Timetable})
	r.RouteAuditWrites(command)
	return StudentTestRepositories{
		ParentRequestShare:           usersRepo.NewParentRequestShareEventRepository(db),
		CareScheduleChangeRequest:    r.CareScheduleChangeRequest,
		ExcusedAbsenceRequest:        r.ExcusedAbsenceRequest,
		OfferingChangeRequest:        r.OfferingChangeRequest,
		ParentRequestEvent:           r.ParentRequestEvent,
		StudentDataChangeRequest:     r.StudentDataChangeRequest,
		FamilyProtection:             r.FamilyProtection,
		StudentDocument:              r.StudentDocument,
		EnrollmentOfferingAdjustment: r.EnrollmentOfferingAdjustment,
		EnrollmentRestorationAudit:   r.EnrollmentRestorationAudit,
		GuardianFinancialChange:      r.GuardianFinancialChange,
		SubstitutionChange:           r.SubstitutionChange,
		EnrollmentTestRepositories:   enrollment, CareExit: lifecycle.CareExit, CareExitCleanup: lifecycle.CareExitCleanup,
		CareWithdrawal: lifecycle.CareWithdrawal, GradeTransition: lifecycle.GradeTransition, PrivacyConsent: r.PrivacyConsent,
		StudentDeletion: r.StudentDeletion, StudentDeletionAudit: r.StudentDeletionAudit, StudentFieldEdit: lifecycle.StudentFieldEdit,
		StudentConsentChange: r.StudentConsentChange, DataDeletion: r.DataDeletion,
		ParentMessageThread: usersRepo.NewParentMessageThreadRepository(db), ParentMessage: usersRepo.NewParentMessageRepository(db),
		Attendance: activeRepo.NewAttendanceRepository(db)}, nil
}
