package api

import (
	"context"
	"fmt"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	enrollmentModule "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	usersModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/services"
)

// The compositions the retained People Directory repository suites
// (database/repositories/users) drive (#2727), composed here so the suites
// import neither the retained repositories nor the legacy composition. Every
// entry goes with the last suite that uses it.

// The composed graphs and the values the suites pass back into them.
type (
	PeopleRepositorySuiteParentMessaging       = ParentMessagingTestRepositories
	PeopleRepositorySuiteCareLifecycle         = CareLifecycleTestRepositories
	PeopleRepositorySuiteAuditCommand          = auditModels.Command
	PeopleRepositorySuiteGuardianProfileOption = peopleCompose.RetainedGuardianProfileOption
	PeopleRepositorySuiteRelationshipOption    = peopleCompose.RetainedGuardianRelationshipOption
	PeopleRepositorySuitePortalMemberships     = peopleCompose.RetainedPortalMembershipQuery
	PeopleRepositorySuiteSchoolMemberships     = peopleCompose.RetainedSchoolMembershipLookup
	PeopleRepositorySuiteStaffAccounts         = peopleCompose.RetainedStaffAccountsFunc
	PeopleRepositorySuiteStaffMessageIdentity  = peopleCompose.RetainedStaffMessageIdentity
	PeopleRepositorySuiteSchoolRoleClass       = peopleCompose.RetainedSchoolRoleClass
	PeopleRepositorySuitePickupCommand         = peopleCompose.RetainedGuardianPickupCommand
	PeopleRepositorySuiteAccessCommand         = peopleCompose.RetainedGuardianAccessCommand
	PeopleRepositorySuiteRecipients            = peopleCompose.RetainedMessageableGuardians
	PeopleRepositorySuiteColleagues            = peopleCompose.RetainedMessageableStaff
	PeopleRepositorySuitePeople                = peopledirectory.Capability
)

// PeopleRepositorySuiteFactory is the test graph over the unobserved
// timetable dependencies: the People Directory and School Membership
// repositories, and the retained repositories the retired repository factory
// held, each built over the same owners (#2743).
type PeopleRepositorySuiteFactory struct {
	services.MembershipTestRepositories
	CareScheduleRepositories
	timetableCompose.SchoolCalendarRepositories
	services.ParentPortalRepositories
	GuardianPhoneNumber   usersModels.GuardianPhoneNumberRepository
	GuardianFinancialData usersModels.GuardianFinancialDataRepository
	FamilyProtection      usersModels.FamilyProtectionEventRepository
	ParentRequestShare    usersModels.ParentRequestShareEventRepository
	ParentRequestEvent    usersModels.ParentRequestEventRepository
	GroupSubstitution     services.GroupSubstitutionRepository
	CaregiverBindingLock  usersModels.CaregiverBindingLocker
	// RFIDCard and Profile are the Identity & Access cards and account
	// profiles the factory used to hand out.
	RFIDCard identityaccess.RFIDCards
	Profile  identityaccess.AccountProfiles

	CareWithdrawal           usersModels.CareWithdrawalCompletionRepository
	StudentDataChangeRequest usersModels.StudentDataChangeRequestRepository
	StudentStatusDay         *services.StudentStatusDayRepository
	ExcusedAbsenceRequest    *services.ExcusedAbsenceRequestRepository
	ActiveGroup              studentpresence.SessionRecords
	GroupSupervisor          studentpresence.SupervisionRecords
	CrossTenant              services.CrossTenantQuery
	StaffAbsence             workforceCompose.StaffAbsenceRepository
	SessionStartLock         interface {
		LockSessionStart(context.Context, int64) error
	}

	carePlan       careplan.Capability
	enrollment     *enrollmentModule.Module
	schoolCalendar schoolcalendar.Calendar
}

// SchoolMembership is the School Membership owner the staff adapters read
// through.
func (f *PeopleRepositorySuiteFactory) SchoolMembership() schoolmembership.Capability {
	return f.Membership
}

