package repositories

import (
	"github.com/moto-nrw/project-phoenix/database/repositories/active"
	"github.com/moto-nrw/project-phoenix/database/repositories/activities"
	"github.com/moto-nrw/project-phoenix/database/repositories/audit"
	"github.com/moto-nrw/project-phoenix/database/repositories/auth"
	calendarRepo "github.com/moto-nrw/project-phoenix/database/repositories/calendar"
	"github.com/moto-nrw/project-phoenix/database/repositories/config"
	"github.com/moto-nrw/project-phoenix/database/repositories/education"
	"github.com/moto-nrw/project-phoenix/database/repositories/enrollment"
	"github.com/moto-nrw/project-phoenix/database/repositories/facilities"
	"github.com/moto-nrw/project-phoenix/database/repositories/feedback"
	"github.com/moto-nrw/project-phoenix/database/repositories/iot"
	mealplanRepo "github.com/moto-nrw/project-phoenix/database/repositories/mealplan"
	parentRepo "github.com/moto-nrw/project-phoenix/database/repositories/parent"
	platformRepo "github.com/moto-nrw/project-phoenix/database/repositories/platform"
	"github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	suggestionsRepo "github.com/moto-nrw/project-phoenix/database/repositories/suggestions"
	"github.com/moto-nrw/project-phoenix/database/repositories/users"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	calendarModels "github.com/moto-nrw/project-phoenix/models/calendar"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	facilityModels "github.com/moto-nrw/project-phoenix/models/facilities"
	feedbackModels "github.com/moto-nrw/project-phoenix/models/feedback"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	mealplanModels "github.com/moto-nrw/project-phoenix/models/mealplan"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	suggestionsModels "github.com/moto-nrw/project-phoenix/models/suggestions"
	userModels "github.com/moto-nrw/project-phoenix/models/users"

	"github.com/uptrace/bun"
)

