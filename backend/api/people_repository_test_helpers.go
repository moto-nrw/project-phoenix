//go:build testhooks

package api

import (
	"context"
	"fmt"
	"time"

	recordports "github.com/moto-nrw/project-phoenix/modules/communication/ports"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/workforce"

	studentDirectoryView "github.com/moto-nrw/project-phoenix/modules/studentdirectoryview/compose"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
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

// PeopleRepositorySuiteFactory holds the People Directory repositories the
// retained suites drive, next to the School Membership repositories they
// read through, over the unobserved timetable dependencies (#2743). The rows
// of the other owners come from the suite repositories below, each holding
// one owner and each built over the same database.
type PeopleRepositorySuiteFactory struct {
	owners.MembershipTestRepositories
	GuardianPhoneNumber      peoplePorts.GuardianPhoneNumberRepository
	GuardianFinancialData    peoplePorts.GuardianFinancialDataRepository
	FamilyProtection         peoplePorts.FamilyProtectionRecords
	ParentRequestShare       peoplePorts.ParentRequestShareRecords
	ParentRequestEvent       peoplePorts.ParentRequestEventRecords
	GroupSubstitution        owners.GroupSubstitutions
	CaregiverBindingLock     peoplePorts.CaregiverBindingLocker
	CareWithdrawal           owners.CareWithdrawalTestRows
	StudentDataChangeRequest StudentDataRequestTestRows
}

// NewPeopleRepositorySuiteFactory composes the People Directory suite
// repositories over the unobserved timetable dependencies.
func NewPeopleRepositorySuiteFactory(db transactionruntime.Database) *PeopleRepositorySuiteFactory {
	return newPeopleRepositorySuiteFactory(db, owners.MustNewUnobservedTimetable(db), owners.MustNewPeopleDirectory(db), MustNewSchoolStructure(db),
		owners.MustNewUnobservedRooms(db), MustNewSchoolCalendar(db), owners.MustNewSchoolMembership(db), owners.MustNewWorkforce(db))
}

func newPeopleRepositorySuiteFactory(
	db transactionruntime.Database, capability timetable.Capability, students peopledirectory.Capability, groups schoolstructure.Query,
	rooms facilities.Capability, calendarOwner schoolcalendar.Capability, membership schoolmembership.Capability, workTime workforce.Capability,
) *PeopleRepositorySuiteFactory {
	if capability == nil || students == nil || groups == nil || rooms == nil || calendarOwner == nil || membership == nil || workTime == nil {
		panic("repository test factory: timetable and projection dependencies are required")
	}
	identity, persons := owners.NewPeopleIdentityAccess(db, nil), owners.NewPersonRepository(db)
	members := owners.NewMembershipRepositories(db, persons, identity, membership, rooms, workTime)
	members.People = students
	members.Student = owners.NewStudentRepository(db, membership, groups)
	members.GuardianProfile = owners.NewGuardianProfileRepository(db)
	members.StudentGuardian = owners.NewStudentGuardianRepository(db)
	// The withdrawal and change-request rows delegate to an unobserved Care
	// Plan owner of db, bound at construction (#2743).
	carePlan := mustNewSuiteCarePlan(db, capability, students)
	return &PeopleRepositorySuiteFactory{
		MembershipTestRepositories: members,
		GuardianPhoneNumber:        peopleCompose.NewGuardianPhoneNumberRepository(db.Pool()),
		GuardianFinancialData:      peopleCompose.NewGuardianFinancialDataRepository(db.Pool()),
		FamilyProtection:           peopleCompose.NewFamilyProtectionEventRepository(db.Pool()),
		ParentRequestShare:         peopleCompose.NewParentRequestShareEventRepository(db.Pool()),
		ParentRequestEvent:         peopleCompose.NewParentRequestEventRepository(db.Pool()),
		GroupSubstitution:          owners.NewGroupSubstitutionRepository(owners.NewSchoolStructureRepositories(db, persons, identity, membership, rooms, workTime).Group, membership, workTime),
		CaregiverBindingLock:       owners.NewCaregiverBindingLocker(membership, workTime, NewStudentPresenceForTests(db), capability),
		CareWithdrawal:             owners.NewCareWithdrawalCompletionRepository(carePlan),
		StudentDataChangeRequest:   NewStudentDataChangeRequestRepository(carePlan),
	}
}

func mustNewSuiteCarePlan(db transactionruntime.Database, capability timetable.Capability, students peopledirectory.Capability) careplan.Capability {
	carePlan, err := owners.NewUnobservedCarePlan(db, capability, students)
	if err != nil {
		panic(fmt.Sprintf("repository test factory: compose care plan: %v", err))
	}
	return carePlan
}

// CarePlanSuiteRepositories are the Care Plan rows of the retained suites:
// the owner itself, the status-day and excused-absence repositories over it
// and the guardian portal's child, phase and application rows. The suites
// reach them through RepositoryTestFactory.
type CarePlanSuiteRepositories struct {
	carePlanCompose.ParentPortalRepositories
	StudentStatusDay      *owners.StudentStatusDayRepository
	ExcusedAbsenceRequest *owners.ExcusedAbsenceRequestRepository
	carePlan              careplan.Capability
}

// CarePlan is the Care Plan owner the schedule, request and withdrawal
// repositories delegate to.
func (f *CarePlanSuiteRepositories) CarePlan() careplan.Capability { return f.carePlan }

// WithSchoolProjections returns a copy whose guardian portal rows resolve
// school names and eligibility through schools, as the service graph's own
// do.
func (f *CarePlanSuiteRepositories) WithSchoolProjections(schools organizationtenancy.Query) *CarePlanSuiteRepositories {
	if schools == nil {
		panic("parent portal repositories: organization tenancy query is required")
	}
	bound := *f
	bound.ParentPortalRepositories = f.WithSchools(owners.ParentPortalSchools{Schools: schools})
	return &bound
}

// NewCarePlanSuiteRepositories composes the Care Plan suite repositories over
// the unobserved owners of db.
func NewCarePlanSuiteRepositories(db transactionruntime.Database) *CarePlanSuiteRepositories {
	students := owners.MustNewPeopleDirectory(db)
	carePlan := mustNewSuiteCarePlan(db, owners.MustNewUnobservedTimetable(db), students)
	return &CarePlanSuiteRepositories{
		ParentPortalRepositories: owners.ComposeParentPortalReads(db, students, owners.NewPeopleIdentityAccess(db, nil), enrollmentCompose.New()),
		StudentStatusDay:         owners.NewStudentStatusDayRepository(carePlan),
		ExcusedAbsenceRequest:    owners.NewExcusedAbsenceRequestRepository(carePlan),
		carePlan:                 carePlan,
	}
}

// ParentMessagingSuiteRepositories are the Communication stores of the
// guardian portal the retained suites drive.
type ParentMessagingSuiteRepositories struct {
	parentStore.MessagingRepositories
}

// WithSchoolProjections returns a copy whose announcement and inbox rows
// resolve school names through schools.
func (r *ParentMessagingSuiteRepositories) WithSchoolProjections(schools organizationtenancy.Query) *ParentMessagingSuiteRepositories {
	if schools == nil {
		panic("parent portal repositories: organization tenancy query is required")
	}
	bound := *r
	bound.MessagingRepositories = r.WithSchoolNames(owners.ParentPortalSchools{Schools: schools})
	return &bound
}

// NewParentMessagingSuiteRepositories composes the guardian portal's
// Communication stores over the unobserved owners of db.
func NewParentMessagingSuiteRepositories(db transactionruntime.Database, clocks ...func() time.Time) *ParentMessagingSuiteRepositories {
	return &ParentMessagingSuiteRepositories{MessagingRepositories: owners.ComposeParentMessagingRepositories(
		db, MustNewSchoolStructure(db), owners.MustNewSchoolMembership(db), owners.NewPeopleIdentityAccess(db, nil), enrollmentCompose.New(), OptionalClock(clocks),
	)}
}

// PresenceSuiteRepositories are the Student Presence session rows and
// queries the retained suites drive.
type PresenceSuiteRepositories struct {
	ActiveGroup      studentpresence.SessionRecords
	GroupSupervisor  studentpresence.SupervisionRecords
	CrossTenant      owners.CrossTenantQuery
	SessionStartLock interface {
		LockSessionStart(context.Context, int64) error
	}
}

// NewPresenceSuiteRepositories composes the session rows over the unobserved
// owners of db and the given clock.
func NewPresenceSuiteRepositories(db transactionruntime.Database, clocks ...func() time.Time) *PresenceSuiteRepositories {
	devices, err := owners.NewDeviceFleet(db)
	if err != nil {
		panic(fmt.Sprintf("repository test factory: compose device fleet: %v", err))
	}
	students, groups := owners.MustNewPeopleDirectory(db), MustNewSchoolStructure(db)
	presence := NewStudentPresenceForTests(db)
	sessions := owners.NewSessionRecords(db, OptionalClock(clocks), devices, owners.MustNewUnobservedRooms(db), owners.MustNewUnobservedTimetable(db), owners.MustNewSchoolMembership(db), students)
	return &PresenceSuiteRepositories{
		ActiveGroup:      sessions,
		GroupSupervisor:  owners.NewGroupSupervisionRecords(sessions, groups),
		CrossTenant:      owners.NewCrossTenantQuery(presence, students, groups),
		SessionStartLock: presence,
	}
}

// CareScheduleSuiteRepositories are the Timetable care-schedule and
// school-calendar rows the retained suites drive.
type CareScheduleSuiteRepositories struct {
	owners.CareScheduleRepositories
	timetableCompose.SchoolCalendarRepositories
}

// NewCareScheduleSuiteRepositories composes the rows over the unobserved
// owners of db.
func NewCareScheduleSuiteRepositories(db transactionruntime.Database) *CareScheduleSuiteRepositories {
	capability := owners.MustNewUnobservedTimetable(db)
	calendar, err := owners.NewSchoolCalendar(db)
	if err != nil {
		panic(fmt.Sprintf("repository test factory: compose school calendar: %v", err))
	}
	return &CareScheduleSuiteRepositories{
		CareScheduleRepositories:   owners.NewCareScheduleRepositories(mustNewSuiteCarePlan(db, capability, owners.MustNewPeopleDirectory(db))),
		SchoolCalendarRepositories: timetableCompose.NewSchoolCalendarRepositories(calendar, timetableCompose.NewPlanningCalendarPeriodUsage(enrollmentCompose.New(), capability)),
	}
}

// StaffAbsenceSuiteRepositories are the Workforce absence rows the retained
// suites drive.
type StaffAbsenceSuiteRepositories struct {
	StaffAbsence workforceCompose.AbsenceRequestRecords
}

// NewStaffAbsenceSuiteRepositories composes the absence rows over the
// unobserved owners of db.
func NewStaffAbsenceSuiteRepositories(db transactionruntime.Database) *StaffAbsenceSuiteRepositories {
	return &StaffAbsenceSuiteRepositories{
		StaffAbsence: owners.NewStaffAbsenceRepository(owners.MustNewWorkforce(db), owners.MustNewSchoolMembership(db), owners.MustNewPeopleDirectory(db)),
	}
}

// AccountSuiteRepositories are the Identity & Access cards and account
// profiles the repository factory used to hand out.
type AccountSuiteRepositories struct {
	RFIDCard identityaccess.RFIDCards
	Profile  identityaccess.AccountProfiles
}

// NewAccountSuiteRepositories composes the rows over db.
func NewAccountSuiteRepositories(db transactionruntime.Database) *AccountSuiteRepositories {
	identity := owners.NewPeopleIdentityAccess(db, nil)
	return &AccountSuiteRepositories{RFIDCard: identity, Profile: identity}
}

// EnrollmentSuiteRepositories serve the Enrollment booking projection of the
// retained suites.
type EnrollmentSuiteRepositories struct {
	enrollment enrollmentModule.Owner
}

// Enrollment is the Enrollment booking projection.
func (f *EnrollmentSuiteRepositories) Enrollment() owners.EnrollmentBookingProjection {
	return owners.NewEnrollmentBookingProjection(owners.ComposeEnrollmentBookings(f.enrollment))
}

// NewEnrollmentSuiteRepositories composes the projection's owner.
func NewEnrollmentSuiteRepositories(transactionruntime.Database) *EnrollmentSuiteRepositories {
	return &EnrollmentSuiteRepositories{enrollment: enrollmentCompose.New()}
}

// SchoolCalendarSuiteRepositories serve the School Calendar owner the
// period, closing day and dateframe repositories delegate to.
type SchoolCalendarSuiteRepositories struct {
	schoolCalendar schoolcalendar.Calendar
}

// SchoolCalendar is the calendar owner.
func (f *SchoolCalendarSuiteRepositories) SchoolCalendar() schoolcalendar.Calendar {
	return f.schoolCalendar
}

// NewSchoolCalendarSuiteRepositories composes the calendar owner of db.
func NewSchoolCalendarSuiteRepositories(db transactionruntime.Database) *SchoolCalendarSuiteRepositories {
	calendar, err := owners.NewSchoolCalendar(db)
	if err != nil {
		panic(fmt.Sprintf("repository test factory: compose school calendar: %v", err))
	}
	return &SchoolCalendarSuiteRepositories{schoolCalendar: calendar}
}

// NewPeopleRepositorySuitePeople is the People Directory owner.
func NewPeopleRepositorySuitePeople(db transactionruntime.Database) peopledirectory.Capability {
	return owners.MustNewPeopleDirectory(db)
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
// its Enrollment audience, constructed here rather than by shared fixtures.
func NewPeopleRepositorySuiteAnnouncements(db transactionruntime.Database, clocks ...func() time.Time) recordports.ParentAnnouncementRepository {
	return parentStore.NewParentAnnouncementRepository(db.Pool(), owners.NewParentAnnouncementAudience(db, owners.ParentAnnouncementApplicants(enrollmentCompose.New()), clocks...))
}

// NewPeopleRepositorySuiteCaregiverBindingLocker is the caregiver binding
// lock over the four owners that hold caregiver capability.
func NewPeopleRepositorySuiteCaregiverBindingLocker(db transactionruntime.Database) peoplePorts.CaregiverBindingLocker {
	return owners.NewCaregiverBindingLocker(owners.MustNewSchoolMembership(db), owners.MustNewWorkforce(db), NewStudentPresenceForTests(db), owners.MustNewUnobservedTimetable(db))
}

// NewPeopleRepositorySuiteCompanions is the companion store over Care Plan.
func NewPeopleRepositorySuiteCompanions(capability careplan.Capability) owners.CompanionRepository {
	return owners.NewStudentCompanionRepository(capability)
}

// NewPeopleRepositorySuiteCompanionEdge validates one companion edge.
func NewPeopleRepositorySuiteCompanionEdge(studentID, companionID int64, weekday int) (*careplan.CompanionEdge, error) {
	return NewStudentCompanionEdge(studentID, companionID, weekday)
}

// ReplacePeopleRepositorySuiteCompanions replaces a child's companion edges.
func ReplacePeopleRepositorySuiteCompanions(ctx context.Context, links owners.CompanionRepository, studentID int64, edges []*careplan.CompanionEdge) error {
	return ReplaceStudentCompanions(ctx, links, studentID, edges)
}

// NewPeopleRepositorySuiteRelationships is the relationship store with its
// owners bound, as the composition root binds it.
func NewPeopleRepositorySuiteRelationships(db transactionruntime.Database) peoplePorts.StudentGuardianRepository {
	return owners.NewStudentGuardianRepository(db)
}

// NewPeopleRepositorySuiteGuardianProfiles is the guardian profile store as
// the composition root binds it.
func NewPeopleRepositorySuiteGuardianProfiles(db transactionruntime.Database) peoplePorts.GuardianProfileRepository {
	return owners.NewGuardianProfileRepository(db)
}

// NewEnrollmentGuardianAutofillSuite is the enrollment form's autofill
// reader as the composition root binds it, over the retained guardian profile
// store of db (#2734).
func NewEnrollmentGuardianAutofillSuite(db transactionruntime.Database) EnrollmentGuardianAutofill {
	return EnrollmentGuardianAutofill{Loader: peopleCompose.NewGuardianProfileLoader(owners.NewGuardianProfileRepository(db), db.Pool(), nil)}
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
		peopleCompose.ApplyDefaultGuardianRole(owners.GuardianRolePresets, link)
	})
}

