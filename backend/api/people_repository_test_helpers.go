//go:build testhooks

package api

import (
	"context"
	"fmt"
	"time"

	guardianLinkView "github.com/moto-nrw/project-phoenix/modules/guardianlinkview/compose"

	studentDirectoryView "github.com/moto-nrw/project-phoenix/modules/studentdirectoryview/compose"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	enrollmentModule "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
)

// The compositions the retained People Directory repository suites
// (database/repositories/users) drive (#2727), composed here so the suites
// import neither the retained repositories nor the service graph. Every
// entry goes with the last suite that uses it.

// The composed graphs and the values the suites pass back into them.
type (
	PeopleRepositorySuiteParentMessaging = ParentMessagingTestRepositories
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
	GuardianPhoneNumber   peoplePorts.GuardianPhoneNumberRepository
	GuardianFinancialData peoplePorts.GuardianFinancialDataRepository
	FamilyProtection      peoplePorts.FamilyProtectionRecords
	ParentRequestShare    peoplePorts.ParentRequestShareRecords
	ParentRequestEvent    peoplePorts.ParentRequestEventRecords
	GroupSubstitution     GroupSubstitutionRepository
	CaregiverBindingLock  peoplePorts.CaregiverBindingLocker
	// RFIDCard and Profile are the Identity & Access cards and account
	// profiles the repository factory used to hand out.
	RFIDCard identityaccess.RFIDCards
	Profile  identityaccess.AccountProfiles

	CareWithdrawal           CareWithdrawalRecords
	StudentDataChangeRequest studentDataRequestRepository
	StudentStatusDay         *StudentStatusDayRepository
	ExcusedAbsenceRequest    *ExcusedAbsenceRequestRepository
	ActiveGroup              studentpresence.SessionRecords
	GroupSupervisor          studentpresence.SupervisionRecords
	CrossTenant              CrossTenantQuery
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
func (f *PeopleRepositorySuiteFactory) Enrollment() EnrollmentBookingProjection {
	return NewEnrollmentBookingProjection(newEnrollmentBookings(f.enrollment))
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

func newPeopleRepositorySuiteFactory(db transactionruntime.Database, deps TimetableDependencies, clocks ...func() time.Time) *PeopleRepositorySuiteFactory {
	if deps.Capability == nil || deps.Students == nil || deps.Groups == nil || deps.Rooms == nil || deps.Calendar == nil || deps.Membership == nil || deps.Workforce == nil {
		panic("repository test factory: timetable and projection dependencies are required")
	}
	now := OptionalClock(clocks)
	identity := NewPeopleIdentityAccess(db, nil)
	members := NewMembershipRepositories(db, NewPersonRepository(db), identity, deps.Membership, deps.Rooms, deps.Workforce)
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
			Identity: NewPeopleIdentityAccess(db, IdentityAccessObserver(deps.ObserveIdentityAccess)), Enrollment: enrollment, Now: now,
		}),
		GuardianPhoneNumber:      peopleCompose.NewGuardianPhoneNumberRepository(db.Pool()),
		GuardianFinancialData:    peopleCompose.NewGuardianFinancialDataRepository(db.Pool()),
		FamilyProtection:         peopleCompose.NewFamilyProtectionEventRepository(db.Pool()),
		ParentRequestShare:       peopleCompose.NewParentRequestShareEventRepository(db.Pool()),
		ParentRequestEvent:       peopleCompose.NewParentRequestEventRepository(db.Pool()),
		GroupSubstitution:        NewGroupSubstitutionRepository(members.Group, deps.Membership, deps.Workforce),
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
func NewPeopleRepositorySuitePeople(db transactionruntime.Database) peopledirectory.Capability {
	return MustNewPeopleDirectory(db)
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
func NewPeopleRepositorySuiteAnnouncements(db transactionruntime.Database, audience AnnouncementEnrollmentQueries, clocks ...func() time.Time) parentStore.ParentAnnouncementRepository {
	return parentStore.NewParentAnnouncementRepository(db.Pool(), ParentAnnouncementApplicants(audience), guardianLinkView.NewReader(), clocks...)
}

// NewPeopleRepositorySuiteCaregiverBindingLocker is the caregiver binding
// lock over the four owners that hold caregiver capability.
func NewPeopleRepositorySuiteCaregiverBindingLocker(db transactionruntime.Database) peoplePorts.CaregiverBindingLocker {
	deps := NewUnobservedTimetableDependencies(db)
	return NewCaregiverBindingLocker(deps.Membership, deps.Workforce, NewStudentPresenceForTests(db), deps.Capability)
}

// NewPeopleRepositorySuiteCompanions is the companion store over Care Plan.
func NewPeopleRepositorySuiteCompanions(capability careplan.Capability) CompanionRepository {
	return NewStudentCompanionRepository(capability)
}

// NewPeopleRepositorySuiteCompanionEdge validates one companion edge.
func NewPeopleRepositorySuiteCompanionEdge(studentID, companionID int64, weekday int) (*careplan.CompanionEdge, error) {
	return NewStudentCompanionEdge(studentID, companionID, weekday)
}

// ReplacePeopleRepositorySuiteCompanions replaces a child's companion edges.
func ReplacePeopleRepositorySuiteCompanions(ctx context.Context, links CompanionRepository, studentID int64, edges []*careplan.CompanionEdge) error {
	return ReplaceStudentCompanions(ctx, links, studentID, edges)
}

// NewPeopleRepositorySuiteRelationships is the relationship store with its
// owners bound, as the composition root binds it.
func NewPeopleRepositorySuiteRelationships(db transactionruntime.Database) peoplePorts.StudentGuardianRepository {
	return NewStudentGuardianRepository(db)
}

// NewPeopleRepositorySuiteGuardianProfiles is the guardian profile store as
// the composition root binds it.
func NewPeopleRepositorySuiteGuardianProfiles(db transactionruntime.Database) peoplePorts.GuardianProfileRepository {
	return NewGuardianProfileRepository(db)
}

// NewEnrollmentGuardianAutofillSuite is the enrollment form's autofill
// reader as the composition root binds it, over the retained guardian profile
// store of db (#2734).
func NewEnrollmentGuardianAutofillSuite(db transactionruntime.Database) EnrollmentGuardianAutofill {
	return EnrollmentGuardianAutofill{Loader: peopleCompose.NewGuardianProfileLoader(NewGuardianProfileRepository(db), db.Pool(), nil)}
}

// NewPeopleRepositorySuiteRecipients is the parent-message recipient lookup
// as the composition root binds it.
func NewPeopleRepositorySuiteRecipients(db transactionruntime.Database) *peopleCompose.RetainedMessageableGuardians {
	return NewMessageableGuardianRepository(db)
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
	return peopleCompose.RetainedWithGuardianDefaultRole(func(link *peopledirectory.GuardianRelationshipSnapshot) {
		peopleCompose.ApplyDefaultGuardianRole(guardianRolePresets, link)
	})
}

// RepositoryTestFactory is the repository factory test graphs compose, next to
// the People Directory and School Membership repositories that left it
// (#2743).
type RepositoryTestFactory = PeopleRepositorySuiteFactory

// NewRepositoryTestFactory composes the test graph over deps.
func NewRepositoryTestFactory(db transactionruntime.Database, deps TimetableDependencies, clocks ...func() time.Time) *RepositoryTestFactory {
	return newPeopleRepositorySuiteFactory(db, deps, clocks...)
}

// NewRepositoryTestFactoryWithPeopleDirectory is NewRepositoryTestFactory for
// suites that handle a composition error.
func NewRepositoryTestFactoryWithPeopleDirectory(db transactionruntime.Database, deps TimetableDependencies, clocks ...func() time.Time) (*RepositoryTestFactory, error) {
	return newPeopleRepositorySuiteFactory(db, deps, clocks...), nil
}

// NewUnboundStudentRepository is the retained student repository without the
// School Membership teacher assignments and the School Structure group names:
// the teacher-scoped roster reads fail closed. Repository suites that only
// read and write the child's row use it.
func NewUnboundStudentRepository(db transactionruntime.Database) peoplePorts.StudentRepository {
	enrollment, err := NewSchoolMembership(db)
	if err != nil {
		panic(fmt.Sprintf("student repository: compose membership: %v", err))
	}
	return peopleCompose.NewStudentRepository(peopleCompose.StudentRepositoryDependencies{
		DB: db.Pool(), Students: studentDirectoryView.NewReader(), Directory: MustNewPeopleDirectory(db), Enrollment: enrollment,
	})
}
