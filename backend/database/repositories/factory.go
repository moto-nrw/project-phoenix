package repositories

import (
	"github.com/moto-nrw/project-phoenix/database/repositories/active"
	"github.com/moto-nrw/project-phoenix/database/repositories/activities"
	"github.com/moto-nrw/project-phoenix/database/repositories/audit"
	"github.com/moto-nrw/project-phoenix/database/repositories/auth"
	calendarRepo "github.com/moto-nrw/project-phoenix/database/repositories/calendar"
	"github.com/moto-nrw/project-phoenix/database/repositories/config"
	displayRepo "github.com/moto-nrw/project-phoenix/database/repositories/display"
	"github.com/moto-nrw/project-phoenix/database/repositories/education"
	"github.com/moto-nrw/project-phoenix/database/repositories/enrollment"
	"github.com/moto-nrw/project-phoenix/database/repositories/facilities"
	"github.com/moto-nrw/project-phoenix/database/repositories/feedback"
	"github.com/moto-nrw/project-phoenix/database/repositories/filestore"
	"github.com/moto-nrw/project-phoenix/database/repositories/iot"
	mealplanRepo "github.com/moto-nrw/project-phoenix/database/repositories/mealplan"
	parentRepo "github.com/moto-nrw/project-phoenix/database/repositories/parent"
	platformRepo "github.com/moto-nrw/project-phoenix/database/repositories/platform"
	"github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	"github.com/moto-nrw/project-phoenix/database/repositories/users"
	filestoreModels "github.com/moto-nrw/project-phoenix/models/filestore"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	calendarModels "github.com/moto-nrw/project-phoenix/models/calendar"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	displayModels "github.com/moto-nrw/project-phoenix/models/display"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	facilityModels "github.com/moto-nrw/project-phoenix/models/facilities"
	feedbackModels "github.com/moto-nrw/project-phoenix/models/feedback"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	mealplanModels "github.com/moto-nrw/project-phoenix/models/mealplan"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"

	"github.com/uptrace/bun"
)