// CarePlan is the Care Plan owner the schedule, request and withdrawal
// repositories delegate to.
func (f *PeopleRepositorySuiteFactory) CarePlan() careplan.Capability { return f.carePlan }

// Enrollment is the Enrollment booking projection of the graph.
func (f *PeopleRepositorySuiteFactory) Enrollment() services.EnrollmentBookingProjection {
	return services.NewEnrollmentBookingProjection(f.enrollment)
}

// SchoolCalendar is the calendar owner the period, closing day and dateframe
// repositories delegate to.
func (f *PeopleRepositorySuiteFactory) SchoolCalendar() schoolcalendar.Calendar {
	return f.schoolCalendar
}

// WithSchoolProjections returns a copy of the graph whose parent portal and
// messaging repositories resolve school names and eligibility through
// schools, as the service graph's own do.
func (f *PeopleRepositorySuiteFactory) WithSchoolProjections(schools organizationtenancy.Query) *PeopleRepositorySuiteFactory {
	bound := *f
	bound.ParentPortalRepositories = f.WithSchools(schools)
	return &bound
}

// NewPeopleRepositorySuiteFactory composes the suite graph over the unobserved
// timetable dependencies.
func NewPeopleRepositorySuiteFactory(db transactionruntime.Database, clocks ...func() time.Time) *PeopleRepositorySuiteFactory {
	return newPeopleRepositorySuiteFactory(db, NewUnobservedTimetableDependencies(db), clocks...)
}

func newPeopleRepositorySuiteFactory(db transactionruntime.Database, deps services.TimetableDependencies, clocks ...func() time.Time) *PeopleRepositorySuiteFactory {
	if deps.Capability == nil || deps.Students == nil || deps.Groups == nil || deps.Rooms == nil || deps.Calendar == nil || deps.Membership == nil || deps.Workforce == nil {
		panic("repository test factory: timetable and projection dependencies are required")
	}
	now := services.OptionalClock(clocks)
	identity := services.NewPeopleIdentityAccess(db.Pool(), nil)
	members := services.NewMembershipRepositories(db.Pool(), services.NewPersonRepository(db.Pool()), identity, deps.Membership, deps.Rooms, deps.Workforce)
	members.People = deps.Students
	members.Student = services.NewStudentRepository(db.Pool(), deps.Membership, deps.Groups)
	members.GuardianProfile = services.NewGuardianProfileRepository(db.Pool())
	members.StudentGuardian = services.NewStudentGuardianRepository(db.Pool())
	// The retained repositories the repository factory used to hold read
	// through unobserved Care Plan, School Calendar, Student Presence, Device
	// Fleet and Enrollment owners of db, bound at construction (#2743). The
	// school projections are bound by WithSchoolProjections.
	carePlan, err := NewUnobservedCarePlan(db, deps)
	if err != nil {
		panic(fmt.Sprintf("repository test factory: compose care plan: %v", err))
	}
	calendar, err := NewSchoolCalendar(db)
	if err != nil {
		panic(fmt.Sprintf("repository test factory: compose school calendar: %v", err))
	}
	devices, err := services.NewDeviceFleet(db.Pool())
	if err != nil {
		panic(fmt.Sprintf("repository test factory: compose device fleet: %v", err))
	}
	presence := NewStudentPresenceForTests(db)
	enrollment := enrollmentCompose.New()
	sessions := services.NewSessionRecords(db.Pool(), services.SessionRecordOwners{
		Now: now, Devices: devices, Rooms: deps.Rooms, Timetable: deps.Capability, Membership: deps.Membership, Persons: deps.Students,
	})
	return &PeopleRepositorySuiteFactory{
		MembershipTestRepositories: members,
		CareScheduleRepositories:   NewCareScheduleRepositories(carePlan),
		SchoolCalendarRepositories: timetableCompose.NewSchoolCalendarRepositories(calendar, timetableCompose.NewPlanningCalendarPeriodUsage(enrollmentCompose.New(), deps.Capability)),
		ParentPortalRepositories: services.NewParentPortalRepositories(db.Pool(), services.ParentPortalOwners{
			People: deps.Students, Groups: deps.Groups, Membership: deps.Membership,
			Identity: services.NewPeopleIdentityAccess(db.Pool(), services.IdentityAccessObserver(deps.ObserveIdentityAccess)), Enrollment: enrollment, Now: now,
		}),
		GuardianPhoneNumber:      peopleCompose.NewGuardianPhoneNumberRepository(db.Pool()),
		GuardianFinancialData:    peopleCompose.NewGuardianFinancialDataRepository(db.Pool()),
		FamilyProtection:         peopleCompose.NewFamilyProtectionEventRepository(db.Pool()),
		ParentRequestShare:       peopleCompose.NewParentRequestShareEventRepository(db.Pool()),
		ParentRequestEvent:       peopleCompose.NewParentRequestEventRepository(db.Pool()),
		GroupSubstitution:        services.NewGroupSubstitutionRepository(members.Group, deps.Membership, deps.Workforce),
		CaregiverBindingLock:     services.NewCaregiverBindingLocker(deps.Membership, deps.Workforce, NewStudentPresenceForTests(db), deps.Capability),
		RFIDCard:                 identity,
		Profile:                  identity,
		CareWithdrawal:           services.NewCareWithdrawalCompletionRepository(carePlan),
		StudentDataChangeRequest: services.NewStudentDataChangeRequestRepository(carePlan),
		StudentStatusDay:         services.NewStudentStatusDayRepository(carePlan),
		ExcusedAbsenceRequest:    services.NewExcusedAbsenceRequestRepository(carePlan),
		ActiveGroup:              sessions,
		GroupSupervisor:          services.NewGroupSupervisionRecords(sessions, deps.Groups),
		CrossTenant:              services.NewCrossTenantQuery(presence, deps.Students, deps.Groups),
		StaffAbsence:             services.NewStaffAbsenceRepository(deps.Workforce, deps.Membership, deps.Students),
		SessionStartLock:         presence,
		carePlan:                 carePlan,
		enrollment:               enrollment,
		schoolCalendar:           calendar,
	}
}