// Factory provides access to all repositories
type Factory struct {
	// Auth domain
	Account                authModels.AccountRepository
	AccountParent          authModels.AccountParentRepository
	AccountTenant          authModels.AccountTenantRepository
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
	Teacher             userModels.TeacherRepository
	Guest               userModels.GuestRepository
	Profile             userModels.ProfileRepository
	PersonGuardian      userModels.PersonGuardianRepository
	StudentGuardian     userModels.StudentGuardianRepository
	GuardianProfile     userModels.GuardianProfileRepository
	GuardianPhoneNumber userModels.GuardianPhoneNumberRepository
	PrivacyConsent      userModels.PrivacyConsentRepository

	// Facilities domain
	Room facilityModels.RoomRepository

	// Education domain
	Group             educationModels.GroupRepository
	GroupTeacher      educationModels.GroupTeacherRepository
	GroupSubstitution educationModels.GroupSubstitutionRepository
	GradeTransition   educationModels.GradeTransitionRepository

	// Schedule domain
	Dateframe               scheduleModels.DateframeRepository
	Timeframe               scheduleModels.TimeframeRepository
	RecurrenceRule          scheduleModels.RecurrenceRuleRepository
	StudentPickupSchedule   scheduleModels.StudentPickupScheduleRepository
	StudentPickupException  scheduleModels.StudentPickupExceptionRepository
	StudentPickupNote       scheduleModels.StudentPickupNoteRepository
	StudentArrivalSchedule  scheduleModels.StudentArrivalScheduleRepository
	StudentArrivalException scheduleModels.StudentArrivalExceptionRepository
	StudentArrivalNote      scheduleModels.StudentArrivalNoteRepository
	CalendarPeriod          scheduleModels.CalendarPeriodRepository
	ActivityInstance        scheduleModels.ActivityInstanceRepository
	InstanceStaff           scheduleModels.InstanceStaffRepository
	InstanceStudent         scheduleModels.InstanceStudentRepository
	ActivityException       scheduleModels.ActivityExceptionRepository

	// Activities domain
	ActivityGroup      activitiesModels.GroupRepository
	ActivityCategory   activitiesModels.CategoryRepository
	ActivitySchedule   activitiesModels.ScheduleRepository
	ActivitySupervisor activitiesModels.SupervisorPlannedRepository
	StudentEnrollment  activitiesModels.StudentEnrollmentRepository

	// Active domain
	ActiveGroup        activeModels.GroupRepository
	ActiveVisit        activeModels.VisitRepository
	GroupSupervisor    activeModels.GroupSupervisorRepository
	CombinedGroup      activeModels.CombinedGroupRepository
	GroupMapping       activeModels.GroupMappingRepository
	Attendance         activeModels.AttendanceRepository
	StudentStatusDay   activeModels.StudentStatusDayRepository
	WorkSession        activeModels.WorkSessionRepository
	WorkSessionBreak   activeModels.WorkSessionBreakRepository
	StaffAbsence       activeModels.StaffAbsenceRepository
	StaffAbsenceAudit  activeModels.StaffAbsenceAuditRepository
	StaffVacationQuota activeModels.StaffVacationQuotaRepository

	// Meal plan domain
	MealPlanEntry mealplanModels.MealPlanEntryRepository

	// Feedback domain
	FeedbackEntry feedbackModels.EntryRepository

	// IoT domain
	Device iotModels.DeviceRepository

	// Config domain
	SettingValue      configModels.SettingValueRepository
	SettingAudit      configModels.SettingAuditRepository
	StaffWorkSchedule configModels.StaffWorkScheduleRepository
	WorkTimeModel     configModels.WorkTimeModelRepository

	// Suggestions domain
	SuggestionPost        suggestionsModels.PostRepository
	SuggestionVote        suggestionsModels.VoteRepository
	SuggestionComment     suggestionsModels.CommentRepository
	SuggestionCommentRead suggestionsModels.CommentReadRepository
	SuggestionPostRead    suggestionsModels.PostReadRepository

	// Audit domain
	DataDeletion                 auditModels.DataDeletionRepository
	DataAccessLog                auditModels.DataAccessLogRepository
	EnrollmentOfferingAdjustment auditModels.EnrollmentOfferingAdjustmentRepository
	GuardianChange               auditModels.GuardianChangeRepository
	AuthEvent                    auditModels.AuthEventRepository
	DataImport                   auditModels.DataImportRepository
	WorkSessionEdit              auditModels.WorkSessionEditRepository
	UnregisteredTagScan          auditModels.UnregisteredTagScanRepository

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
	EmailOutbox              platformModels.EmailOutboxRepository

	// Operator MFA (issue #1308 phase 7b)
	OperatorMFACredential     platformModels.OperatorMFACredentialRepository
	OperatorMFAEmailChallenge platformModels.OperatorMFAEmailChallengeRepository
	OperatorMFATrustedDevice  platformModels.OperatorMFATrustedDeviceRepository
	OperatorPasskeyCredential platformModels.OperatorPasskeyCredentialRepository
	OperatorPasskeySession    platformModels.OperatorPasskeySessionRepository

	// Enrollment domain (parent-enrollment PR 5+)
	FormSchema           enrollmentModels.FormSchemaRepository
	Request              enrollmentModels.RequestRepository
	RequestChild         enrollmentModels.RequestChildRepository
	RequestGuardian      enrollmentModels.RequestGuardianRepository
	LateInvite           enrollmentModels.LateInviteRepository
	CareOffering         enrollmentModels.CareOfferingRepository
	RequestChildOffering enrollmentModels.RequestChildOfferingRepository
	ChangeRequest        enrollmentModels.ChangeRequestRepository
	ChangeRequestMessage enrollmentModels.ChangeRequestMessageRepository
	SubmissionRateLimit  enrollmentModels.SubmissionRateLimitRepository
	Phase                enrollmentModels.PhaseRepository

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

	// Calendar domain
	CalendarAppointment               calendarModels.AppointmentRepository
	CalendarRecurrenceRule            calendarModels.RecurrenceRuleRepository
	CalendarAppointmentRecipient      calendarModels.AppointmentRecipientRepository
	CalendarAppointmentRecipientChild calendarModels.AppointmentRecipientStudentRepository
	CalendarAppointmentTarget         calendarModels.AppointmentTargetRepository
	CalendarOccurrenceOverride        calendarModels.AppointmentOccurrenceOverrideRepository
}

