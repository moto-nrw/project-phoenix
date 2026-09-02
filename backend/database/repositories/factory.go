package repositories

import (
	"context"
	"time"

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
	"github.com/moto-nrw/project-phoenix/database/repositories/filestore"
	"github.com/moto-nrw/project-phoenix/database/repositories/iot"
	parentRepo "github.com/moto-nrw/project-phoenix/database/repositories/parent"
	platformRepo "github.com/moto-nrw/project-phoenix/database/repositories/platform"
	"github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	"github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/database/repositories/workforce"
	filestoreModels "github.com/moto-nrw/project-phoenix/models/filestore"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	calendarModels "github.com/moto-nrw/project-phoenix/models/calendar"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	deliveryModels "github.com/moto-nrw/project-phoenix/models/delivery"
	displayModels "github.com/moto-nrw/project-phoenix/models/display"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	facilityModels "github.com/moto-nrw/project-phoenix/models/facilities"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	schoolStructureCompose "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"

	"github.com/uptrace/bun"
)

// Factory provides access to all repositories
type Factory struct {
	db                       *bun.DB
	organizationTenancyBound bool
	schoolStructureBound     bool

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

	CaregiverBindingLock userModels.CaregiverBindingLocker

	// Staff Stammdaten (#1423)
	StaffMasterData    userModels.StaffMasterDataRepository
	StaffQualification userModels.StaffQualificationRepository
	StaffFinancialData userModels.StaffFinancialDataRepository

	// Guardian payment data (#2608)
	GuardianFinancialData userModels.GuardianFinancialDataRepository

	// Staff documents (#1424)
	StaffDocument   userModels.StaffDocumentRepository
	StudentDocument userModels.StudentDocumentRepository

	// School file storage (#2596) and the attachments of Elternmitteilungen
	// (#2890), which reuse its document primitives.
	FileFolder             filestoreModels.FolderRepository
	File                   filestoreModels.FileRepository
	AnnouncementAttachment filestoreModels.AnnouncementAttachmentRepository
	FileEvent              auditModels.FileEventRepository
	SubstitutionChange     auditModels.SubstitutionChangeCreator

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
	CrossTenant      CrossTenantQuery
	CombinedGroup    activeModels.CombinedGroupRepository
	GroupMapping     activeModels.GroupMappingRepository
	Attendance       activeModels.AttendanceRepository
	StudentStatusDay activeModels.StudentStatusDayOverviewRepository
	// Statistics serves the aggregate reads of the Statistik page (#2606).
	Statistics activeModels.StatisticsRepository
	// CourseStatistics serves the course participation section of the
	// Statistik page (#2891).
	CourseStatistics                scheduleModels.CourseStatisticsRepository
	ExcusedAbsenceRequest           activeModels.ExcusedAbsenceRequestRepository
	WorkSession                     activeModels.WorkSessionRepository
	WorkSessionBreak                activeModels.WorkSessionBreakRepository
	StaffAbsence                    activeModels.StaffAbsenceRepository
	StaffAbsenceAudit               activeModels.StaffAbsenceAuditRepository
	StaffAbsenceType                activeModels.StaffAbsenceTypeRepository
	StaffAbsenceTypeAllowance       activeModels.StaffAbsenceTypeAllowanceRepository
	StaffAbsenceTypeAllowanceChange activeModels.StaffAbsenceTypeAllowanceChangeRepository
	StaffVacationQuota              activeModels.StaffVacationQuotaRepository
	StaffVacationOpening            activeModels.StaffVacationOpeningRepository
	StaffBalanceAdjust              activeModels.StaffBalanceAdjustmentRepository
	StaffMonthSnapshot              activeModels.StaffMonthBalanceSnapshotRepository

	SessionStartLock activeModels.SessionStartLocker

	// IoT domain
	Device             iotModels.DeviceRepository
	PushSubscription   deliveryModels.PushSubscriptionRepository
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
	Operator                 platformModels.OperatorRepository
	Announcement             platformModels.AnnouncementRepository
	AnnouncementView         platformModels.AnnouncementViewRepository
	OperatorAuditLog         platformModels.OperatorAuditLogRepository
	OperatorEmailChangeToken platformModels.OperatorEmailChangeTokenRepository
	OperatorRefreshToken     platformModels.OperatorRefreshTokenRepository
	OperatorInvitationToken  platformModels.OperatorInvitationTokenRepository
	OperatorSummaries        platformModels.OperatorSummariesRepository
	School                   platformModels.SchoolRepository

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

	// Staff notices (Tagesinformationen: interne Hinweise der Leitung, #2180)
	StaffNotice userModels.StaffNoticeRepository
}

