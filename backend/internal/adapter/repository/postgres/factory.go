package repositories

import (
	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/active"
	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/activities"
	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/audit"
	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/auth"
	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/config"
	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/education"
	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/facilities"
	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/feedback"
	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/iot"
	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/schedule"
	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/users"

	activeModels "github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	activitiesModels "github.com/moto-nrw/project-phoenix/internal/core/domain/activities"
	auditModels "github.com/moto-nrw/project-phoenix/internal/core/domain/audit"
	configModels "github.com/moto-nrw/project-phoenix/internal/core/domain/config"
	educationModels "github.com/moto-nrw/project-phoenix/internal/core/domain/education"
	facilityModels "github.com/moto-nrw/project-phoenix/internal/core/domain/facilities"
	feedbackModels "github.com/moto-nrw/project-phoenix/internal/core/domain/feedback"
	iotModels "github.com/moto-nrw/project-phoenix/internal/core/domain/iot"
	scheduleModels "github.com/moto-nrw/project-phoenix/internal/core/domain/schedule"
	userModels "github.com/moto-nrw/project-phoenix/internal/core/domain/users"
	authPort "github.com/moto-nrw/project-phoenix/internal/core/port/auth"

	"github.com/uptrace/bun"
)

// Factory provides access to all repositories
type Factory struct {
	// Auth domain
	Account                authPort.AccountRepository
	AccountParent          authPort.AccountParentRepository
	Role                   authPort.RoleRepository
	Permission             authPort.PermissionRepository
	RolePermission         authPort.RolePermissionRepository
	AccountRole            authPort.AccountRoleRepository
	AccountPermission      authPort.AccountPermissionRepository
	Token                  authPort.TokenRepository
	PasswordResetToken     authPort.PasswordResetTokenRepository
	PasswordResetRateLimit authPort.PasswordResetRateLimitRepository
	InvitationToken        authPort.InvitationTokenRepository
	GuardianInvitation     authPort.GuardianInvitationRepository

	// Users domain
	Person          userModels.PersonRepository
	RFIDCard        userModels.RFIDCardRepository
	Staff           userModels.StaffRepository
	Student         userModels.StudentRepository
	Teacher         userModels.TeacherRepository
	Guest           userModels.GuestRepository
	Profile         userModels.ProfileRepository
	PersonGuardian  userModels.PersonGuardianRepository
	StudentGuardian userModels.StudentGuardianRepository
	GuardianProfile userModels.GuardianProfileRepository
	PrivacyConsent  userModels.PrivacyConsentRepository

	// Facilities domain
	Room facilityModels.RoomRepository

	// Education domain
	Group             educationModels.GroupRepository
	GroupTeacher      educationModels.GroupTeacherRepository
	GroupSubstitution educationModels.GroupSubstitutionRepository

	// Schedule domain
	Dateframe      scheduleModels.DateframeRepository
	Timeframe      scheduleModels.TimeframeRepository
	RecurrenceRule scheduleModels.RecurrenceRuleRepository

	// Activities domain
	ActivityGroup      activitiesModels.GroupRepository
	ActivityCategory   activitiesModels.CategoryRepository
	ActivitySchedule   activitiesModels.ScheduleRepository
	ActivitySupervisor activitiesModels.SupervisorPlannedRepository
	StudentEnrollment  activitiesModels.StudentEnrollmentRepository

	// Active domain
	ActiveGroup     activeModels.GroupRepository
	ActiveVisit     activeModels.VisitRepository
	GroupSupervisor activeModels.GroupSupervisorRepository
	CombinedGroup   activeModels.CombinedGroupRepository
	GroupMapping    activeModels.GroupMappingRepository
	Attendance      activeModels.AttendanceRepository

	// Feedback domain
	FeedbackEntry feedbackModels.EntryRepository

	// IoT domain
	Device iotModels.DeviceRepository

	// Config domain
	Setting configModels.SettingRepository

	// Audit domain
	DataDeletion auditModels.DataDeletionRepository
	AuthEvent    auditModels.AuthEventRepository
	DataImport   auditModels.DataImportRepository
}

// NewFactory creates a new repository factory with all repositories
func NewFactory(db *bun.DB) *Factory {
	return &Factory{
		// Auth repositories
		Account:                auth.NewAccountRepository(db),
		AccountParent:          auth.NewAccountParentRepository(db),
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

		// Users repositories
		Person:          users.NewPersonRepository(db),
		RFIDCard:        users.NewRFIDCardRepository(db),
		Staff:           users.NewStaffRepository(db),
		Student:         users.NewStudentRepository(db),
		Teacher:         users.NewTeacherRepository(db),
		Guest:           users.NewGuestRepository(db),
		Profile:         users.NewProfileRepository(db),
		PersonGuardian:  users.NewPersonGuardianRepository(db),
		StudentGuardian: users.NewStudentGuardianRepository(db),
		GuardianProfile: users.NewGuardianProfileRepository(db),
		PrivacyConsent:  users.NewPrivacyConsentRepository(db),

		// Facilities repositories
		Room: facilities.NewRoomRepository(db),

		// Education repositories
		Group:             education.NewGroupRepository(db),
		GroupTeacher:      education.NewGroupTeacherRepository(db),
		GroupSubstitution: education.NewGroupSubstitutionRepository(db),

		// Schedule repositories
		Dateframe:      schedule.NewDateframeRepository(db),
		Timeframe:      schedule.NewTimeframeRepository(db),
		RecurrenceRule: schedule.NewRecurrenceRuleRepository(db),

		// Activities repositories
		ActivityGroup:      activities.NewGroupRepository(db),
		ActivityCategory:   activities.NewCategoryRepository(db),
		ActivitySchedule:   activities.NewScheduleRepository(db),
		ActivitySupervisor: activities.NewSupervisorPlannedRepository(db),
		StudentEnrollment:  activities.NewStudentEnrollmentRepository(db),

		// Active repositories
		ActiveGroup:     active.NewGroupRepository(db),
		ActiveVisit:     active.NewVisitRepository(db),
		GroupSupervisor: active.NewGroupSupervisorRepository(db),
		CombinedGroup:   active.NewCombinedGroupRepository(db),
		GroupMapping:    active.NewGroupMappingRepository(db),
		Attendance:      active.NewAttendanceRepository(db),

		// Feedback repositories
		FeedbackEntry: feedback.NewEntryRepository(db),

		// IoT repositories
		Device: iot.NewDeviceRepository(db),

		// Config repositories
		Setting: config.NewSettingRepository(db),

		// Audit repositories
		DataDeletion: audit.NewDataDeletionRepository(db),
		AuthEvent:    audit.NewAuthEventRepository(db),
		DataImport:   audit.NewDataImportRepository(db),
	}
}
