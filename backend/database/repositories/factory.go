package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/audit"
	"github.com/moto-nrw/project-phoenix/database/repositories/config"
	"github.com/moto-nrw/project-phoenix/database/repositories/education"
	parentRepo "github.com/moto-nrw/project-phoenix/database/repositories/parent"
	"github.com/moto-nrw/project-phoenix/database/repositories/pwausage"
	"github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/modules/appointments"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	staffStore "github.com/moto-nrw/project-phoenix/modules/communication/staffstore"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	devicefleetRepositoryAdapter "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose/repositoryadapter"
	enrollmentCapability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesRepositoryAdapter "github.com/moto-nrw/project-phoenix/modules/facilities/compose/repositoryadapter"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	workforceCapability "github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/adapters/timerecords"
	workforceRepositoryAdapter "github.com/moto-nrw/project-phoenix/modules/workforce/compose/repositoryadapter"
	workforceLegacy "github.com/moto-nrw/project-phoenix/modules/workforce/legacy"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	deliveryModels "github.com/moto-nrw/project-phoenix/models/delivery"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	facilityModels "github.com/moto-nrw/project-phoenix/models/facilities"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"

	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/uptrace/bun"
)

// Factory provides access to all repositories
type Factory struct {
	db                       *bun.DB
	organizationTenancyBound bool
	peopleDirectoryBound     bool
	carePlanBound            bool
	schoolStructureBound     bool
	schoolMembershipBound    bool
	facilitiesBound          bool
	appointmentsBound        bool
	// roomBinders hand a room owner to the raw repositories that used to
	// join facilities.rooms; kept so BindFacilities reaches them after the
	// projections wrapped the factory fields (#2665).
	roomBinders         []func(facilitiesModule.Query)
	schoolCalendarBound bool
	// schoolCalendar is the bound capability behind the calendar period,
	// closing day and dateframe adapters (#2666).
	schoolCalendar schoolcalendar.Calendar
	appointments   appointments.Capability
	carePlan       careplan.Capability

	// students is the bound People Directory; audit adapters rebuilt by
	// ConfigureAuditRuntime after the binding read it again (#2662).
	// membershipDeps carries the owner-side lookups the staff, teacher and
	// guest adapters compose with; it survives every rebinding.
	membershipDeps *staffMembershipDeps
	// schoolMembership is the bound capability, exposed to the service graph
	// through SchoolMembership().
	schoolMembership schoolmembership.Capability
	students         peopledirectory.Capability

	// Users domain
	Person              userModels.PersonRepository
	RFIDCard            identityaccess.RFIDCards
	Staff               userModels.StaffRepository
	Student             userModels.StudentRepository
	CareWithdrawal      userModels.CareWithdrawalCompletionRepository
	Teacher             userModels.TeacherRepository
	Guest               userModels.GuestRepository
	Profile             identityaccess.AccountProfiles
	StudentGuardian     userModels.StudentGuardianRepository
	GuardianProfile     userModels.GuardianProfileRepository
	GuardianPhoneNumber userModels.GuardianPhoneNumberRepository
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
	StaffDocument userModels.StaffDocumentRepository

	// School file storage trail (#2596); folders, files and attachments are
	// owned by modules/filestorage (#2707).
	FileEvent          auditModels.FileEventRepository
	SubstitutionChange auditModels.SubstitutionChangeCreator

	// Facilities domain
	Room facilityModels.RoomRepository

	// Education domain
	Group            educationModels.GroupRepository
	GroupTeacher     educationModels.GroupTeacherRepository
	ClassTeacher     educationModels.ClassTeacherRepository
	ClassArrivalTime educationModels.ClassArrivalTimeRepository
	// ClassArrivalException holds class-wide arrival day exceptions (#2962).
	ClassArrivalException scheduleModels.ClassArrivalExceptionRepository
	GroupSubstitution     educationModels.GroupSubstitutionRepository

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
	PlanningTrack             scheduleModels.PlanningTrackRepository
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
	ActiveGroup           studentpresence.SessionRecords
	GroupSupervisor       studentpresence.SupervisionRecords
	CrossTenant           CrossTenantQuery
	StudentStatusDay      *StudentStatusDayRepository
	ExcusedAbsenceRequest *ExcusedAbsenceRequestRepository
	WorkSession           timerecords.WorkSessionRepository
	WorkSessionBreak      timerecords.WorkSessionBreakRepository
	StaffAbsence          timerecords.StaffAbsenceRepository
	StaffAbsenceAudit     timerecords.StaffAbsenceAuditRepository
	StaffAbsenceType      workforceCapability.AbsenceTypeQuery
	StaffVacationQuota    timerecords.StaffVacationQuotaRepository
	StaffVacationOpening  timerecords.StaffVacationOpeningRepository
	StaffBalanceAdjust    timerecords.StaffBalanceAdjustmentRepository
	StaffMonthSnapshot    workforceCapability.MonthSnapshots

	SessionStartLock interface {
		LockSessionStart(context.Context, int64) error
	}

	// IoT domain
	Device             iotModels.DeviceRepository
	PushSubscription   deliveryModels.PushSubscriptionRepository
	PWAStandaloneUsage *pwausage.PWAStandaloneUsageRepository

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
	StudentConsentChange         auditModels.StudentConsentChangeRepository
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
	// The operator invitation and e-mail change links are owned by Identity
	// & Access (#2722).
	OperatorAuditLog platformModels.OperatorAuditLogRepository
	// School is the Organisation & Tenancy capability that owns
	// platform.schools (#3253); BindOrganizationTenancy replaces the
	// unobserved default with the serving root's module.
	School organizationtenancy.Capability
	// The operator MFA and passkey records are owned by Identity & Access
	// (#2723, #2724).

	// Enrollment domain (parent-enrollment PR 5+)
	SubmissionRateLimit *enrollmentCapability.Module

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
	CalendarStaffFeedTombstone schoolcalendar.FeedHistory

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
	f.StudentConsentChange = audit.NewStudentConsentChangeRepository(runtime)
	f.DeviationEvent = audit.NewDeviationEventRepository(runtime)
	f.AuthEvent = audit.NewAuthEventRepository(runtime)
	f.DataImport = audit.NewDataImportRepository(runtime)
	f.WorkSessionEdit = audit.NewWorkSessionEditRepository(runtime)
	f.StudentFieldEdit = audit.NewStudentFieldEditRepository(runtime)
	f.UnregisteredTagScan = NewUnregisteredTagScanRepository(mustNewDeviceFleet(f.db))
	f.TimeTrackingDeletion = audit.NewTimeTrackingDeletionRepository(runtime)
	f.PersonnelNumberChange = audit.NewPersonnelNumberChangeRepository(runtime)
	f.StaffMasterDataChange = audit.NewStaffMasterDataChangeRepository(runtime)
	f.GuardianFinancialChange = audit.NewGuardianFinancialChangeRepository(runtime)
	f.ClassListEntryChange = audit.NewClassListEntryChangeRepository(runtime)
	f.TimeTrackingAuditLog = audit.NewTimeTrackingAuditLogRepository(runtime)
	f.BookingConsistency = audit.NewBookingConsistencyRepository(runtime, NewEnrollmentBookingProjection(enrollmentCompose.New()))
	f.bindAuditStudentDirectory()
	f.bindCarePlanAuditDirectory()
	if f.students != nil {
		f.bindGuardianDirectories(f.students)
	}
}

