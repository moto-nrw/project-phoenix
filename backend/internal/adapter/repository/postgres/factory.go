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

	activePort "github.com/moto-nrw/project-phoenix/internal/core/port/active"
	activitiesPort "github.com/moto-nrw/project-phoenix/internal/core/port/activities"
	auditPort "github.com/moto-nrw/project-phoenix/internal/core/port/audit"
	authPort "github.com/moto-nrw/project-phoenix/internal/core/port/auth"
	configPort "github.com/moto-nrw/project-phoenix/internal/core/port/config"
	educationPort "github.com/moto-nrw/project-phoenix/internal/core/port/education"
	facilityPort "github.com/moto-nrw/project-phoenix/internal/core/port/facilities"
	feedbackPort "github.com/moto-nrw/project-phoenix/internal/core/port/feedback"
	iotPort "github.com/moto-nrw/project-phoenix/internal/core/port/iot"
	schedulePort "github.com/moto-nrw/project-phoenix/internal/core/port/schedule"
	userPort "github.com/moto-nrw/project-phoenix/internal/core/port/users"

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
	Person          userPort.PersonRepository
	RFIDCard        userPort.RFIDCardRepository
	Staff           userPort.StaffRepository
	Student         userPort.StudentRepository
	Teacher         userPort.TeacherRepository
	Guest           userPort.GuestRepository
	Profile         userPort.ProfileRepository
	PersonGuardian  userPort.PersonGuardianRepository
	StudentGuardian userPort.StudentGuardianRepository
	GuardianProfile userPort.GuardianProfileRepository
	PrivacyConsent  userPort.PrivacyConsentRepository

	// Facilities domain
	Room facilityPort.RoomRepository

	// Education domain
	Group                      educationPort.GroupRepository
	GroupTeacher               educationPort.GroupTeacherRepository
	GroupSubstitution          educationPort.GroupSubstitutionRepository
	GroupSubstitutionRelations educationPort.GroupSubstitutionRelationsRepository

	// Schedule domain
	Dateframe      schedulePort.DateframeRepository
	Timeframe      schedulePort.TimeframeRepository
	RecurrenceRule schedulePort.RecurrenceRuleRepository

	// Activities domain
	ActivityGroup      activitiesPort.GroupRepository
	ActivityCategory   activitiesPort.CategoryRepository
	ActivitySchedule   activitiesPort.ScheduleRepository
	ActivitySupervisor activitiesPort.SupervisorPlannedRepository
	StudentEnrollment  activitiesPort.StudentEnrollmentRepository

	// Active domain
	ActiveGroup     *active.GroupRepository
	ActiveVisit     activePort.VisitRepository
	GroupSupervisor activePort.GroupSupervisorRepository
	CombinedGroup   activePort.CombinedGroupRepository
	GroupMapping    activePort.GroupMappingRepository
	Attendance      activePort.AttendanceRepository

	// Feedback domain
	FeedbackEntry feedbackPort.EntryRepository

	// IoT domain
	Device iotPort.DeviceRepository

	// Config domain
	Setting configPort.SettingRepository

	// Audit domain
	DataDeletion auditPort.DataDeletionRepository
	AuthEvent    auditPort.AuthEventRepository
	DataImport   auditPort.DataImportRepository
}

// NewFactory creates a new repository factory with all repositories
func NewFactory(db *bun.DB) *Factory {
	groupSubstitutionRepo := education.NewGroupSubstitutionRepository(db)

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
		Group:                      education.NewGroupRepository(db),
		GroupTeacher:               education.NewGroupTeacherRepository(db),
		GroupSubstitution:          groupSubstitutionRepo,
		GroupSubstitutionRelations: groupSubstitutionRepo,

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