// NewAuditStore binds the Audit Postgres adapter to the transaction resolver
// owned by the composition root.
func (f *Factory) NewAuditStore(runtime audit.Runtime) auditModels.AppendStore {
	return NewAuditStore(runtime)
}

func NewAuditStore(runtime audit.Runtime) auditModels.AppendStore { return audit.NewAppender(runtime) }

// ListRecentAuditRetentionSummaries exposes the Audit-owned cleanup query
// without widening the producer-facing DataDeletionRepository interface.
func ListRecentAuditRetentionSummaries(ctx context.Context, db *bun.DB, since time.Time, limit int) ([]auditModels.RecentDeletionSummary, error) {
	runtime := audit.NewRuntime(db, auditModels.TenantIDFromContext)
	return audit.NewDataDeletionRepository(runtime).ListRecentRetentionSummaries(ctx, since, limit)
}

// ConfigureAuditRuntime binds every Audit query adapter to the caller's
// transaction, tenant, and read-only root DB fallback before services capture
// the interfaces. Audit writes are routed separately through the fail-closed
// command by RouteAuditWrites.
func (f *Factory) ConfigureAuditRuntime(runtime audit.Runtime) {
	f.FileEvent = audit.NewFileEventRepository(runtime)
	f.SubstitutionChange = audit.NewSubstitutionChangeRepository(runtime)
	f.DataDeletion = audit.NewDataDeletionRepository(runtime)
	f.StudentDeletionAudit = audit.NewStudentDeletionRepository(runtime)
	f.EnrollmentDeletionAudit = audit.NewEnrollmentDeletionRepository(runtime)
	f.EnrollmentRestorationAudit = audit.NewEnrollmentRestorationRepository(runtime)
	f.DataAccessLog = audit.NewDataAccessLogRepository(runtime)
	f.EnrollmentOfferingAdjustment = audit.NewEnrollmentOfferingAdjustmentRepository(runtime)
	f.GuardianChange = audit.NewGuardianChangeRepository(runtime)
	f.DeviationEvent = audit.NewDeviationEventRepository(runtime)
	f.AuthEvent = audit.NewAuthEventRepository(runtime)
	f.DataImport = audit.NewDataImportRepository(runtime)
	f.WorkSessionEdit = audit.NewWorkSessionEditRepository(runtime)
	f.StudentFieldEdit = audit.NewStudentFieldEditRepository(runtime)
	f.UnregisteredTagScan = audit.NewUnregisteredTagScanRepository(runtime)
	f.TimeTrackingDeletion = audit.NewTimeTrackingDeletionRepository(runtime)
	f.PersonnelNumberChange = audit.NewPersonnelNumberChangeRepository(runtime)
	f.StaffMasterDataChange = audit.NewStaffMasterDataChangeRepository(runtime)
	f.GuardianFinancialChange = audit.NewGuardianFinancialChangeRepository(runtime)
	f.ClassListEntryChange = audit.NewClassListEntryChangeRepository(runtime)
	f.TimeTrackingAuditLog = audit.NewTimeTrackingAuditLogRepository(runtime)
	f.BookingConsistency = audit.NewBookingConsistencyRepository(runtime)
	f.StudentDeletion = users.NewStudentDeletionRepository(f.db, f.StudentDeletionAudit.CountStudentReferences)
	f.EnrollmentDeletion = enrollment.NewDeletionRepository(f.db, f.EnrollmentOfferingAdjustment.CountForDeletion)
}

