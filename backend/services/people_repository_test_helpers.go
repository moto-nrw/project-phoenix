package services

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	enrollmentModule "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce/adapters/timerecords"
	"github.com/moto-nrw/project-phoenix/services/users"
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
	PeopleRepositorySuiteAnnouncementAudience  = AnnouncementEnrollmentQueries
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
	PeopleRepositorySuiteSchoolStructure       = schoolstructure.Capability
	PeopleRepositorySuiteCarePlan              = careplan.Capability
	PeopleRepositorySuiteCompanionLinks        = usersModels.StudentCompanionRepository
)

// PeopleRepositorySuiteFactory is the test graph over the unobserved
// timetable dependencies: the People Directory and School Membership
// repositories, and the retained repositories the retired repository factory
// held, each built over the same owners (#2743).
type PeopleRepositorySuiteFactory struct {
	MembershipTestRepositories
	CareScheduleRepositories
	timetableCompose.SchoolCalendarRepositories
	ParentPortalRepositories
	GuardianPhoneNumber   usersModels.GuardianPhoneNumberRepository
	GuardianFinancialData usersModels.GuardianFinancialDataRepository
	FamilyProtection      usersModels.FamilyProtectionEventRepository
	ParentRequestShare    usersModels.ParentRequestShareEventRepository
	ParentRequestEvent    usersModels.ParentRequestEventRepository
	GroupSubstitution     GroupSubstitutionRepository
	CaregiverBindingLock  usersModels.CaregiverBindingLocker
	// RFIDCard and Profile are the Identity & Access cards and account
	// profiles the factory used to hand out.
	RFIDCard identityaccess.RFIDCards
	Profile  identityaccess.AccountProfiles

	CareWithdrawal           usersModels.CareWithdrawalCompletionRepository
	StudentDataChangeRequest usersModels.StudentDataChangeRequestRepository
	StudentStatusDay         *StudentStatusDayRepository
	ExcusedAbsenceRequest    *ExcusedAbsenceRequestRepository
	ActiveGroup              studentpresence.SessionRecords
	GroupSupervisor          studentpresence.SupervisionRecords
	CrossTenant              CrossTenantQuery
	StaffAbsence             timerecords.StaffAbsenceRepository
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
func (f *PeopleRepositorySuiteFactory) Enrollment() EnrollmentBookingProjection {
	return NewEnrollmentBookingProjection(f.enrollment)
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
func NewPeopleRepositorySuiteFactory(db *bun.DB, clocks ...func() time.Time) *PeopleRepositorySuiteFactory {
	return newPeopleRepositorySuiteFactory(db, NewUnobservedTimetableDependencies(db), clocks...)
}

func newPeopleRepositorySuiteFactory(db *bun.DB, deps TimetableDependencies, clocks ...func() time.Time) *PeopleRepositorySuiteFactory {
	if deps.Capability == nil || deps.Students == nil || deps.Groups == nil || deps.Rooms == nil || deps.Calendar == nil || deps.Membership == nil || deps.Workforce == nil {
		panic("repository test factory: timetable and projection dependencies are required")
	}
	now := optionalClock(clocks)
	identity := newPeopleIdentityAccess(db, nil)
	members := newMembershipRepositories(db, NewPersonRepository(db), identity, deps.Membership, deps.Rooms, deps.Workforce)
	members.People = deps.Students
	members.Student = NewStudentRepository(db, deps.Membership, deps.Groups)
	members.GuardianProfile = NewGuardianProfileRepository(db)
	members.StudentGuardian = NewStudentGuardianRepository(db)
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
	devices, err := NewDeviceFleet(db)
	if err != nil {
		panic(fmt.Sprintf("repository test factory: compose device fleet: %v", err))
	}
	presence := NewStudentPresenceForTests(db)
	enrollment := enrollmentCompose.New()
	sessions := NewSessionRecords(db, SessionRecordOwners{
		Now: now, Devices: devices, Rooms: deps.Rooms, Timetable: deps.Capability, Membership: deps.Membership, Persons: deps.Students,
	})
	return &PeopleRepositorySuiteFactory{
		MembershipTestRepositories: members,
		CareScheduleRepositories:   NewCareScheduleRepositories(carePlan),
		SchoolCalendarRepositories: timetableCompose.NewSchoolCalendarRepositories(calendar, timetableCompose.NewPlanningCalendarPeriodUsage(enrollmentCompose.New(), deps.Capability)),
		ParentPortalRepositories: NewParentPortalRepositories(db, ParentPortalOwners{
			People: deps.Students, Groups: deps.Groups, Membership: deps.Membership,
			Identity: newPeopleIdentityAccess(db, IdentityAccessObserver(deps.ObserveIdentityAccess)), Enrollment: enrollment, Now: now,
		}),
		GuardianPhoneNumber:      peopleCompose.NewGuardianPhoneNumberRepository(db),
		GuardianFinancialData:    peopleCompose.NewGuardianFinancialDataRepository(db),
		FamilyProtection:         peopleCompose.NewFamilyProtectionEventRepository(db),
		ParentRequestShare:       peopleCompose.NewParentRequestShareEventRepository(db),
		ParentRequestEvent:       peopleCompose.NewParentRequestEventRepository(db),
		GroupSubstitution:        newGroupSubstitutionRepository(members.Group, deps.Membership, deps.Workforce),
		CaregiverBindingLock:     NewCaregiverBindingLocker(deps.Membership, deps.Workforce, NewStudentPresenceForTests(db), deps.Capability),
		RFIDCard:                 identity,
		Profile:                  identity,
		CareWithdrawal:           NewCareWithdrawalCompletionRepository(carePlan),
		StudentDataChangeRequest: NewStudentDataChangeRequestRepository(carePlan),
		StudentStatusDay:         NewStudentStatusDayRepository(carePlan),
		ExcusedAbsenceRequest:    NewExcusedAbsenceRequestRepository(carePlan),
		ActiveGroup:              sessions,
		GroupSupervisor:          NewGroupSupervisionRecords(sessions, deps.Groups),
		CrossTenant:              NewCrossTenantQuery(presence, deps.Students, deps.Groups),
		StaffAbsence:             NewStaffAbsenceRepository(deps.Workforce, deps.Membership, deps.Students),
		SessionStartLock:         presence,
		carePlan:                 carePlan,
		enrollment:               enrollment,
		schoolCalendar:           calendar,
	}
}

// NewPeopleRepositorySuitePeople is the People Directory owner.
func NewPeopleRepositorySuitePeople(db *bun.DB) peopledirectory.Capability {
	return MustNewPeopleDirectory(db)
}

// NewPeopleRepositorySuiteSchoolStructure is the School Structure owner.
func NewPeopleRepositorySuiteSchoolStructure(db *bun.DB) (schoolstructure.Capability, error) {
	return NewSchoolStructure(db)
}

// NewPeopleRepositorySuiteCareLifecycle composes the care lifecycle; a nil
// command records into the test audit store.
func NewPeopleRepositorySuiteCareLifecycle(db *bun.DB, command auditModels.Command) (CareLifecycleTestRepositories, error) {
	return NewCareLifecycleTestRepositories(db, command)
}

// NewPeopleRepositorySuiteParentMessaging composes the parent messaging stores.
func NewPeopleRepositorySuiteParentMessaging(db *bun.DB) (ParentMessagingTestRepositories, error) {
	return NewParentMessagingTestRepositories(db)
}

// NewPeopleRepositorySuiteAnnouncements is the parent announcement store over
// the given Enrollment audience queries.
func NewPeopleRepositorySuiteAnnouncements(db *bun.DB, audience AnnouncementEnrollmentQueries, clocks ...func() time.Time) usersModels.ParentAnnouncementRepository {
	return NewParentAnnouncementRepository(db, audience, clocks...)
}

// NewPeopleRepositorySuiteCaregiverBindingLocker is the caregiver binding
// lock over the four owners that hold caregiver capability.
func NewPeopleRepositorySuiteCaregiverBindingLocker(db *bun.DB) usersModels.CaregiverBindingLocker {
	deps := NewUnobservedTimetableDependencies(db)
	return NewCaregiverBindingLocker(deps.Membership, deps.Workforce, NewStudentPresenceForTests(db), deps.Capability)
}

// NewPeopleRepositorySuiteCompanions is the companion store over Care Plan.
func NewPeopleRepositorySuiteCompanions(capability careplan.Capability) usersModels.StudentCompanionRepository {
	return NewStudentCompanionRepository(capability)
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
func NewPeopleRepositorySuiteRelationships(db *bun.DB) usersModels.StudentGuardianRepository {
	return NewStudentGuardianRepository(db)
}

// NewPeopleRepositorySuiteGuardianProfiles is the guardian profile store as
// the composition root binds it.
func NewPeopleRepositorySuiteGuardianProfiles(db *bun.DB) usersModels.GuardianProfileRepository {
	return NewGuardianProfileRepository(db)
}

// NewEnrollmentGuardianAutofillSuite is the enrollment form's autofill
// reader as the composition root binds it, over the retained guardian profile
// store of db (#2734).
func NewEnrollmentGuardianAutofillSuite(db *bun.DB) EnrollmentGuardianAutofill {
	return EnrollmentGuardianAutofill{loader: users.NewGuardianProfileLoader(NewGuardianProfileRepository(db), db, nil)}
}

// NewPeopleRepositorySuiteRecipients is the parent-message recipient lookup
// as the composition root binds it.
func NewPeopleRepositorySuiteRecipients(db *bun.DB) *peopleCompose.RetainedMessageableGuardians {
	return NewMessageableGuardianRepository(db)
}

// NewPeopleRepositorySuiteRetainedPersons is the person store without the
// account lookup.
func NewPeopleRepositorySuiteRetainedPersons(db *bun.DB) usersModels.PersonRepository {
	return peopleCompose.NewRetainedPersonRepository(db)
}

// NewPeopleRepositorySuiteRetainedGuardianProfiles is the guardian profile
// store with only the given options bound.
func NewPeopleRepositorySuiteRetainedGuardianProfiles(db *bun.DB, options ...peopleCompose.RetainedGuardianProfileOption) usersModels.GuardianProfileRepository {
	return peopleCompose.NewRetainedGuardianProfileRepository(db, options...)
}

// NewPeopleRepositorySuiteRetainedRelationships is the relationship store
// with only the given options bound.
func NewPeopleRepositorySuiteRetainedRelationships(db *bun.DB, options ...peopleCompose.RetainedGuardianRelationshipOption) usersModels.StudentGuardianRepository {
	return peopleCompose.NewRetainedGuardianRelationshipRepository(db, options...)
}

// NewPeopleRepositorySuiteRetainedRecipients is the recipient lookup over the
// given membership lookup.
func NewPeopleRepositorySuiteRetainedRecipients(db *bun.DB, memberships peopleCompose.RetainedSchoolMembershipLookup) *peopleCompose.RetainedMessageableGuardians {
	return peopleCompose.NewRetainedMessageableGuardianRepository(db, memberships)
}

// NewPeopleRepositorySuiteRetainedColleagues is the colleague lookup over the
// given owner queries.
func NewPeopleRepositorySuiteRetainedColleagues(db *bun.DB, staffAccounts peopleCompose.RetainedStaffAccountsFunc, identity peopleCompose.RetainedStaffMessageIdentity) *peopleCompose.RetainedMessageableStaff {
	return peopleCompose.NewRetainedMessageableStaffRepository(db, staffAccounts, identity)
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
	return peopleCompose.RetainedWithGuardianDefaultRole(applyDefaultGuardianRole)
}

// VerifyPeopleRepositorySuiteStudentSchema runs the students table check.
func VerifyPeopleRepositorySuiteStudentSchema(ctx context.Context, db *bun.DB) error {
	return peopleCompose.VerifyRetainedStudentSchema(ctx, db)
}

// RepositoryTestFactory is the repository factory test graphs compose, next to
// the People Directory and School Membership repositories that left it
// (#2743).
type RepositoryTestFactory = PeopleRepositorySuiteFactory

// NewRepositoryTestFactory composes the test graph over deps.
func NewRepositoryTestFactory(db *bun.DB, deps TimetableDependencies, clocks ...func() time.Time) *RepositoryTestFactory {
	return newPeopleRepositorySuiteFactory(db, deps, clocks...)
}

// NewRepositoryTestFactoryWithPeopleDirectory is NewRepositoryTestFactory for
// suites that handle a composition error.
func NewRepositoryTestFactoryWithPeopleDirectory(db *bun.DB, deps TimetableDependencies, clocks ...func() time.Time) (*RepositoryTestFactory, error) {
	return newPeopleRepositorySuiteFactory(db, deps, clocks...), nil
}
