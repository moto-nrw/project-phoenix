package repositories

import (
	"github.com/moto-nrw/project-phoenix/database/repositories/active"
	"github.com/moto-nrw/project-phoenix/database/repositories/activities"
	"github.com/moto-nrw/project-phoenix/database/repositories/audit"
	"github.com/moto-nrw/project-phoenix/database/repositories/auth"
	"github.com/moto-nrw/project-phoenix/database/repositories/config"
	"github.com/moto-nrw/project-phoenix/database/repositories/education"
	"github.com/moto-nrw/project-phoenix/database/repositories/facilities"
	"github.com/moto-nrw/project-phoenix/database/repositories/feedback"
	"github.com/moto-nrw/project-phoenix/database/repositories/iot"
	"github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	"github.com/moto-nrw/project-phoenix/database/repositories/users"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	facilityModels "github.com/moto-nrw/project-phoenix/models/facilities"
	feedbackModels "github.com/moto-nrw/project-phoenix/models/feedback"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"

	"github.com/uptrace/bun"
)

// Factory provides access to all repositories
type Factory struct {
	// Auth domain
	Account            authModels.AccountRepository
	AccountParent      authModels.AccountParentRepository
	Role               authModels.RoleRepository
	Permission         authModels.PermissionRepository
	RolePermission     authModels.RolePermissionRepository
	AccountRole        authModels.AccountRoleRepository
	AccountPermission  authModels.AccountPermissionRepository
	Token              authModels.TokenRepository
	PasswordResetToken authModels.PasswordResetTokenRepository

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
}

// NewFactory creates a new repository factory with all repositories
func NewFactory(db *bun.DB) *Factory {
	return &Factory{
		// Auth repositories
		Account:            auth.NewAccountRepository(db),
		AccountParent:      auth.NewAccountParentRepository(db),
		Role:               auth.NewRoleRepository(db),
		Permission:         auth.NewPermissionRepository(db),
		RolePermission:     auth.NewRolePermissionRepository(db),
		AccountRole:        auth.NewAccountRoleRepository(db),
		AccountPermission:  auth.NewAccountPermissionRepository(db),
		Token:              auth.NewTokenRepository(db),
		PasswordResetToken: auth.NewPasswordResetTokenRepository(db),

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
	}
}