// BindOrganizationTenancy replaces school-owning and school-enriched legacy
// adapters with compositions over the public owner capability.
func (f *Factory) BindOrganizationTenancy(capability organizationtenancy.Capability) {
	if capability == nil {
		panic("repository factory: organization tenancy capability is required")
	}
	if f.organizationTenancyBound {
		return
	}
	f.organizationTenancyBound = true
	memberships := f.AccountTenant
	rawAccountTenant, ok := f.AccountTenant.(interface {
		ListAccountsBySchoolIDs(context.Context, []int64) ([]authModels.OrgAccountInfo, error)
	})
	if ok {
		f.AccountTenant = schoolAccountTenantRepository{AccountTenantRepository: f.AccountTenant, raw: rawAccountTenant, schools: capability}
	}
	if f.Account != nil {
		f.Account = schoolAccountRepository{AccountRepository: f.Account, schools: capability}
	}
	f.School = NewSchoolCapabilityAdapter(capability, memberships)
	if f.ParentChild != nil {
		f.ParentChild = schoolChildRepository{ChildRepository: f.ParentChild, schools: capability}
	}
	if f.ParentEnrollablePhase != nil {
		f.ParentEnrollablePhase = schoolEnrollablePhaseRepository{EnrollablePhaseRepository: f.ParentEnrollablePhase, schools: capability}
	}
	if f.ParentEnrollmentRequest != nil {
		f.ParentEnrollmentRequest = schoolEnrollmentRequestRepository{EnrollmentRequestRepository: f.ParentEnrollmentRequest, schools: capability}
	}
	if f.ParentAnnouncement != nil {
		f.ParentAnnouncement = schoolParentAnnouncementRepository{ParentAnnouncementRepository: f.ParentAnnouncement, schools: capability}
	}
	if f.ParentMessageRead != nil {
		f.ParentMessageRead = schoolParentMessageReadRepository{ParentMessageReadRepository: f.ParentMessageRead, schools: capability}
	}
}

// NewOrganizationTenancy composes the school owner behind the legacy
// composition seam. Consumers should depend on a narrow projection instead
// of importing the module's compose package themselves.
func NewOrganizationTenancy(db *bun.DB) (organizationtenancy.Capability, error) {
	return organizationCompose.New(organizationCompose.Dependencies{
		DB:      db,
		Observe: func(organizationCompose.Observation) {},
	})
}

// BindSchoolStructure replaces the group-enriched legacy adapters with
// compositions over the public School Structure query, so no repository
// outside the owner reads education.groups itself.
func (f *Factory) BindSchoolStructure(groups schoolstructure.Query) {
	if groups == nil {
		panic("repository factory: school structure query is required")
	}
	if f.schoolStructureBound {
		return
	}
	f.schoolStructureBound = true
	if f.Student != nil {
		f.Student = groupStudentRepository{StudentRepository: f.Student, groups: groups}
	}
	if f.ActiveVisit != nil {
		f.ActiveVisit = groupVisitRepository{VisitRepository: f.ActiveVisit, groups: groups}
	}
	if f.GroupSupervisor != nil {
		f.GroupSupervisor = groupSupervisorRepository{GroupSupervisorRepository: f.GroupSupervisor, groups: groups}
	}
	if f.CrossTenant != nil {
		f.CrossTenant = groupCrossTenantRepository{CrossTenantQuery: f.CrossTenant, groups: groups}
	}
	if withTargets, ok := f.ActivityGroup.(activityGroupTargets); ok {
		f.ActivityGroup = groupActivityGroupRepository{activityGroupTargets: withTargets, groups: groups}
	}
	if f.ParentMessageRead != nil {
		f.ParentMessageRead = groupParentMessageReadRepository{ParentMessageReadRepository: f.ParentMessageRead, groups: groups}
	}
}

// NewSchoolStructure composes the group owner behind the legacy composition
// seam for test graphs and CLI roots that do not record observations.
func NewSchoolStructure(db *bun.DB) (schoolstructure.Capability, error) {
	return schoolStructureCompose.New(schoolStructureCompose.Dependencies{
		DB:      db,
		Observe: func(schoolStructureCompose.Observation) {},
	})
}

