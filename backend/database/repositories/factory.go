package repositories

import (
	"context"
	"fmt"
	"time"

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
	education "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
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
	facilityModels "github.com/moto-nrw/project-phoenix/models/facilities"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
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

	// students is the bound People Directory (#2662).
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

	// Facilities domain
	Room facilityModels.RoomRepository

	// Education domain
	Group        EducationGroupRepository
	GroupTeacher GroupTeacherRepository
	ClassTeacher ClassTeacherRepository
	// ClassArrivalException holds class-wide arrival day exceptions (#2962).
	ClassArrivalException scheduleModels.ClassArrivalExceptionRepository
	GroupSubstitution     GroupSubstitutionRepository

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
	StaffWorkSchedule configModels.StaffWorkScheduleRepository
	WorkTimeModel     configModels.WorkTimeModelRepository

	// UnregisteredTagScan is served by Device Fleet (#2676).
	UnregisteredTagScan auditModels.UnregisteredTagScanRepository

	// Platform domain (operator dashboard)
	// The operator invitation and e-mail change links are owned by Identity
	// & Access (#2722); the operator audit ledger is built by the Audit
	// composition (ADR 0049).
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
	parentRuntime := parentRepo.RuntimeFunc(carePlanCompose.TenantAmbientDatabase(db))
	appointmentsModule, err := NewAppointments(db)
	if err != nil {
		panic(fmt.Sprintf("repository factory: compose appointments: %v", err))
	}
	// The account facts other owners read belong to Identity & Access
	// (#2720): the account lookups the People Directory and Care Plan
	// repositories need are bound at construction.
	identity := newIdentityAccess(db, timetableDependencies.ObserveIdentityAccess)
	personRepo := NewPersonRepository(db)
	studentRepo := NewStudentRepository(db)
	var factory *Factory
	groupRepo := NewEducationGroupRepository(db, education.GroupRepositoryDependencies{
		TeachingAssignments: func(ctx context.Context, groupIDs, teacherIDs []int64) ([]education.TeacherGroupID, error) {
			assignments, err := factory.schoolMembership.ListGroupAssignments(ctx, schoolmembership.GroupAssignmentFilter{GroupIDs: groupIDs, TeacherIDs: teacherIDs})
			if err != nil {
				return nil, err
			}
			result := make([]education.TeacherGroupID, 0, len(assignments))
			for _, assignment := range assignments {
				result = append(result, education.TeacherGroupID{TeacherID: assignment.TeacherID, GroupID: assignment.GroupID})
			}
			return result, nil
		},
		SupervisingStaff: func(ctx context.Context, pairs education.GroupMembershipPairs) ([]education.StaffGroupID, error) {
			return supervisionStaffResolver(lazyStaffLookup{get: func() schoolmembership.Capability { return factory.schoolMembership }}, workforceSubstitutedStaff(timetableDependencies.Workforce))(ctx, pairs)
		},
	})
	factory = &Factory{
		db: db,
		// Users repositories
		Person:              personRepo,
		RFIDCard:            identity,
		Student:             studentRepo,
		Profile:             identity,
		StudentGuardian:     newGuardianRelationships(db, identity.FindActiveSchoolMemberships, timetableDependencies.ObserveIdentityAccess),
		GuardianProfile:     NewGuardianProfileRepository(db),
		GuardianPhoneNumber: users.NewGuardianPhoneNumberRepository(peopleRuntime(db)),
		FamilyProtection:    users.NewFamilyProtectionEventRepository(peopleRuntime(db)),
		ParentRequestShare:  users.NewParentRequestShareEventRepository(peopleRuntime(db)),
		ParentRequestEvent:  users.NewParentRequestEventRepository(peopleRuntime(db)),

		// The caregiver blocker re-check locks four owners' binding tables;
		// each owner takes its own table lock through its public capability.
		CaregiverBindingLock: users.NewCaregiverBindingLocker(NewCaregiverBindingOwners(timetableDependencies, presenceCapability)),

		// Staff Stammdaten (#1423) belong to Workforce (#2690): the retained
		// contracts are served by the adapters over the one facade.
		StaffMasterData:    workforceLegacy.NewStaffMasterDataRepository(timetableDependencies.Workforce),
		StaffQualification: workforceLegacy.NewStaffQualificationRepository(timetableDependencies.Workforce),
		StaffFinancialData: workforceLegacy.NewStaffFinancialDataRepository(timetableDependencies.Workforce),

		// Guardian payment data (#2608)
		GuardianFinancialData: users.NewGuardianFinancialDataRepository(peopleRuntime(db)),

		// Staff documents (#1424) — StaffDocument is bound by
		// bindStaffMembershipDecorators, it needs the membership owner.

		// School file storage trail (#2596)

		// Facilities repositories
		Room: facilitiesRepositoryAdapter.New(),

		// Education repositories
		Group:                 groupRepo,
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
		StaffWorkSchedule: workforceRepositoryAdapter.NewStaffWorkScheduleRepository(timetableDependencies.Workforce),
		WorkTimeModel:     workforceRepositoryAdapter.NewWorkTimeModelRepository(timetableDependencies.Workforce),

		// Device Fleet serves the unregistered tag scans (#2676).
		UnregisteredTagScan: NewUnregisteredTagScanRepository(deviceFleet),

		// Platform repositories. Operators and their refresh sessions are
		// served by the public Identity & Access capability the service root
		// binds (#3252).
		School: mustNewOrganizationTenancy(db),

		// Enrollment repositories
		SubmissionRateLimit: enrollmentModule,

		// Parent (cross-tenant guardian portal — PR 9+)
		ParentChild:             parentRepo.NewChildRepository(identity.ListActiveAccountSchoolIDs),
		ParentEnrollablePhase:   parentRepo.NewEnrollablePhaseRepository(enrollmentModule, identity.ListActiveAccountSchoolIDs),
		ParentEnrollmentRequest: parentRepo.NewEnrollmentRequestRepository(parentRuntime, enrollmentModule, identityAccountDirectory{accounts: identity}),

		// Parent Stammdaten direct-edit audit + change-request review
		StudentDataChangeRequest: nil, // bound to Care Plan below

		// Parent-OGS messaging (tenant-scoped two-way conversation per child)
		ParentMessageThread: parentStore.NewParentMessageThreadRepository(db, users.NewMessageableGuardianRepository(peopleRuntime(db), identity.FindActiveSchoolMemberships)),
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
	// follows the factory's current Care Plan binding.
	factory.CareWithdrawal = newCareWithdrawalCompletionRepository(
		func() careplan.Capability { return factory.carePlan },
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

	// Group substitutions belong to Workforce (#2688): the retained contract
	// is served by the adapter, which resolves groups through School
	// Structure and staff through School Membership.
	factory.GroupSubstitution = newGroupSubstitutions(
		workforceLegacy.NewGroupSubstitutionRepository(timetableDependencies.Workforce, func(ctx context.Context, ids []int64) (map[int64]string, error) {
			return NewGroupNames(factory.Group).GroupNamesByID(ctx, ids)
		}),
		func(ctx context.Context, ids []int64) (map[int64]*education.Group, error) {
			return factory.Group.FindByIDs(ctx, ids)
		},
		lazyStaffLookup{get: func() schoolmembership.Capability { return factory.schoolMembership }},
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
	factory.membershipDeps.groupTeachers = func() GroupTeacherRepository { return factory.GroupTeacher }
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

func (r *Factory) Enrollment() EnrollmentBookingProjection {
	return NewEnrollmentBookingProjection(r.SubmissionRateLimit)
}