// NewAttendanceCorrectionRepository composes the append-only attendance trail
// with the request-scoped audit runtime used by timetable corrections.
func NewAttendanceCorrectionRepository(runtime audit.Runtime) auditModels.AttendanceCorrectionRepository {
	return audit.NewAttendanceCorrectionRepository(runtime)
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
	f.School = capability
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

func mustNewOrganizationTenancy(db *bun.DB) organizationtenancy.Capability {
	capability, err := NewOrganizationTenancy(db)
	if err != nil {
		panic(fmt.Sprintf("repository factory: organization tenancy: %v", err))
	}
	return capability
}

// BindPeopleDirectory replaces the legacy adapters that used to join
// users.persons themselves with compositions over the public People
// Directory capability (#2661).
func (f *Factory) BindPeopleDirectory(capability peopledirectory.Capability) {
	if capability == nil {
		panic("repository factory: people directory capability is required")
	}
	if f.peopleDirectoryBound {
		return
	}
	f.peopleDirectoryBound = true
	f.students = capability
	// Students first: the observed directory replaces the default binding on
	// the raw repositories before the person projections wrap them.
	f.bindStudentDirectories(capability, capability)
	f.bindGuardianDirectories(capability)
	f.bindPersonProjections(capability)
	if f.carePlan != nil {
		f.bindCarePlanAdapters(f.carePlan)
	}
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
	if f.GroupSupervisor != nil {
		f.GroupSupervisor = groupSupervisorRepository{SupervisionRecords: f.GroupSupervisor, groups: groups}
	}
	if f.CrossTenant != nil {
		f.CrossTenant = groupCrossTenantRepository{CrossTenantQuery: f.CrossTenant, groups: groups}
	}
	if f.ActivityGroup != nil {
		withTargets, ok := f.ActivityGroup.(activityGroupTargets)
		if !ok {
			panic(fmt.Sprintf("repository factory: activity group repository %T must also serve group targets", f.ActivityGroup))
		}
		f.ActivityGroup = groupActivityGroupRepository{activityGroupTargets: withTargets, groups: groups}
	}
	if f.ParentMessageRead != nil {
		f.ParentMessageRead = groupParentMessageReadRepository{ParentMessageReadRepository: f.ParentMessageRead, groups: groups}
	}
}

// BindSchoolMembership replaces the staff, teacher, guest, class-list-entry
// and teaching-assignment adapters — and every legacy repository that used to
// join membership tables itself — with compositions over the observed School
// Membership capability (#2667, #2668, #2669). The factory already works
// without it (NewFactory composes an unobserved module), so this binding is
// about runtime evidence, not about correctness.
func (f *Factory) BindSchoolMembership(capability schoolmembership.Capability) {
	if capability == nil {
		panic("repository factory: school membership capability is required")
	}
	if f.schoolMembershipBound {
		return
	}
	f.schoolMembershipBound = true
	f.bindStaffMembershipAdapters(capability)
}

// SchoolMembership returns the capability the staff adapters read through.
func (f *Factory) SchoolMembership() schoolmembership.Capability { return f.schoolMembership }

// bindStaffMembershipAdapters points every membership-derived repository at the
// given capability.
func (f *Factory) bindStaffMembershipAdapters(capability schoolmembership.Capability) {
	f.schoolMembership = capability
	f.GroupTeacher = newGroupTeacherRepository(capability, f.Group)
	f.ClassTeacher = newClassTeacherRepository(capability)
	f.Staff = staffMembershipRepository{membership: capability, deps: f.membershipDeps}
	f.Teacher = teacherMembershipRepository{membership: capability, deps: f.membershipDeps}
	f.Guest = guestMembershipRepository{membership: capability}
}

// NewFactory creates a new repository factory with all repositories
func NewFactory(db *bun.DB, timetableDependencies TimetableDependencies, clocks ...func() time.Time) *Factory {
	if timetableDependencies.Capability == nil || timetableDependencies.Students == nil || timetableDependencies.Groups == nil || timetableDependencies.Rooms == nil || timetableDependencies.Calendar == nil || timetableDependencies.Membership == nil || timetableDependencies.Workforce == nil {
		panic("repository factory: timetable and projection dependencies are required")
	}
	timetableCapability := timetableDependencies.Capability
	presenceCapability := newStudentPresence(db)
	var now func() time.Time
	if len(clocks) > 0 && clocks[0] != nil {
		now = clocks[0]
	}
	deviceFleet := mustNewDeviceFleet(db)
	sessionRecords := newPresenceSessionRecords(presenceCompose.SessionRecordDependencies{
		DB: db, Now: now, Devices: activeDeviceDirectory{devices: deviceFleet}, Rooms: &activeRoomDirectory{},
		Activities: NewSessionActivities(timetableActivityGroupRepository{timetable: timetableCapability}),
		Staff:      &presenceSupervisionStaff{},
	})
	enrollmentModule := enrollmentCompose.New()
	parentAnnouncement := NewParentAnnouncementRepository(db, enrollmentModule, now)
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
	parentRuntime := parentRepo.RuntimeFunc(carePlanCompose.TenantAmbientDatabase(db))
	appointmentsModule, err := NewAppointments(db)
	if err != nil {
		panic(fmt.Sprintf("repository factory: compose appointments: %v", err))
	}
	studentDeletionAudit := audit.NewStudentDeletionRepository(auditRepositoryRuntime)
	enrollmentOfferingAdjustment := audit.NewEnrollmentOfferingAdjustmentRepository(auditRepositoryRuntime)
	// The account facts other owners read belong to Identity & Access
	// (#2720): the account lookups the People Directory and Care Plan
	// repositories need are bound at construction.
	identity := newIdentityAccess(db, timetableDependencies.ObserveIdentityAccess)
	personRepo := NewPersonRepository(db)
	studentRepo := NewStudentRepository(db)
	groupRepo := education.NewGroupRepository(db)
	factory := &Factory{
		db: db,
		// Users repositories
		Person:              personRepo,
		RFIDCard:            identity,
		Student:             studentRepo,
		Profile:             identity,
		StudentGuardian:     newGuardianRelationships(db, identity.FindActiveSchoolMemberships, timetableDependencies.ObserveIdentityAccess),
		GuardianProfile:     NewGuardianProfileRepository(db),
		GuardianPhoneNumber: users.NewGuardianPhoneNumberRepository(db),
		FamilyProtection:    users.NewFamilyProtectionEventRepository(db),
		ParentRequestShare:  users.NewParentRequestShareEventRepository(db),
		ParentRequestEvent:  users.NewParentRequestEventRepository(db),

		// The caregiver blocker re-check locks four owners' binding tables;
		// each owner takes its own table lock through its public capability.
		CaregiverBindingLock: users.NewCaregiverBindingLocker(NewCaregiverBindingOwners(timetableDependencies, presenceCapability)),

		// Staff Stammdaten (#1423) belong to Workforce (#2690): the retained
		// contracts are served by the adapters over the one facade.
		StaffMasterData:    workforceLegacy.NewStaffMasterDataRepository(timetableDependencies.Workforce),
		StaffQualification: workforceLegacy.NewStaffQualificationRepository(timetableDependencies.Workforce),
		StaffFinancialData: workforceLegacy.NewStaffFinancialDataRepository(timetableDependencies.Workforce),

		// Guardian payment data (#2608)
		GuardianFinancialData: users.NewGuardianFinancialDataRepository(db),

		// Staff documents (#1424) — StaffDocument is bound by
		// bindStaffMembershipDecorators, it needs the membership owner.

		// School file storage trail (#2596)
		FileEvent:          audit.NewFileEventRepository(auditRepositoryRuntime),
		SubstitutionChange: audit.NewSubstitutionChangeRepository(auditRepositoryRuntime),

		// Facilities repositories
		Room: facilitiesRepositoryAdapter.New(),

		// Education repositories
		Group:                 groupRepo,
		ClassArrivalTime:      education.NewClassArrivalTimeRepository(db),
		ClassArrivalException: timetableCompose.NewClassArrivalExceptionRepository(db),
		GroupSubstitution:     nil, // bound to Workforce below

		// Schedule repositories. Dateframe, CalendarPeriod and ClosingDay
		// belong to School Calendar and are bound below.
		Timeframe:                 nil, // bound to Timetable below
		RecurrenceRule:            nil, // bound to Timetable below
		StudentPickupSchedule:     nil, // bound to Care Plan below
		StudentPickupException:    nil, // bound to Care Plan below
		StudentPickupNote:         nil, // bound to Care Plan below
		StudentArrivalSchedule:    nil, // bound to Care Plan below
		StudentArrivalException:   nil, // bound to Care Plan below
		StudentArrivalNote:        nil, // bound to Care Plan below
		CareScheduleChangeRequest: nil, // bound to Care Plan below
		PlanningTrack:             nil, // bound to Timetable below
		ActivityInstance:          nil, // bound to Timetable below
		InstanceIdempotency:       nil, // bound to Timetable below
		InstanceStaff:             nil, // bound to Timetable below
		InstanceStudent:           newTimetableInstanceStudentRepository(db, timetableCapability, presenceCapability),
		ActivityException:         nil, // bound to Timetable below

		// Activities repositories
		ActivityGroup:      nil, // bound to Timetable below
		ActivityCategory:   nil, // bound to Timetable below
		ActivitySchedule:   nil, // bound to Timetable below
		ActivitySupervisor: nil, // bound to Timetable below
		StudentEnrollment:  nil, // bound to Timetable below

		// Active repositories
		ActiveGroup:           sessionRecords,
		GroupSupervisor:       sessionRecords,
		CrossTenant:           &visitorProjection{visits: presenceCapability},
		StudentStatusDay:      nil, // bound to Care Plan below
		ExcusedAbsenceRequest: nil, // bound to Care Plan below
		// Work sessions, breaks, balances and vacation rows belong to
		// Workforce (#2690); the facade carries its own clock.
		WorkSession:          workforceLegacy.NewWorkSessionRepository(timetableDependencies.Workforce),
		WorkSessionBreak:     workforceLegacy.NewWorkSessionBreakRepository(timetableDependencies.Workforce),
		StaffAbsence:         workforceLegacy.NewStaffAbsenceRepository(timetableDependencies.Workforce),
		StaffAbsenceAudit:    workforceLegacy.NewStaffAbsenceAuditRepository(timetableDependencies.Workforce),
		StaffAbsenceType:     timetableDependencies.Workforce,
		StaffVacationQuota:   workforceLegacy.NewStaffVacationQuotaRepository(timetableDependencies.Workforce),
		StaffVacationOpening: workforceLegacy.NewStaffVacationOpeningRepository(timetableDependencies.Workforce),
		StaffBalanceAdjust:   workforceLegacy.NewStaffBalanceAdjustmentRepository(timetableDependencies.Workforce),
		StaffMonthSnapshot:   timetableDependencies.Workforce,

		SessionStartLock: presenceCapability,

		// IoT repositories
		Device:             devicefleetRepositoryAdapter.NewDeviceRepository(deviceFleet),
		PushSubscription:   deliveryCompose.NewPushSubscriptionRepository(db),
		PWAStandaloneUsage: pwausage.NewPWAStandaloneUsageRepository(db),

		// Config repositories
		SettingValue:      config.NewSettingValueRepository(config.NewRuntime(db)),
		SettingAudit:      config.NewSettingAuditRepository(config.NewRuntime(db)),
		StaffWorkSchedule: workforceRepositoryAdapter.NewStaffWorkScheduleRepository(timetableDependencies.Workforce),
		WorkTimeModel:     workforceRepositoryAdapter.NewWorkTimeModelRepository(timetableDependencies.Workforce),

		// Audit repositories
		DataDeletion:                 audit.NewDataDeletionRepository(auditRepositoryRuntime),
		StudentDeletionAudit:         studentDeletionAudit,
		EnrollmentDeletionAudit:      audit.NewEnrollmentDeletionRepository(auditRepositoryRuntime),
		EnrollmentRestorationAudit:   audit.NewEnrollmentRestorationRepository(auditRepositoryRuntime),
		DataAccessLog:                audit.NewDataAccessLogRepository(auditRepositoryRuntime),
		EnrollmentOfferingAdjustment: enrollmentOfferingAdjustment,
		GuardianChange:               audit.NewGuardianChangeRepository(auditRepositoryRuntime),
		StudentConsentChange:         audit.NewStudentConsentChangeRepository(auditRepositoryRuntime),
		DeviationEvent:               audit.NewDeviationEventRepository(auditRepositoryRuntime),
		AuthEvent:                    audit.NewAuthEventRepository(auditRepositoryRuntime),
		DataImport:                   audit.NewDataImportRepository(auditRepositoryRuntime),
		WorkSessionEdit:              audit.NewWorkSessionEditRepository(auditRepositoryRuntime),
		StudentFieldEdit:             audit.NewStudentFieldEditRepository(auditRepositoryRuntime),
		UnregisteredTagScan:          NewUnregisteredTagScanRepository(deviceFleet),
		TimeTrackingDeletion:         audit.NewTimeTrackingDeletionRepository(auditRepositoryRuntime),
		PersonnelNumberChange:        audit.NewPersonnelNumberChangeRepository(auditRepositoryRuntime),
		StaffMasterDataChange:        audit.NewStaffMasterDataChangeRepository(auditRepositoryRuntime),
		GuardianFinancialChange:      audit.NewGuardianFinancialChangeRepository(auditRepositoryRuntime),
		ClassListEntryChange:         audit.NewClassListEntryChangeRepository(auditRepositoryRuntime),
		TimeTrackingAuditLog:         audit.NewTimeTrackingAuditLogRepository(auditRepositoryRuntime),
		BookingConsistency:           audit.NewBookingConsistencyRepository(auditRepositoryRuntime, NewEnrollmentBookingProjection(enrollmentModule)),

		// Platform repositories. Operators and their refresh sessions are
		// served by the public Identity & Access capability the service root
		// binds (#3252); the operator audit ledger belongs to Audit (#2720).
		OperatorAuditLog: newOperatorAuditLog(auditRepositoryRuntime),
		School:           mustNewOrganizationTenancy(db),

		// Enrollment repositories
		SubmissionRateLimit: enrollmentModule,

		// Parent (cross-tenant guardian portal — PR 9+)
		ParentChild:             parentRepo.NewChildRepository(identity.ListActiveAccountSchoolIDs),
		ParentEnrollablePhase:   parentRepo.NewEnrollablePhaseRepository(enrollmentModule, identity.ListActiveAccountSchoolIDs),
		ParentEnrollmentRequest: parentRepo.NewEnrollmentRequestRepository(parentRuntime, enrollmentModule, identityAccountDirectory{accounts: identity}),

		// Parent Stammdaten direct-edit audit + change-request review
		StudentDataChangeRequest: nil, // bound to Care Plan below

		// Parent-OGS messaging (tenant-scoped two-way conversation per child)
		ParentMessageThread: parentStore.NewParentMessageThreadRepository(db, users.NewMessageableGuardianRepository(db, identity.FindActiveSchoolMemberships)),
		ParentMessage:       parentStore.NewParentMessageRepository(db),
		// ParentMessageRead and StaffMessageRead are bound by
		// bindStaffMembershipDecorators, they need the membership owner.

		StaffMessageThread: staffStore.NewStaffMessageThreadRepository(db),
		StaffMessage:       staffStore.NewStaffMessageRepository(db),

		// Calendar repositories
		CalendarStaffFeedTombstone: schoolCalendarCompose.NewFeedHistory(db),
		ParentAnnouncement:         parentAnnouncement,
		StaffNotice:                timetableCompose.NewStaffNoticeRepository(db),
	}
	// Care withdrawal completions belong to Care Plan (#3221); the adapter
	// follows the factory's current Care Plan and People Directory bindings.
	factory.CareWithdrawal = newCareWithdrawalCompletionRepository(
		func() careplan.Capability { return factory.carePlan },
		func() peopledirectory.StudentQuery { return factory.students },
	)
	factory.appointments = appointmentsModule
	studentRepo.(interface {
		BindTeacherGroupIDs(func(context.Context, int64) ([]int64, error))
	}).BindTeacherGroupIDs(func(ctx context.Context, teacherID int64) ([]int64, error) {
		assignments, err := factory.schoolMembership.ListGroupAssignments(ctx, schoolmembership.GroupAssignmentFilter{TeacherIDs: []int64{teacherID}})
		if err != nil {
			return nil, err
		}
		ids := make([]int64, 0, len(assignments))
		for _, assignment := range assignments {
			ids = append(ids, assignment.GroupID)
		}
		return ids, nil
	})
	studentRepo.(interface {
		BindTeacherStaffGroupIDs(func(context.Context, []int64) ([]int64, error))
	}).BindTeacherStaffGroupIDs(func(ctx context.Context, staffIDs []int64) ([]int64, error) {
		assignments, err := factory.schoolMembership.ListGroupAssignments(ctx, schoolmembership.GroupAssignmentFilter{TeacherStaffIDs: staffIDs})
		if err != nil {
			return nil, err
		}
		ids := make([]int64, 0, len(assignments))
		for _, assignment := range assignments {
			ids = append(ids, assignment.GroupID)
		}
		return ids, nil
	})
	groupRepo.(*education.GroupRepository).BindTeachingAssignments(func(ctx context.Context, groupIDs, teacherIDs []int64) ([]education.TeacherGroupID, error) {
		assignments, err := factory.schoolMembership.ListGroupAssignments(ctx, schoolmembership.GroupAssignmentFilter{GroupIDs: groupIDs, TeacherIDs: teacherIDs})
		if err != nil {
			return nil, err
		}
		result := make([]education.TeacherGroupID, 0, len(assignments))
		for _, assignment := range assignments {
			result = append(result, education.TeacherGroupID{TeacherID: assignment.TeacherID, GroupID: assignment.GroupID})
		}
		return result, nil
	})
	// Group substitutions belong to Workforce (#2688): the retained contract
	// is served by the adapter, which resolves groups through School
	// Structure and staff through School Membership.
	factory.GroupSubstitution = workforceLegacy.NewGroupSubstitutionRepository(timetableDependencies.Workforce,
		func(ctx context.Context, ids []int64) (map[int64]*educationModels.Group, error) {
			return factory.Group.FindByIDs(ctx, ids)
		},
		substitutionStaffResolver(lazyStaffLookup{get: func() schoolmembership.Capability { return factory.schoolMembership }}),
	)
	factory.bindAppointments(appointmentsModule)
	// Bind student ports while their repositories are still raw. The staff
	// projections below wrap some of the same repositories.
	factory.bindDefaultPeopleDirectory(db)
	factory.bindDefaultFacilities(db)
	carePlan, err := NewCarePlan(db, factory.students, factory.InstanceStudent)
	if err != nil {
		panic(fmt.Sprintf("repository factory: compose care plan: %v", err))
	}
	factory.bindCarePlanAdapters(carePlan)
	factory.membershipDeps = newStaffMembershipDeps(personRepo, identity, MustNewStaffEmployment(db))
	// Staff, teachers and guests belong to School Membership. Without an
	// explicit binding the factory composes an unobserved module so every
	// legacy consumer — repository tests and CLI roots included — reads the
	// same rows through the same owner.
	membership, err := NewSchoolMembership(db)
	if err != nil {
		panic(fmt.Sprintf("repository factory: compose school membership: %v", err))
	}
	factory.membershipDeps.groupTeachers = func() educationModels.GroupTeacherRepository { return factory.GroupTeacher }
	factory.bindStaffMembershipAdapters(membership)
	// Calendar periods, closing days and dateframes belong to School
	// Calendar; the unobserved default keeps every legacy consumer on the
	// owner until the production root binds the observed module (#2666).
	calendar, err := NewSchoolCalendar(db)
	if err != nil {
		panic(fmt.Sprintf("repository factory: compose school calendar: %v", err))
	}
	factory.bindSchoolCalendarAdapters(calendar, NewCalendarPeriodUsage(enrollmentCompose.New(), timetableCapability))
	// The decorators are wired once, innermost: they read the capability
	// lazily so a later BindSchoolMembership swap reaches them too, and they
	// stay under the school/person/group wrappers bound afterwards.
	factory.bindStaffMembershipDecorators(timetableDependencies.Workforce, identity)
	// Same lazy capability for the repositories that used to join users.staff
	// or users.teachers themselves; wired here so they sit inside the person,
	// school and group wrappers bound afterwards (#2667, agent A2).
	factory.bindStaffProjections(lazyStaffLookup{
		get: func() schoolmembership.Capability { return factory.schoolMembership },
	}, timetableDependencies.Workforce)
	adapters := newTimetableRepositories(db, timetableCapability, presenceCapability, timetableDependencies.Students, timetableDependencies.Groups, timetableDependencies.Rooms, timetableDependencies.Calendar, timetableDependencies.Membership, timetableDependencies.Workforce)
	factory.ActivityCategory, factory.ActivityGroup = adapters.ActivityCategory, adapters.ActivityGroup
	factory.ActivitySchedule, factory.ActivitySupervisor = adapters.ActivitySchedule, adapters.ActivitySupervisor
	factory.StudentEnrollment, factory.Timeframe = adapters.StudentEnrollment, adapters.Timeframe
	factory.PlanningTrack, factory.RecurrenceRule = adapters.PlanningTrack, adapters.RecurrenceRule
	factory.ActivityException, factory.ActivityInstance = adapters.ActivityException, adapters.ActivityInstance
	factory.InstanceIdempotency, factory.InstanceStaff = adapters.InstanceIdempotency, adapters.InstanceStaff
	return factory
}

// SetConfigRuntime replaces the bootstrap settings repositories with
// tenant-aware instances before the service graph captures them.
//
// The work-time repositories are deliberately not rebound: they are Workforce
// capability adapters now, and that owner resolves the tenant transaction from
// the context itself (#2687). Passing a custom runtime here does not, and must
// not, reach them.
func (f *Factory) SetConfigRuntime(runtime config.Runtime) {
	f.SettingValue = config.NewSettingValueRepository(runtime)
	f.SettingAudit = config.NewSettingAuditRepository(runtime)
}

func (r *Factory) Enrollment() EnrollmentBookingProjection {
	return NewEnrollmentBookingProjection(r.SubmissionRateLimit)
}