// NewFactory creates a new repository factory with all repositories
func NewFactory(db *bun.DB, clocks ...func() time.Time) *Factory {
	var now func() time.Time
	if len(clocks) > 0 && clocks[0] != nil {
		now = clocks[0]
	}
	activityInstance := schedule.NewActivityInstanceRepository(db, now)
	groupSupervisor := active.NewGroupSupervisorRepository(db, now)
	attendance := active.NewAttendanceRepository(db, now)
	parentAnnouncement := users.NewParentAnnouncementRepository(db, now)
	auditRepositoryRuntime := func(ctx context.Context) (bun.IDB, int64) {
		tenantID := auditModels.TenantIDFromContext(ctx)
		if raw, ok := auditModels.TransactionFromContext(ctx); ok {
			switch tx := raw.(type) {
			case bun.Tx:
				return tx, tenantID
			case *bun.Tx:
				if tx != nil {
					return tx, tenantID
				}
			}
		}
		return db, tenantID
	}
	studentDeletionAudit := audit.NewStudentDeletionRepository(auditRepositoryRuntime)
	enrollmentOfferingAdjustment := audit.NewEnrollmentOfferingAdjustmentRepository(auditRepositoryRuntime)
	return &Factory{
		db: db,
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
		StudentDeletion:     users.NewStudentDeletionRepository(db, studentDeletionAudit.CountStudentReferences),
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

		CaregiverBindingLock: users.NewCaregiverBindingLocker(db),

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
		FileFolder:             filestore.NewFolderRepository(db),
		File:                   filestore.NewFileRepository(db),
		AnnouncementAttachment: filestore.NewAnnouncementAttachmentRepository(db),
		FileEvent:              audit.NewFileEventRepository(auditRepositoryRuntime),
		SubstitutionChange:     audit.NewSubstitutionChangeRepository(auditRepositoryRuntime),

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
		ActiveGroup:                     active.NewGroupRepository(db),
		ActiveVisit:                     active.NewVisitRepository(db),
		GroupSupervisor:                 groupSupervisor,
		CrossTenant:                     active.NewCrossTenantRepository(db),
		CombinedGroup:                   active.NewCombinedGroupRepository(db),
		GroupMapping:                    active.NewGroupMappingRepository(db),
		Attendance:                      attendance,
		StudentStatusDay:                active.NewStudentStatusDayRepository(db),
		Statistics:                      active.NewStatisticsRepository(db),
		CourseStatistics:                schedule.NewCourseStatisticsRepository(db),
		ExcusedAbsenceRequest:           active.NewExcusedAbsenceRequestRepository(db),
		WorkSession:                     active.NewWorkSessionRepository(db, now),
		WorkSessionBreak:                active.NewWorkSessionBreakRepository(db),
		StaffAbsence:                    active.NewStaffAbsenceRepository(db),
		StaffAbsenceAudit:               active.NewStaffAbsenceAuditRepository(db),
		StaffAbsenceType:                active.NewStaffAbsenceTypeRepository(db),
		StaffAbsenceTypeAllowance:       workforce.NewStaffAbsenceTypeAllowanceRepository(db),
		StaffAbsenceTypeAllowanceChange: workforce.NewStaffAbsenceTypeAllowanceChangeRepository(db),
		StaffVacationQuota:              active.NewStaffVacationQuotaRepository(db),
		StaffVacationOpening:            active.NewStaffVacationOpeningRepository(db),
		StaffBalanceAdjust:              active.NewStaffBalanceAdjustmentRepository(db),
		StaffMonthSnapshot:              active.NewStaffMonthBalanceSnapshotRepository(db),

		SessionStartLock: active.NewSessionStartLocker(db),

		// IoT repositories
		Device:             iot.NewDeviceRepository(db),
		PushSubscription:   deliveryCompose.NewPushSubscriptionRepository(db),
		PWAStandaloneUsage: iot.NewPWAStandaloneUsageRepository(db),

		// Config repositories
		SettingValue:      config.NewSettingValueRepository(config.NewRuntime(db)),
		SettingAudit:      config.NewSettingAuditRepository(config.NewRuntime(db)),
		StaffWorkSchedule: config.NewStaffWorkScheduleRepository(config.NewRuntime(db)),
		WorkTimeModel:     config.NewWorkTimeModelRepository(config.NewRuntime(db)),

		// Audit repositories
		DataDeletion:                 audit.NewDataDeletionRepository(auditRepositoryRuntime),
		StudentDeletionAudit:         studentDeletionAudit,
		EnrollmentDeletionAudit:      audit.NewEnrollmentDeletionRepository(auditRepositoryRuntime),
		EnrollmentRestorationAudit:   audit.NewEnrollmentRestorationRepository(auditRepositoryRuntime),
		DataAccessLog:                audit.NewDataAccessLogRepository(auditRepositoryRuntime),
		EnrollmentOfferingAdjustment: enrollmentOfferingAdjustment,
		GuardianChange:               audit.NewGuardianChangeRepository(auditRepositoryRuntime),
		DeviationEvent:               audit.NewDeviationEventRepository(auditRepositoryRuntime),
		AuthEvent:                    audit.NewAuthEventRepository(auditRepositoryRuntime),
		DataImport:                   audit.NewDataImportRepository(auditRepositoryRuntime),
		WorkSessionEdit:              audit.NewWorkSessionEditRepository(auditRepositoryRuntime),
		StudentFieldEdit:             audit.NewStudentFieldEditRepository(auditRepositoryRuntime),
		UnregisteredTagScan:          audit.NewUnregisteredTagScanRepository(auditRepositoryRuntime),
		TimeTrackingDeletion:         audit.NewTimeTrackingDeletionRepository(auditRepositoryRuntime),
		PersonnelNumberChange:        audit.NewPersonnelNumberChangeRepository(auditRepositoryRuntime),
		StaffMasterDataChange:        audit.NewStaffMasterDataChangeRepository(auditRepositoryRuntime),
		GuardianFinancialChange:      audit.NewGuardianFinancialChangeRepository(auditRepositoryRuntime),
		ClassListEntryChange:         audit.NewClassListEntryChangeRepository(auditRepositoryRuntime),
		TimeTrackingAuditLog:         audit.NewTimeTrackingAuditLogRepository(auditRepositoryRuntime),
		BookingConsistency:           audit.NewBookingConsistencyRepository(auditRepositoryRuntime),

		// Platform repositories
		Operator:                 platformRepo.NewOperatorRepository(db),
		Announcement:             platformRepo.NewAnnouncementRepository(db),
		AnnouncementView:         platformRepo.NewAnnouncementViewRepository(db),
		OperatorAuditLog:         platformRepo.NewOperatorAuditLogRepository(db),
		OperatorEmailChangeToken: platformRepo.NewOperatorEmailChangeTokenRepository(db),
		OperatorRefreshToken:     platformRepo.NewOperatorRefreshTokenRepository(db),
		OperatorInvitationToken:  platformRepo.NewOperatorInvitationTokenRepository(db),
		OperatorSummaries:        platformRepo.NewOperatorSummariesRepository(db),
		School:                   platformRepo.NewSchoolRepository(db),

		OperatorMFACredential:     platformRepo.NewOperatorMFACredentialRepository(db),
		OperatorMFAEmailChallenge: platformRepo.NewOperatorMFAEmailChallengeRepository(db),
		OperatorMFATrustedDevice:  platformRepo.NewOperatorMFATrustedDeviceRepository(db),
		OperatorPasskeyCredential: platformRepo.NewOperatorPasskeyCredentialRepository(db),
		OperatorPasskeySession:    platformRepo.NewOperatorPasskeySessionRepository(db),

		// Enrollment repositories
		FormSchema:            enrollment.NewFormSchemaRepository(db),
		Request:               enrollment.NewRequestRepository(db),
		EnrollmentDeletion:    enrollment.NewDeletionRepository(db, enrollmentOfferingAdjustment.CountForDeletion),
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
		ParentAnnouncement:                parentAnnouncement,
		StaffNotice:                       schedule.NewStaffNoticeRepository(db),
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