// NewPeopleRepositorySuitePeople is the People Directory owner.
func NewPeopleRepositorySuitePeople(db transactionruntime.Database) peopledirectory.Capability {
	return services.MustNewPeopleDirectory(db.Pool())
}

// NewPeopleRepositorySuiteCareLifecycle composes the care lifecycle; a nil
// command records into the test audit store.
func NewPeopleRepositorySuiteCareLifecycle(db transactionruntime.Database, command auditModels.Command) (CareLifecycleTestRepositories, error) {
	return NewCareLifecycleTestRepositories(db, command)
}

// NewPeopleRepositorySuiteParentMessaging composes the parent messaging stores.
func NewPeopleRepositorySuiteParentMessaging(db transactionruntime.Database) (ParentMessagingTestRepositories, error) {
	return NewParentMessagingTestRepositories(db)
}

// NewPeopleRepositorySuiteAnnouncements is the parent announcement store over
// the given Enrollment audience queries.
func NewPeopleRepositorySuiteAnnouncements(db transactionruntime.Database, audience services.AnnouncementEnrollmentQueries, clocks ...func() time.Time) usersModels.ParentAnnouncementRepository {
	return parentStore.NewParentAnnouncementRepository(db.Pool(), services.ParentAnnouncementApplicants(audience), clocks...)
}

// NewPeopleRepositorySuiteCaregiverBindingLocker is the caregiver binding
// lock over the four owners that hold caregiver capability.
func NewPeopleRepositorySuiteCaregiverBindingLocker(db transactionruntime.Database) usersModels.CaregiverBindingLocker {
	deps := NewUnobservedTimetableDependencies(db)
	return services.NewCaregiverBindingLocker(deps.Membership, deps.Workforce, NewStudentPresenceForTests(db), deps.Capability)
}