// Factory provides access to all repositories
type Factory struct {
	// Auth domain
	Account                authModels.AccountRepository
	AccountParent          authModels.AccountParentRepository
	AccountTenant          authModels.AccountTenantRepository
	StaffCalendarFeedToken authModels.StaffCalendarFeedTokenRepository
	Role                   authModels.RoleRepository
	Permission             authModels.PermissionRepository
	RolePermission         authModels.RolePermissionRepository
	AccountRole            authModels.AccountRoleRepository
	AccountPermission      authModels.AccountPermissionRepository
	Token                  authModels.TokenRepository
	PasswordResetToken     authModels.PasswordResetTokenRepository
	PasswordResetRateLimit authModels.PasswordResetRateLimitRepository
	InvitationToken        authModels.InvitationTokenRepository
	GuardianInvitation     authModels.GuardianInvitationRepository
	MFACredential          authModels.MFACredentialRepository
	MFAEmailChallenge      authModels.MFAEmailChallengeRepository
	MFATrustedDevice       authModels.MFATrustedDeviceRepository
	MFAOverride            authModels.MFAOverrideRepository
	PasskeyCredential      authModels.PasskeyCredentialRepository
	PasskeySession         authModels.PasskeySessionRepository

	// Users domain
	Person              userModels.PersonRepository
	RFIDCard            userModels.RFIDCardRepository
	Staff               userModels.StaffRepository
	Student             userModels.StudentRepository
	ClassListEntry      userModels.ClassListEntryRepository
	StudentDeletion     userModels.StudentDeletionRepository
	CareExit            userModels.CareExitRepository
	CareExitCleanup     userModels.CareExitCleanupRepository
	CareWithdrawal      userModels.CareWithdrawalCompletionRepository
	Teacher             userModels.TeacherRepository
	Guest               userModels.GuestRepository
	Profile             userModels.ProfileRepository
	StudentGuardian     userModels.StudentGuardianRepository
	StudentCompanion    userModels.StudentCompanionRepository
	GuardianProfile     userModels.GuardianProfileRepository
	GuardianPhoneNumber userModels.GuardianPhoneNumberRepository
	PrivacyConsent      userModels.PrivacyConsentRepository
	FamilyProtection    userModels.FamilyProtectionEventRepository
	ParentRequestShare  userModels.ParentRequestShareEventRepository
	ParentRequestEvent  userModels.ParentRequestEventRepository

	// Staff Stammdaten (#1423)
	StaffMasterData    userModels.StaffMasterDataRepository
	StaffQualification userModels.StaffQualificationRepository
	StaffFinancialData userModels.StaffFinancialDataRepository

	// Guardian payment data (#2608)
	GuardianFinancialData userModels.GuardianFinancialDataRepository

	// Staff documents (#1424)
	StaffDocument   userModels.StaffDocumentRepository
	StudentDocument userModels.StudentDocumentRepository

	// School file storage (#2596)
	FileFolder filestoreModels.FolderRepository
	File       filestoreModels.FileRepository
	FileEvent  auditModels.FileEventRepository

	NotificationPreference userModels.NotificationPreferenceRepository

	// Facilities domain
	Room facilityModels.RoomRepository

	// Education domain
	Group             educationModels.GroupRepository
	GroupTeacher      educationModels.GroupTeacherRepository
	ClassTeacher      educationModels.ClassTeacherRepository
	ClassArrivalTime  educationModels.ClassArrivalTimeRepository
	GroupSubstitution educationModels.GroupSubstitutionRepository
	GradeTransition   educationModels.GradeTransitionRepository

	// Schedule domain
	Dateframe                 scheduleModels.DateframeRepository
	Timeframe                 scheduleModels.TimeframeRepository
	RecurrenceRule            scheduleModels.RecurrenceRuleRepository
	StudentPickupSchedule     scheduleModels.StudentPickupScheduleRepository
	StudentPickupException    scheduleModels.StudentPickupExceptionRepository
	StudentPickupNote         scheduleModels.StudentPickupNoteRepository
	StudentArrivalSchedule    scheduleModels.StudentArrivalScheduleRepository
	StudentArrivalException   scheduleModels.StudentArrivalExceptionRepository
	StudentArrivalNote        scheduleModels.StudentArrivalNoteRepository
	CareScheduleChangeRequest scheduleModels.CareScheduleChangeRequestRepository
	StaffShift                scheduleModels.StaffShiftRepository
	StaffShiftSeries          scheduleModels.StaffShiftSeriesRepository
	StaffShiftSeriesException scheduleModels.StaffShiftSeriesExceptionRepository
	ShiftType                 scheduleModels.ShiftTypeRepository
	PlanningTrack             scheduleModels.PlanningTrackRepository
	TimetableConflictAck      scheduleModels.TimetableConflictAckRepository
	CalendarPeriod            scheduleModels.CalendarPeriodRepository
	ClosingDay                scheduleModels.ClosingDayRepository
	ActivityInstance          scheduleModels.ActivityInstanceRepository
	InstanceIdempotency       scheduleModels.InstanceIdempotencyRepository
	InstanceStaff             scheduleModels.InstanceStaffRepository
	InstanceStudent           scheduleModels.InstanceStudentRepository
	ActivityException         scheduleModels.ActivityExceptionRepository

	// Activities domain
	ActivityGroup      activitiesModels.GroupRepository
	ActivityCategory   activitiesModels.CategoryRepository
	ActivitySchedule   activitiesModels.ScheduleRepository
	ActivitySupervisor activitiesModels.SupervisorPlannedRepository
	StudentEnrollment  activitiesModels.StudentEnrollmentRepository

	// Active domain
	ActiveGroup      activeModels.GroupRepository
	ActiveVisit      activeModels.VisitRepository
	GroupSupervisor  activeModels.GroupSupervisorRepository
	CombinedGroup    activeModels.CombinedGroupRepository
	GroupMapping     activeModels.GroupMappingRepository
	Attendance       activeModels.AttendanceRepository
	StudentStatusDay activeModels.StudentStatusDayOverviewRepository
	// Statistics serves the aggregate reads of the Statistik page (#2606).
	Statistics            activeModels.StatisticsRepository
	ExcusedAbsenceRequest activeModels.ExcusedAbsenceRequestRepository
	WorkSession           activeModels.WorkSessionRepository
	WorkSessionBreak      activeModels.WorkSessionBreakRepository
	StaffAbsence          activeModels.StaffAbsenceRepository
	StaffAbsenceAudit     activeModels.StaffAbsenceAuditRepository
	StaffAbsenceType      activeModels.StaffAbsenceTypeRepository
	StaffVacationQuota    activeModels.StaffVacationQuotaRepository
	StaffVacationOpening  activeModels.StaffVacationOpeningRepository
	StaffBalanceAdjust    activeModels.StaffBalanceAdjustmentRepository
	StaffMonthSnapshot    activeModels.StaffMonthBalanceSnapshotRepository

	// Meal plan domain
	MealPlanEntry mealplanModels.MealPlanEntryRepository

	// Feedback domain
	FeedbackEntry feedbackModels.EntryRepository

	// IoT domain
	Device             iotModels.DeviceRepository
	PushSubscription   iotModels.PushSubscriptionRepository
	PWAStandaloneUsage *iot.PWAStandaloneUsageRepository

	// Config domain
	SettingValue      configModels.SettingValueRepository
	SettingAudit      configModels.SettingAuditRepository
	StaffWorkSchedule configModels.StaffWorkScheduleRepository
	WorkTimeModel     configModels.WorkTimeModelRepository

	// Audit domain
	DataDeletion                 auditModels.DataDeletionRepository
	StudentDeletionAudit         auditModels.StudentDeletionRepository
	EnrollmentDeletionAudit      auditModels.EnrollmentDeletionRepository
	EnrollmentRestorationAudit   auditModels.EnrollmentRestorationRepository
	DataAccessLog                auditModels.DataAccessLogRepository
	EnrollmentOfferingAdjustment auditModels.EnrollmentOfferingAdjustmentRepository
	GuardianChange               auditModels.GuardianChangeRepository
	DeviationEvent               auditModels.DeviationEventRepository
	AuthEvent                    auditModels.AuthEventRepository
	DataImport                   auditModels.DataImportRepository
	WorkSessionEdit              auditModels.WorkSessionEditRepository
	StudentFieldEdit             auditModels.StudentFieldEditRepository
	UnregisteredTagScan          auditModels.UnregisteredTagScanRepository
	TimeTrackingDeletion         auditModels.TimeTrackingDeletionRepository
	PersonnelNumberChange        auditModels.PersonnelNumberChangeCreator
	StaffMasterDataChange        auditModels.StaffMasterDataChangeCreator
	GuardianFinancialChange      auditModels.GuardianFinancialChangeCreator
	ClassListEntryChange         auditModels.ClassListEntryChangeRepository
	TimeTrackingAuditLog         auditModels.TimeTrackingAuditLogRepository
	BookingConsistency           auditModels.BookingConsistencyRepository

	// Platform domain (operator dashboard)
	Organization             platformModels.OrganizationRepository
	Operator                 platformModels.OperatorRepository
	Announcement             platformModels.AnnouncementRepository
	AnnouncementView         platformModels.AnnouncementViewRepository
	OperatorAuditLog         platformModels.OperatorAuditLogRepository
	OperatorEmailChangeToken platformModels.OperatorEmailChangeTokenRepository
	OperatorRefreshToken     platformModels.OperatorRefreshTokenRepository
	OperatorInvitationToken  platformModels.OperatorInvitationTokenRepository
	OperatorSummaries        platformModels.OperatorSummariesRepository
	School                   platformModels.SchoolRepository
	EmailOutbox              platformModels.EmailOutboxCleanupRepository
	EmailDelivery            platformModels.EmailDeliveryRepository

	// Operator MFA (issue #1308 phase 7b)
	OperatorMFACredential     platformModels.OperatorMFACredentialRepository
	OperatorMFAEmailChallenge platformModels.OperatorMFAEmailChallengeRepository
	OperatorMFATrustedDevice  platformModels.OperatorMFATrustedDeviceRepository
	OperatorPasskeyCredential platformModels.OperatorPasskeyCredentialRepository
	OperatorPasskeySession    platformModels.OperatorPasskeySessionRepository

	// Enrollment domain (parent-enrollment PR 5+)
	FormSchema           enrollmentModels.FormSchemaRepository
	Request              enrollmentModels.RequestRepository
	EnrollmentDeletion   enrollmentModels.DeletionRepository
	RequestChild         enrollmentModels.RequestChildRepository
	RequestGuardian      enrollmentModels.RequestGuardianRepository
	LateInvite           enrollmentModels.LateInviteRepository
	CareOffering         enrollmentModels.CareOfferingRepository
	OfferingChangeImpact enrollmentModels.OfferingChangeImpactRepository
	RequestChildOffering enrollmentModels.RequestChildOfferingRepository
	ChangeRequest        enrollmentModels.ChangeRequestRepository
	ChangeRequestMessage enrollmentModels.ChangeRequestMessageRepository
	// OfferingChangeRequest carries post-enrollment care/AG change requests
	// from the parents portal (#1665).
	OfferingChangeRequest enrollmentModels.OfferingChangeRequestRepository
	SubmissionRateLimit   enrollmentModels.SubmissionRateLimitRepository
	Phase                 enrollmentModels.PhaseRepository
	PhaseExpiry           enrollmentModels.PhaseExpiryRepository

	// Display domain (info-point dashboards, issue #1325)
	Display displayModels.Repository

	// Parent domain (cross-tenant guardian portal — PR 9+)
	ParentChild             parentModels.ChildRepository
	ParentEnrollablePhase   parentModels.EnrollablePhaseRepository
	ParentEnrollmentRequest parentModels.EnrollmentRequestRepository

	// Parent Stammdaten direct-edit audit + change-request review
	StudentDataChangeRequest userModels.StudentDataChangeRequestRepository

	// Parent-OGS messaging (tenant-scoped two-way conversation per child)
	ParentMessageThread userModels.ParentMessageThreadRepository
	ParentMessage       userModels.ParentMessageRepository
	ParentMessageRead   userModels.ParentMessageReadRepository

	// OGS-internal colleague chat (#2598)
	StaffMessageThread userModels.StaffMessageThreadRepository
	StaffMessage       userModels.StaffMessageRepository
	StaffMessageRead   userModels.StaffMessageReadRepository

	// Calendar domain
	CalendarAppointment               calendarModels.AppointmentRepository
	CalendarRecurrenceRule            calendarModels.RecurrenceRuleRepository
	CalendarAppointmentRecipient      calendarModels.AppointmentRecipientRepository
	CalendarAppointmentRecipientChild calendarModels.AppointmentRecipientStudentRepository
	CalendarAppointmentTarget         calendarModels.AppointmentTargetRepository
	CalendarOccurrenceOverride        calendarModels.AppointmentOccurrenceOverrideRepository
	CalendarStaffFeedTombstone        calendarModels.StaffFeedTombstoneRepository

	// Parent announcements (tenant-authored broadcast news to guardians)
	ParentAnnouncement userModels.ParentAnnouncementRepository
}