// NewFactory creates a new repository factory with all repositories
func NewFactory(db *bun.DB) *Factory {
	return &Factory{
		// Auth repositories
		Account:                auth.NewAccountRepository(db),
		AccountParent:          auth.NewAccountParentRepository(db),
		AccountTenant:          auth.NewAccountTenantRepository(db),
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
		Teacher:             users.NewTeacherRepository(db),
		Guest:               users.NewGuestRepository(db),
		Profile:             users.NewProfileRepository(db),
		PersonGuardian:      users.NewPersonGuardianRepository(db),
		StudentGuardian:     users.NewStudentGuardianRepository(db),
		GuardianProfile:     users.NewGuardianProfileRepository(db),
		GuardianPhoneNumber: users.NewGuardianPhoneNumberRepository(db),
		PrivacyConsent:      users.NewPrivacyConsentRepository(db),

		// Facilities repositories
		Room: facilities.NewRoomRepository(db),

		// Education repositories
		Group:             education.NewGroupRepository(db),
		GroupTeacher:      education.NewGroupTeacherRepository(db),
		GroupSubstitution: education.NewGroupSubstitutionRepository(db),
		GradeTransition:   education.NewGradeTransitionRepository(db),

		// Schedule repositories
		Dateframe:               schedule.NewDateframeRepository(db),
		Timeframe:               schedule.NewTimeframeRepository(db),
		RecurrenceRule:          schedule.NewRecurrenceRuleRepository(db),
		StudentPickupSchedule:   schedule.NewStudentPickupScheduleRepository(db),
		StudentPickupException:  schedule.NewStudentPickupExceptionRepository(db),
		StudentPickupNote:       schedule.NewStudentPickupNoteRepository(db),
		StudentArrivalSchedule:  schedule.NewStudentArrivalScheduleRepository(db),
		StudentArrivalException: schedule.NewStudentArrivalExceptionRepository(db),
		StudentArrivalNote:      schedule.NewStudentArrivalNoteRepository(db),
		CalendarPeriod:          schedule.NewCalendarPeriodRepository(db),
		ActivityInstance:        schedule.NewActivityInstanceRepository(db),
		InstanceStaff:           schedule.NewInstanceStaffRepository(db),
		InstanceStudent:         schedule.NewInstanceStudentRepository(db),
		ActivityException:       schedule.NewActivityExceptionRepository(db),

		// Activities repositories
		ActivityGroup:      activities.NewGroupRepository(db),
		ActivityCategory:   activities.NewCategoryRepository(db),
		ActivitySchedule:   activities.NewScheduleRepository(db),
		ActivitySupervisor: activities.NewSupervisorPlannedRepository(db),
		StudentEnrollment:  activities.NewStudentEnrollmentRepository(db),

		// Active repositories
		ActiveGroup:        active.NewGroupRepository(db),
		ActiveVisit:        active.NewVisitRepository(db),
		GroupSupervisor:    active.NewGroupSupervisorRepository(db),
		CombinedGroup:      active.NewCombinedGroupRepository(db),
		GroupMapping:       active.NewGroupMappingRepository(db),
		Attendance:         active.NewAttendanceRepository(db),
		StudentStatusDay:   active.NewStudentStatusDayRepository(db),
		WorkSession:        active.NewWorkSessionRepository(db),
		WorkSessionBreak:   active.NewWorkSessionBreakRepository(db),
		StaffAbsence:       active.NewStaffAbsenceRepository(db),
		StaffAbsenceAudit:  active.NewStaffAbsenceAuditRepository(db),
		StaffVacationQuota: active.NewStaffVacationQuotaRepository(db),

		// Meal plan repositories
		MealPlanEntry: mealplanRepo.NewMealPlanEntryRepository(db),

		// Feedback repositories
		FeedbackEntry: feedback.NewEntryRepository(db),

		// IoT repositories
		Device: iot.NewDeviceRepository(db),

		// Config repositories
		SettingValue:      config.NewSettingValueRepository(db),
		SettingAudit:      config.NewSettingAuditRepository(db),
		StaffWorkSchedule: config.NewStaffWorkScheduleRepository(db),
		WorkTimeModel:     config.NewWorkTimeModelRepository(db),

		// Suggestions repositories
		SuggestionPost:        suggestionsRepo.NewPostRepository(db),
		SuggestionVote:        suggestionsRepo.NewVoteRepository(db),
		SuggestionComment:     suggestionsRepo.NewCommentRepository(db),
		SuggestionCommentRead: suggestionsRepo.NewCommentReadRepository(db),
		SuggestionPostRead:    suggestionsRepo.NewPostReadRepository(db),

		// Audit repositories
		DataDeletion:                 audit.NewDataDeletionRepository(db),
		DataAccessLog:                audit.NewDataAccessLogRepository(db),
		EnrollmentOfferingAdjustment: audit.NewEnrollmentOfferingAdjustmentRepository(db),
		GuardianChange:               audit.NewGuardianChangeRepository(db),
		AuthEvent:                    audit.NewAuthEventRepository(db),
		DataImport:                   audit.NewDataImportRepository(db),
		WorkSessionEdit:              audit.NewWorkSessionEditRepository(db),
		UnregisteredTagScan:          audit.NewUnregisteredTagScanRepository(db),

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

		OperatorMFACredential:     platformRepo.NewOperatorMFACredentialRepository(db),
		OperatorMFAEmailChallenge: platformRepo.NewOperatorMFAEmailChallengeRepository(db),
		OperatorMFATrustedDevice:  platformRepo.NewOperatorMFATrustedDeviceRepository(db),
		OperatorPasskeyCredential: platformRepo.NewOperatorPasskeyCredentialRepository(db),
		OperatorPasskeySession:    platformRepo.NewOperatorPasskeySessionRepository(db),

		// Enrollment repositories
		FormSchema:           enrollment.NewFormSchemaRepository(db),
		Request:              enrollment.NewRequestRepository(db),
		RequestChild:         enrollment.NewRequestChildRepository(db),
		RequestGuardian:      enrollment.NewRequestGuardianRepository(db),
		LateInvite:           enrollment.NewLateInviteRepository(db),
		CareOffering:         enrollment.NewCareOfferingRepository(db),
		RequestChildOffering: enrollment.NewRequestChildOfferingRepository(db),
		ChangeRequest:        enrollment.NewChangeRequestRepository(db),
		ChangeRequestMessage: enrollment.NewChangeRequestMessageRepository(db),
		SubmissionRateLimit:  enrollment.NewSubmissionRateLimitRepository(db),
		Phase:                enrollment.NewPhaseRepository(db),

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

		// Calendar repositories
		CalendarAppointment:               calendarRepo.NewAppointmentRepository(db),
		CalendarRecurrenceRule:            calendarRepo.NewRecurrenceRuleRepository(db),
		CalendarAppointmentRecipient:      calendarRepo.NewAppointmentRecipientRepository(db),
		CalendarAppointmentRecipientChild: calendarRepo.NewAppointmentRecipientStudentRepository(db),
		CalendarAppointmentTarget:         calendarRepo.NewAppointmentTargetRepository(db),
		CalendarOccurrenceOverride:        calendarRepo.NewAppointmentOccurrenceOverrideRepository(db),
	}
}