// RepositoryTestFactory names the Care Plan suite repositories for the
// suites that compose the retained repositories; the People Directory and
// the other owners' rows come from the suite repositories above (#4080).
type RepositoryTestFactory = CarePlanSuiteRepositories

// NewRepositoryTestFactory composes the Care Plan suite repositories over the
// unobserved owners of db.
func NewRepositoryTestFactory(db transactionruntime.Database) *RepositoryTestFactory {
	return NewCarePlanSuiteRepositories(db)
}

// NewRepositoryTestFactoryWithPeopleDirectory is NewPeopleRepositorySuiteFactory
// for suites that handle a composition error.
func NewRepositoryTestFactoryWithPeopleDirectory(db transactionruntime.Database) (*PeopleRepositorySuiteFactory, error) {
	return NewPeopleRepositorySuiteFactory(db), nil
}

// NewUnboundStudentRepository is the retained student repository without the
// School Membership teacher assignments and the School Structure group names:
// the teacher-scoped roster reads fail closed. Repository suites that only
// read and write the child's row use it.
func NewUnboundStudentRepository(db transactionruntime.Database) peoplePorts.StudentRepository {
	enrollment, err := owners.NewSchoolMembership(db)
	if err != nil {
		panic(fmt.Sprintf("student repository: compose membership: %v", err))
	}
	return peopleCompose.NewStudentRepository(peopleCompose.StudentRepositoryDependencies{
		DB: db.Pool(), Students: studentDirectoryView.NewReader(), Directory: owners.MustNewPeopleDirectory(db), Enrollment: enrollment,
	})
}