// NewFactory creates a new repository factory with all repositories
func NewFactory(db *bun.DB) *Factory {
	activityInstance := schedule.NewActivityInstanceRepository(db)
	return &Factory{
		// Auth repositories
		Account:                auth.NewAccountRepository(db),
		AccountParent:          auth.NewAccountParentRepository(db),
		AccountTenant:          auth.NewAccountTenantRepository(db),
		StaffCalendarFeedToken: auth.NewStaffCalendarFeedTokenRepository(db),
		Role:                   auth.NewRoleRepository(db),
		Permission:             auth.NewPermissionRepository(db),
		RolePermission:         auth.NewRolePermissionRepository(db),
		AccountRole:            auth.NewAccountRoleRepository(db),
		AccountPermission:      auth.NewAccountPermissionRepository(db),
		Token:                  auth.NewTokenRepository(db),
		PasswordResetToken:     auth.NewPasswordResetTokenRepository(db),
		PasswordResetRateLimit: auth.NewPasswordResetRateLimitRepository(db),
		InvitationToken:        auth.NewInvitationTokenRepository(db),
		GuardianInvitation:     auth.NewGuardianInvitationRepository(db),
		MFACredential:          auth.NewMFACredentialRepository(db),
		MFAEmailChallenge:      auth.NewMFAEmailChallengeRepository(db),
		MFATrustedDevice:       auth.NewMFATrustedDeviceRepository(db),
		MFAOverride:            auth.NewMFAOverrideRepository(db),
		PasskeyCredential:      auth.NewPasskeyCredentialRepository(db),
		PasskeySession:         auth.NewPasskeySessionRepository(db),

		// Users repositories
		Person:              users.NewPersonRepository(db),
		RFIDCard:            users.NewRFIDCardRepository(db),
		Staff:               users.NewStaffRepository(db),
		Student:             users.NewStudentRepository(db),
		ClassListEntry:      users.NewClassListEntryRepository(db),
		StudentDeletion:     users.NewStudentDeletionRepository(db),
		CareExit:            users.NewCareExitRepository(db),
		CareExitCleanup:     users.NewCareExitCleanupRepository(db),
		CareWithdrawal:      users.NewCareWithdrawalCompletionRepository(db),
		Teacher:             users.NewTeacherRepository(db),
		Guest:               users.NewGuestRepository(db),
		Profile:             users.NewProfileRepository(db),
		StudentGuardian:     users.NewStudentGuardianRepository(db),
		StudentCompanion:    users.NewStudentCompanionRepository(db),
		GuardianProfile:     users.NewGuardianProfileRepository(db),
		GuardianPhoneNumber: users.NewGuardianPhoneNumberRepository(db),
		PrivacyConsent:      users.NewPrivacyConsentRepository(db),
		FamilyProtection:    users.NewFamilyProtectionEventRepository(db),
		ParentRequestShare:  users.NewParentRequestShareEventRepository(db),
		ParentRequestEvent:  users.NewParentRequestEventRepository(db),

		// Staff Stammdaten (#1423)
		StaffMasterData:    users.NewStaffMasterDataRepository(db),
		StaffQualification: users.NewStaffQualificationRepository(db),
		StaffFinancialData: users.NewStaffFinancialDataRepository(db),

		// Guardian payment data (#2608)
		GuardianFinancialData: users.NewGuardianFinancialDataRepository(db),

		// Staff documents (#1424)
		StaffDocument:   users.NewStaffDocumentRepository(db),
		StudentDocument: users.NewStudentDocumentRepository(db),

		// School file storage (#2596)
		FileFolder: filestore.NewFolderRepository(db),
		File:       filestore.NewFileRepository(db),
		FileEvent:  audit.NewFileEventRepository(db),

		NotificationPreference: users.NewNotificationPreferenceRepository(db),

		// Facilities repositories
		Room: facilities.NewRoomRepository(db),

		// Education repositories
		Group:             education.NewGroupRepository(db),
		GroupTeacher:      education.NewGroupTeacherRepository(db),
		ClassTeacher:      education.NewClassTeacherRepository(db),
		ClassArrivalTime:  education.NewClassArrivalTimeRepository(db),
		GroupSubstitution: education.NewGroupSubstitutionRepository(db),
		GradeTransition:   education.NewGradeTransitionRepository(db),

		// Schedule repositories
		Dateframe:                 schedule.NewDateframeRepository(db),
		Timeframe:                 schedule.NewTimeframeRepository(db),
		RecurrenceRule:            schedule.NewRecurrenceRuleRepository(db),
		StudentPickupSchedule:     schedule.NewStudentPickupScheduleRepository(db),
		StudentPickupException:    schedule.NewStudentPickupExceptionRepository(db),
		StudentPickupNote:         schedule.NewStudentPickupNoteRepository(db),
		StudentArrivalSchedule:    schedule.NewStudentArrivalScheduleRepository(db),
		StudentArrivalException:   schedule.NewStudentArrivalExceptionRepository(db),
		StudentArrivalNote:        schedule.NewStudentArrivalNoteRepository(db),
		CareScheduleChangeRequest: schedule.NewCareScheduleChangeRequestRepository(db),
		StaffShift:                schedule.NewStaffShiftRepository(db),
		StaffShiftSeries:          schedule.NewStaffShiftSeriesRepository(db),
		StaffShiftSeriesException: schedule.NewStaffShiftSeriesExceptionRepository(db),
		ShiftType:                 schedule.NewShiftTypeRepository(db),
		PlanningTrack:             schedule.NewPlanningTrackRepository(db),
		TimetableConflictAck:      schedule.NewTimetableConflictAckRepository(db),
		CalendarPeriod:            schedule.NewCalendarPeriodRepository(db),
		ClosingDay:                schedule.NewClosingDayRepository(db),
		ActivityInstance:          activityInstance,
		InstanceIdempotency:       activityInstance,
		InstanceStaff:             schedule.NewInstanceStaffRepository(db),
		InstanceStudent:           schedule.NewInstanceStudentRepository(db),
		ActivityException:         schedule.NewActivityExceptionRepository(db),

		// Activities repositories
		ActivityGroup:      activities.NewGroupRepository(db),
		ActivityCategory:   activities.NewCategoryRepository(db),
		ActivitySchedule:   activities.NewScheduleRepository(db),
		ActivitySupervisor: activities.NewSupervisorPlannedRepository(db),
		StudentEnrollment:  activities.NewStudentEnrollmentRepository(db),

		// Active repositories
		ActiveGroup:           active.NewGroupRepository(db),
		ActiveVisit:           active.NewVisitRepository(db),
		GroupSupervisor:       active.NewGroupSupervisorRepository(db),
		CombinedGroup:         active.NewCombinedGroupRepository(db),
		GroupMapping:          active.NewGroupMappingRepository(db),
		Attendance:            active.NewAttendanceRepository(db),
		StudentStatusDay:      active.NewStudentStatusDayRepository(db),
		Statistics:            active.NewStatisticsRepository(db),
		ExcusedAbsenceRequest: active.NewExcusedAbsenceRequestRepository(db),
		WorkSession:           active.NewWorkSessionRepository(db),
		WorkSessionBreak:      active.NewWorkSessionBreakRepository(db),
		StaffAbsence:          active.NewStaffAbsenceRepository(db),
		StaffAbsenceAudit:     active.NewStaffAbsenceAuditRepository(db),
		StaffAbsenceType:      active.NewStaffAbsenceTypeRepository(db),
		StaffVacationQuota:    active.NewStaffVacationQuotaRepository(db),
		StaffVacationOpening:  active.NewStaffVacationOpeningRepository(db),
		StaffBalanceAdjust:    active.NewStaffBalanceAdjustmentRepository(db),
		StaffMonthSnapshot:    active.NewStaffMonthBalanceSnapshotRepository(db),

		// Meal plan repositories
		MealPlanEntry: mealplanRepo.NewMealPlanEntryRepository(db),

		// Feedback repositories
		FeedbackEntry: feedback.NewEntryRepository(db),

		// IoT repositories
		Device:             iot.NewDeviceRepository(db),
		PushSubscription:   iot.NewPushSubscriptionRepository(db),
		PWAStandaloneUsage: iot.NewPWAStandaloneUsageRepository(db),

		// Config repositories
		SettingValue:      config.NewSettingValueRepository(config.NewRuntime(db)),
		SettingAudit:      config.NewSettingAuditRepository(config.NewRuntime(db)),
		StaffWorkSchedule: config.NewStaffWorkScheduleRepository(config.NewRuntime(db)),
		WorkTimeModel:     config.NewWorkTimeModelRepository(config.NewRuntime(db)),

		// Audit repositories
		DataDeletion:                 audit.NewDataDeletionRepository(db),
		StudentDeletionAudit:         audit.NewStudentDeletionRepository(db),
		EnrollmentDeletionAudit:      audit.NewEnrollmentDeletionRepository(db),
		EnrollmentRestorationAudit:   audit.NewEnrollmentRestorationRepository(db),
		DataAccessLog:                audit.NewDataAccessLogRepository(db),
		EnrollmentOfferingAdjustment: audit.NewEnrollmentOfferingAdjustmentRepository(db),
		GuardianChange:               audit.NewGuardianChangeRepository(db),
		DeviationEvent:               audit.NewDeviationEventRepository(db),
		AuthEvent:                    audit.NewAuthEventRepository(db),
		DataImport:                   audit.NewDataImportRepository(db),
		WorkSessionEdit:              audit.NewWorkSessionEditRepository(db),
		StudentFieldEdit:             audit.NewStudentFieldEditRepository(db),
		UnregisteredTagScan:          audit.NewUnregisteredTagScanRepository(db),
		TimeTrackingDeletion:         audit.NewTimeTrackingDeletionRepository(db),
		PersonnelNumberChange:        audit.NewPersonnelNumberChangeRepository(db),
		StaffMasterDataChange:        audit.NewStaffMasterDataChangeRepository(db),
		GuardianFinancialChange:      audit.NewGuardianFinancialChangeRepository(db),
		ClassListEntryChange:         audit.NewClassListEntryChangeRepository(db),
		TimeTrackingAuditLog:         audit.NewTimeTrackingAuditLogRepository(db),
		BookingConsistency:           audit.NewBookingConsistencyRepository(db),

		// Platform repositories
		Organization:             platformRepo.NewOrganizationRepository(db),
		Operator:                 platformRepo.NewOperatorRepository(db),
		Announcement:             platformRepo.NewAnnouncementRepository(db),
		AnnouncementView:         platformRepo.NewAnnouncementViewRepository(db),
		OperatorAuditLog:         platformRepo.NewOperatorAuditLogRepository(db),
		OperatorEmailChangeToken: platformRepo.NewOperatorEmailChangeTokenRepository(db),
		OperatorRefreshToken:     platformRepo.NewOperatorRefreshTokenRepository(db),
		OperatorInvitationToken:  platformRepo.NewOperatorInvitationTokenRepository(db),
		OperatorSummaries:        platformRepo.NewOperatorSummariesRepository(db),
		School:                   platformRepo.NewSchoolRepository(db),
		EmailOutbox:              platformRepo.NewEmailOutboxRepository(db),
		EmailDelivery:            platformRepo.NewEmailDeliveryRepository(db),

		OperatorMFACredential:     platformRepo.NewOperatorMFACredentialRepository(db),
		OperatorMFAEmailChallenge: platformRepo.NewOperatorMFAEmailChallengeRepository(db),
		OperatorMFATrustedDevice:  platformRepo.NewOperatorMFATrustedDeviceRepository(db),
		OperatorPasskeyCredential: platformRepo.NewOperatorPasskeyCredentialRepository(db),
		OperatorPasskeySession:    platformRepo.NewOperatorPasskeySessionRepository(db),

		// Enrollment repositories
		FormSchema:            enrollment.NewFormSchemaRepository(db),
		Request:               enrollment.NewRequestRepository(db),
		EnrollmentDeletion:    enrollment.NewDeletionRepository(db),
		RequestChild:          enrollment.NewRequestChildRepository(db),
		RequestGuardian:       enrollment.NewRequestGuardianRepository(db),
		LateInvite:            enrollment.NewLateInviteRepository(db),
		CareOffering:          enrollment.NewCareOfferingRepository(db),
		OfferingChangeImpact:  enrollment.NewOfferingChangeImpactRepository(db),
		RequestChildOffering:  enrollment.NewRequestChildOfferingRepository(db),
		OfferingChangeRequest: enrollment.NewOfferingChangeRequestRepository(db),
		ChangeRequest:         enrollment.NewChangeRequestRepository(db),
		ChangeRequestMessage:  enrollment.NewChangeRequestMessageRepository(db),
		SubmissionRateLimit:   enrollment.NewSubmissionRateLimitRepository(db),
		Phase:                 enrollment.NewPhaseRepository(db),
		PhaseExpiry:           enrollment.NewPhaseExpiryRepository(db),

		// Display (info-point dashboards, issue #1325)
		Display: displayRepo.NewDisplayRepository(db),

		// Parent (cross-tenant guardian portal — PR 9+)
		ParentChild:             parentRepo.NewChildRepository(db),
		ParentEnrollablePhase:   parentRepo.NewEnrollablePhaseRepository(db),
		ParentEnrollmentRequest: parentRepo.NewEnrollmentRequestRepository(db),

		// Parent Stammdaten direct-edit audit + change-request review
		StudentDataChangeRequest: users.NewStudentDataChangeRequestRepository(db),

		// Parent-OGS messaging (tenant-scoped two-way conversation per child)
		ParentMessageThread: users.NewParentMessageThreadRepository(db),
		ParentMessage:       users.NewParentMessageRepository(db),
		ParentMessageRead:   users.NewParentMessageReadRepository(db),

		StaffMessageThread: users.NewStaffMessageThreadRepository(db),
		StaffMessage:       users.NewStaffMessageRepository(db),
		StaffMessageRead:   users.NewStaffMessageReadRepository(db),

		// Calendar repositories
		CalendarAppointment:               calendarRepo.NewAppointmentRepository(db),
		CalendarRecurrenceRule:            calendarRepo.NewRecurrenceRuleRepository(db),
		CalendarAppointmentRecipient:      calendarRepo.NewAppointmentRecipientRepository(db),
		CalendarAppointmentRecipientChild: calendarRepo.NewAppointmentRecipientStudentRepository(db),
		CalendarAppointmentTarget:         calendarRepo.NewAppointmentTargetRepository(db),
		CalendarOccurrenceOverride:        calendarRepo.NewAppointmentOccurrenceOverrideRepository(db),
		CalendarStaffFeedTombstone:        calendarRepo.NewStaffFeedTombstoneRepository(db),
		ParentAnnouncement:                users.NewParentAnnouncementRepository(db),
	}
}

// SetConfigRuntime replaces the bootstrap repositories with tenant-aware
// instances before the service graph captures them.
func (f *Factory) SetConfigRuntime(runtime config.Runtime) {
	f.SettingValue = config.NewSettingValueRepository(runtime)
	f.SettingAudit = config.NewSettingAuditRepository(runtime)
	f.StaffWorkSchedule = config.NewStaffWorkScheduleRepository(runtime)
	f.WorkTimeModel = config.NewWorkTimeModelRepository(runtime)
}