// NewPeopleRepositorySuiteCompanions is the companion store over Care Plan.
func NewPeopleRepositorySuiteCompanions(capability careplan.Capability) usersModels.StudentCompanionRepository {
	return services.NewStudentCompanionRepository(capability)
}

// NewPeopleRepositorySuiteCompanionEdge validates one companion edge.
func NewPeopleRepositorySuiteCompanionEdge(studentID, companionID int64, weekday int) (*usersModels.StudentCompanion, error) {
	return NewStudentCompanionEdge(studentID, companionID, weekday)
}

// ReplacePeopleRepositorySuiteCompanions replaces a child's companion edges.
func ReplacePeopleRepositorySuiteCompanions(ctx context.Context, links usersModels.StudentCompanionRepository, studentID int64, edges []*usersModels.StudentCompanion) error {
	return ReplaceStudentCompanions(ctx, links, studentID, edges)
}

// NewPeopleRepositorySuiteRelationships is the relationship store with its
// owners bound, as the composition root binds it.
func NewPeopleRepositorySuiteRelationships(db transactionruntime.Database) usersModels.StudentGuardianRepository {
	return services.NewStudentGuardianRepository(db.Pool())
}

// NewPeopleRepositorySuiteGuardianProfiles is the guardian profile store as
// the composition root binds it.
func NewPeopleRepositorySuiteGuardianProfiles(db transactionruntime.Database) usersModels.GuardianProfileRepository {
	return services.NewGuardianProfileRepository(db.Pool())
}

// NewEnrollmentGuardianAutofillSuite is the enrollment form's autofill
// reader as the composition root binds it, over the retained guardian profile
// store of db (#2734).
func NewEnrollmentGuardianAutofillSuite(db transactionruntime.Database) services.EnrollmentGuardianAutofill {
	return services.EnrollmentGuardianAutofill{Loader: peopleCompose.NewGuardianProfileLoader(services.NewGuardianProfileRepository(db.Pool()), db.Pool(), nil)}
}

// NewPeopleRepositorySuiteRecipients is the parent-message recipient lookup
// as the composition root binds it.
func NewPeopleRepositorySuiteRecipients(db transactionruntime.Database) *peopleCompose.RetainedMessageableGuardians {
	return NewMessageableGuardianRepository(db)
}

// NewPeopleRepositorySuiteRetainedPersons is the person store without the
// account lookup.
func NewPeopleRepositorySuiteRetainedPersons(db transactionruntime.Database) usersModels.PersonRepository {
	return peopleCompose.NewRetainedPersonRepository(db.Pool())
}

// NewPeopleRepositorySuiteRetainedGuardianProfiles is the guardian profile
// store with only the given options bound.
func NewPeopleRepositorySuiteRetainedGuardianProfiles(db transactionruntime.Database, options ...peopleCompose.RetainedGuardianProfileOption) usersModels.GuardianProfileRepository {
	return peopleCompose.NewRetainedGuardianProfileRepository(db.Pool(), options...)
}

// NewPeopleRepositorySuiteRetainedRelationships is the relationship store
// with only the given options bound.
func NewPeopleRepositorySuiteRetainedRelationships(db transactionruntime.Database, options ...peopleCompose.RetainedGuardianRelationshipOption) usersModels.StudentGuardianRepository {
	return peopleCompose.NewRetainedGuardianRelationshipRepository(db.Pool(), options...)
}

// NewPeopleRepositorySuiteRetainedRecipients is the recipient lookup over the
// given membership lookup.
func NewPeopleRepositorySuiteRetainedRecipients(db transactionruntime.Database, memberships peopleCompose.RetainedSchoolMembershipLookup) *peopleCompose.RetainedMessageableGuardians {
	return peopleCompose.NewRetainedMessageableGuardianRepository(db.Pool(), memberships)
}

