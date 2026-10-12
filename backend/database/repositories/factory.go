package repositories

import (
	"context"
	"fmt"
	"time"

	parentRepo "github.com/moto-nrw/project-phoenix/database/repositories/parent"
	"github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	enrollmentCapability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce/adapters/timerecords"
	workforceLegacy "github.com/moto-nrw/project-phoenix/modules/workforce/legacy"

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
	carePlanBound            bool
	facilitiesBound          bool
	// roomBinders hand a room owner to the raw repositories that used to
	// join facilities.rooms; kept so BindFacilities reaches them after the
	// projections wrapped the factory fields (#2665).
	roomBinders         []func(facilitiesModule.Query)
	schoolCalendarBound bool
	// schoolCalendar is the bound capability behind the calendar period,
	// closing day and dateframe adapters (#2666).
	schoolCalendar schoolcalendar.Calendar
	carePlan       careplan.Capability

	CareWithdrawal userModels.CareWithdrawalCompletionRepository

	// Schedule domain
	Dateframe                 scheduleModels.DateframeRepository
	StudentPickupSchedule     scheduleModels.StudentPickupScheduleRepository
	StudentPickupException    scheduleModels.StudentPickupExceptionRepository
	StudentPickupNote         scheduleModels.StudentPickupNoteRepository
	StudentArrivalSchedule    scheduleModels.StudentArrivalScheduleRepository
	StudentArrivalException   scheduleModels.StudentArrivalExceptionRepository
	StudentArrivalNote        scheduleModels.StudentArrivalNoteRepository
	CareScheduleChangeRequest scheduleModels.CareScheduleChangeRequestRepository
	CalendarPeriod            scheduleModels.CalendarPeriodRepository
	ClosingDay                scheduleModels.ClosingDayRepository

	// Active domain
	ActiveGroup           studentpresence.SessionRecords
	GroupSupervisor       studentpresence.SupervisionRecords
	CrossTenant           CrossTenantQuery
	StudentStatusDay      *StudentStatusDayRepository
	ExcusedAbsenceRequest *ExcusedAbsenceRequestRepository
	StaffAbsence          timerecords.StaffAbsenceRepository

	SessionStartLock interface {
		LockSessionStart(context.Context, int64) error
	}

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
	ParentMessageRead   userModels.ParentMessageReadRepository

	// OGS-internal colleague chat (#2598)
	StaffMessageRead userModels.StaffMessageReadRepository

	// Parent announcements (tenant-authored broadcast news to guardians)
	ParentAnnouncement userModels.ParentAnnouncementRepository
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

// NewFactory creates a new repository factory with all repositories. The
// owner capabilities in timetableDependencies are the ones the retained
// adapters read through; the serving root passes its observed modules.
func NewFactory(db *bun.DB, timetableDependencies TimetableDependencies, clocks ...func() time.Time) *Factory {
	if timetableDependencies.Capability == nil || timetableDependencies.Students == nil || timetableDependencies.Groups == nil || timetableDependencies.Rooms == nil || timetableDependencies.Calendar == nil || timetableDependencies.Membership == nil || timetableDependencies.Workforce == nil {
		panic("repository factory: timetable and projection dependencies are required")
	}
	timetableCapability := timetableDependencies.Capability
	persons := timetableDependencies.Students
	membership := timetableDependencies.Membership
	presenceCapability := newStudentPresence(db)
	var now func() time.Time
	if len(clocks) > 0 && clocks[0] != nil {
		now = clocks[0]
	}
	deviceFleet := mustNewDeviceFleet(db)
	sessionRecords := newPresenceSessionRecords(presenceCompose.SessionRecordDependencies{
		DB: db, Now: now, Devices: activeDeviceDirectory{devices: deviceFleet}, Rooms: &activeRoomDirectory{},
		Activities: NewSessionActivities(timetableCompose.NewActivityGroupRecords(timetableCapability)),
		Staff:      &presenceSupervisionStaff{},
	})
	enrollmentModule := enrollmentCompose.New()
	parentAnnouncement := NewParentAnnouncementRepository(db, enrollmentModule, now)
	parentRuntime := parentRepo.RuntimeFunc(carePlanCompose.TenantAmbientDatabase(db))
	// The account facts other owners read belong to Identity & Access
	// (#2720): the account lookups the Care Plan and parent repositories need
	// are bound at construction.
	identity := newIdentityAccess(db, timetableDependencies.ObserveIdentityAccess)
	factory := &Factory{
		db: db,

		// Schedule repositories. Dateframe, CalendarPeriod and ClosingDay
		// belong to School Calendar and are bound below; the Timetable
		// repositories are built where they are used (#2743).
		StudentPickupSchedule:     nil, // bound to Care Plan below
		StudentPickupException:    nil, // bound to Care Plan below
		StudentPickupNote:         nil, // bound to Care Plan below
		StudentArrivalSchedule:    nil, // bound to Care Plan below
		StudentArrivalException:   nil, // bound to Care Plan below
		StudentArrivalNote:        nil, // bound to Care Plan below
		CareScheduleChangeRequest: nil, // bound to Care Plan below

		// Active repositories
		ActiveGroup:           sessionRecords,
		GroupSupervisor:       sessionRecords,
		CrossTenant:           &visitorProjection{visits: presenceCapability},
		StudentStatusDay:      nil, // bound to Care Plan below
		ExcusedAbsenceRequest: nil, // bound to Care Plan below
		// Staff absences belong to Workforce (#2690); the person and staff
		// projections wrap the adapter below.
		StaffAbsence: workforceLegacy.NewStaffAbsenceRepository(timetableDependencies.Workforce),

		SessionStartLock: presenceCapability,

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
		// ParentMessageRead and StaffMessageRead are bound by
		// bindStaffMembershipDecorators, they need the membership owner.

		ParentAnnouncement: parentAnnouncement,
	}
	// Care withdrawal completions belong to Care Plan (#3221); the adapter
	// follows the factory's current Care Plan binding.
	factory.CareWithdrawal = newCareWithdrawalCompletionRepository(
		func() careplan.Capability { return factory.carePlan },
	)
	// The People Directory reads go to the given owner (#2662, #2663): bind
	// the student and guardian ports while their repositories are still raw.
	// The staff and person projections below wrap some of the same
	// repositories.
	factory.bindStudentDirectories(persons, persons)
	factory.bindGuardianDirectories(persons)
	factory.bindDefaultFacilities(db)
	// Care Plan reads the participant slots through the Timetable adapter
	// over the factory's Student Presence owner.
	carePlan, err := NewCarePlan(db, persons, timetableCompose.NewInstanceStudentRepository(db, timetableCapability, presenceCapability, NewPartialAbsencePreview(db)))
	if err != nil {
		panic(fmt.Sprintf("repository factory: compose care plan: %v", err))
	}
	factory.bindCarePlanAdapters(carePlan)
	// Calendar periods, closing days and dateframes belong to School
	// Calendar; the unobserved default keeps every legacy consumer on the
	// owner until the production root binds the observed module (#2666).
	calendar, err := NewSchoolCalendar(db)
	if err != nil {
		panic(fmt.Sprintf("repository factory: compose school calendar: %v", err))
	}
	factory.bindSchoolCalendarAdapters(calendar, NewCalendarPeriodUsage(enrollmentCompose.New(), timetableCapability))
	// The decorators are wired innermost, so the school, person and group
	// wrappers bound afterwards keep wrapping them.
	factory.bindStaffMembershipDecorators(membership, identity, newPersonRepository(db))
	// Same membership for the repositories that used to join users.staff or
	// users.teachers themselves; wired here so they sit inside the person,
	// school and group wrappers bound afterwards (#2667, agent A2).
	factory.bindStaffProjections(membership, timetableDependencies.Workforce)
	// The person projections sit outside the staff projections, so they still
	// find the Staff rows they attach Staff.Person to (#2661).
	factory.bindPersonProjections(persons)
	// The group-enriched adapters name their groups through the public School
	// Structure query, so no repository outside the owner reads
	// education.groups itself.
	groups := timetableDependencies.Groups
	factory.GroupSupervisor = groupSupervisorRepository{SupervisionRecords: factory.GroupSupervisor, groups: groups}
	factory.CrossTenant = groupCrossTenantRepository{CrossTenantQuery: factory.CrossTenant, groups: groups}
	factory.ParentMessageRead = groupParentMessageReadRepository{ParentMessageReadRepository: factory.ParentMessageRead, groups: groups}
	return factory
}

func (r *Factory) Enrollment() EnrollmentBookingProjection {
	return NewEnrollmentBookingProjection(r.SubmissionRateLimit)
}