// NewPeopleRepositorySuiteRetainedColleagues is the colleague lookup over the
// given owner queries.
func NewPeopleRepositorySuiteRetainedColleagues(db transactionruntime.Database, staffAccounts peopleCompose.RetainedStaffAccountsFunc, identity peopleCompose.RetainedStaffMessageIdentity) *peopleCompose.RetainedMessageableStaff {
	return peopleCompose.NewRetainedMessageableStaffRepository(db.Pool(), staffAccounts, identity)
}

// PeopleRepositorySuiteWithPortalMemberships binds the reachability query.
func PeopleRepositorySuiteWithPortalMemberships(query peopleCompose.RetainedPortalMembershipQuery) peopleCompose.RetainedGuardianProfileOption {
	return peopleCompose.RetainedWithPortalMemberships(query)
}

// PeopleRepositorySuiteWithMemberships binds the relationship membership lookup.
func PeopleRepositorySuiteWithMemberships(query peopleCompose.RetainedSchoolMembershipLookup) peopleCompose.RetainedGuardianRelationshipOption {
	return peopleCompose.RetainedWithGuardianRelationshipMemberships(query)
}

// PeopleRepositorySuiteWithOwners binds the relationship's owner halves.
func PeopleRepositorySuiteWithOwners(pickup peopleCompose.RetainedGuardianPickupCommand, access peopleCompose.RetainedGuardianAccessCommand) peopleCompose.RetainedGuardianRelationshipOption {
	return peopleCompose.RetainedWithGuardianRelationshipOwners(pickup, access)
}

// PeopleRepositorySuiteWithDefaultRole binds the default role preset.
func PeopleRepositorySuiteWithDefaultRole() peopleCompose.RetainedGuardianRelationshipOption {
	return peopleCompose.RetainedWithGuardianDefaultRole(services.ApplyDefaultGuardianRole)
}

// VerifyPeopleRepositorySuiteStudentSchema runs the students table check.
func VerifyPeopleRepositorySuiteStudentSchema(ctx context.Context, db transactionruntime.Database) error {
	return peopleCompose.VerifyRetainedStudentSchema(ctx, db.Pool())
}

// RepositoryTestFactory is the repository factory test graphs compose, next to
// the People Directory and School Membership repositories that left it
// (#2743).
type RepositoryTestFactory = PeopleRepositorySuiteFactory

// NewRepositoryTestFactory composes the test graph over deps.
func NewRepositoryTestFactory(db transactionruntime.Database, deps services.TimetableDependencies, clocks ...func() time.Time) *RepositoryTestFactory {
	return newPeopleRepositorySuiteFactory(db, deps, clocks...)
}

// NewRepositoryTestFactoryWithPeopleDirectory is NewRepositoryTestFactory for
// suites that handle a composition error.
func NewRepositoryTestFactoryWithPeopleDirectory(db transactionruntime.Database, deps services.TimetableDependencies, clocks ...func() time.Time) (*RepositoryTestFactory, error) {
	return newPeopleRepositorySuiteFactory(db, deps, clocks...), nil
}

// NewUnboundStudentRepository is the retained student repository without the
// School Membership teacher assignments and the School Structure group names:
// the teacher-scoped roster reads fail closed. Repository suites that only
// read and write the child's row use it.
func NewUnboundStudentRepository(db transactionruntime.Database) usersModels.StudentRepository {
	enrollment, err := services.NewSchoolMembership(db.Pool())
	if err != nil {
		panic(fmt.Sprintf("student repository: compose membership: %v", err))
	}
	return peopleCompose.NewStudentRepository(peopleCompose.StudentRepositoryDependencies{
		DB: db.Pool(), Directory: services.MustNewPeopleDirectory(db.Pool()), Enrollment: enrollment,
	})
}
