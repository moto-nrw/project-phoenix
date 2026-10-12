package services

import (
	"context"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/services/users"
)

// The compositions the retained People Directory repository suites
// (database/repositories/users) drive (#2727), composed here so the suites
// import neither the retained repositories nor the legacy composition. Every
// entry goes with the last suite that uses it.

// The composed graphs and the values the suites pass back into them.
type (
	PeopleRepositorySuiteParentMessaging       = repositories.ParentMessagingTestRepositories
	PeopleRepositorySuiteCareLifecycle         = repositories.CareLifecycleTestRepositories
	PeopleRepositorySuiteAuditCommand          = auditModels.Command
	PeopleRepositorySuiteAnnouncementAudience  = repositories.AnnouncementEnrollmentQueries
	PeopleRepositorySuiteGuardianProfileOption = repositories.RetainedGuardianProfileOption
	PeopleRepositorySuiteRelationshipOption    = repositories.RetainedGuardianRelationshipOption
	PeopleRepositorySuitePortalMemberships     = repositories.RetainedPortalMembershipQuery
	PeopleRepositorySuiteSchoolMemberships     = repositories.RetainedSchoolMembershipLookup
	PeopleRepositorySuiteStaffAccounts         = repositories.RetainedStaffAccountsFunc
	PeopleRepositorySuiteStaffMessageIdentity  = repositories.RetainedStaffMessageIdentity
	PeopleRepositorySuiteSchoolRoleClass       = repositories.RetainedSchoolRoleClass
	PeopleRepositorySuitePickupCommand         = repositories.RetainedGuardianPickupCommand
	PeopleRepositorySuiteAccessCommand         = repositories.RetainedGuardianAccessCommand
	PeopleRepositorySuiteRecipients            = repositories.RetainedMessageableGuardians
	PeopleRepositorySuiteColleagues            = repositories.RetainedMessageableStaff
	PeopleRepositorySuitePeople                = peopledirectory.Capability
	PeopleRepositorySuiteSchoolStructure       = schoolstructure.Capability
	PeopleRepositorySuiteCarePlan              = careplan.Capability
	PeopleRepositorySuiteCompanionLinks        = usersModels.StudentCompanionRepository
)

// PeopleRepositorySuiteFactory is the legacy repository factory over the
// unobserved timetable dependencies, next to the People Directory and School
// Membership repositories built where they are used (#2743).
type PeopleRepositorySuiteFactory struct {
	*repositories.Factory
	repositories.MembershipTestRepositories
	GuardianPhoneNumber   usersModels.GuardianPhoneNumberRepository
	GuardianFinancialData usersModels.GuardianFinancialDataRepository
	FamilyProtection      usersModels.FamilyProtectionEventRepository
	ParentRequestShare    usersModels.ParentRequestShareEventRepository
	ParentRequestEvent    usersModels.ParentRequestEventRepository
	GroupSubstitution     repositories.GroupSubstitutionRepository
	CaregiverBindingLock  usersModels.CaregiverBindingLocker
	// RFIDCard and Profile are the Identity & Access cards and account
	// profiles the factory used to hand out.
	RFIDCard identityaccess.RFIDCards
	Profile  identityaccess.AccountProfiles
}

// SchoolMembership is the School Membership owner the staff adapters read
// through.
func (f *PeopleRepositorySuiteFactory) SchoolMembership() schoolmembership.Capability {
	return f.Membership
}

// NewPeopleRepositorySuiteFactory composes the suite graph over the unobserved
// timetable dependencies.
func NewPeopleRepositorySuiteFactory(db *bun.DB, clocks ...func() time.Time) *PeopleRepositorySuiteFactory {
	return newPeopleRepositorySuiteFactory(db, NewUnobservedTimetableDependencies(db), clocks...)
}

func newPeopleRepositorySuiteFactory(db *bun.DB, deps repositories.TimetableDependencies, clocks ...func() time.Time) *PeopleRepositorySuiteFactory {
	identity := newPeopleIdentityAccess(db, nil)
	members := newMembershipRepositories(db, NewPersonRepository(db), identity, deps.Membership, deps.Rooms, deps.Workforce)
	members.People = deps.Students
	members.Student = NewStudentRepository(db, deps.Membership, deps.Groups)
	members.GuardianProfile = NewGuardianProfileRepository(db)
	members.StudentGuardian = NewStudentGuardianRepository(db)
	return &PeopleRepositorySuiteFactory{
		Factory:                    repositories.NewRetainedRepositoryFactory(db, deps, clocks...),
		MembershipTestRepositories: members,
		GuardianPhoneNumber:        peopleCompose.NewGuardianPhoneNumberRepository(db),
		GuardianFinancialData:      peopleCompose.NewGuardianFinancialDataRepository(db),
		FamilyProtection:           peopleCompose.NewFamilyProtectionEventRepository(db),
		ParentRequestShare:         peopleCompose.NewParentRequestShareEventRepository(db),
		ParentRequestEvent:         peopleCompose.NewParentRequestEventRepository(db),
		GroupSubstitution:          newGroupSubstitutionRepository(members.Group, deps.Membership, deps.Workforce),
		CaregiverBindingLock:       NewCaregiverBindingLocker(deps.Membership, deps.Workforce, repositories.NewStudentPresenceForTests(db), deps.Capability),
		RFIDCard:                   identity, Profile: identity,
	}
}

// NewPeopleRepositorySuitePeople is the People Directory owner.
func NewPeopleRepositorySuitePeople(db *bun.DB) peopledirectory.Capability {
	return MustNewPeopleDirectory(db)
}

// NewPeopleRepositorySuiteSchoolStructure is the School Structure owner.
func NewPeopleRepositorySuiteSchoolStructure(db *bun.DB) (schoolstructure.Capability, error) {
	return repositories.NewSchoolStructure(db)
}

// NewPeopleRepositorySuiteCareLifecycle composes the care lifecycle; a nil
// command records into the test audit store.
func NewPeopleRepositorySuiteCareLifecycle(db *bun.DB, command auditModels.Command) (repositories.CareLifecycleTestRepositories, error) {
	return NewCareLifecycleTestRepositories(db, command)
}

// NewPeopleRepositorySuiteParentMessaging composes the parent messaging stores.
func NewPeopleRepositorySuiteParentMessaging(db *bun.DB) (repositories.ParentMessagingTestRepositories, error) {
	return repositories.NewParentMessagingTestRepositories(db)
}

// NewPeopleRepositorySuiteAnnouncements is the parent announcement store over
// the given Enrollment audience queries.
func NewPeopleRepositorySuiteAnnouncements(db *bun.DB, audience repositories.AnnouncementEnrollmentQueries, clocks ...func() time.Time) usersModels.ParentAnnouncementRepository {
	return repositories.NewParentAnnouncementRepository(db, audience, clocks...)
}

// NewPeopleRepositorySuiteCaregiverBindingLocker is the caregiver binding
// lock over the four owners that hold caregiver capability.
func NewPeopleRepositorySuiteCaregiverBindingLocker(db *bun.DB) usersModels.CaregiverBindingLocker {
	deps := NewUnobservedTimetableDependencies(db)
	return NewCaregiverBindingLocker(deps.Membership, deps.Workforce, repositories.NewStudentPresenceForTests(db), deps.Capability)
}

// NewPeopleRepositorySuiteCompanions is the companion store over Care Plan.
func NewPeopleRepositorySuiteCompanions(capability careplan.Capability) usersModels.StudentCompanionRepository {
	return repositories.NewStudentCompanionRepository(capability)
}

// NewPeopleRepositorySuiteCompanionEdge validates one companion edge.
func NewPeopleRepositorySuiteCompanionEdge(studentID, companionID int64, weekday int) (*usersModels.StudentCompanion, error) {
	return repositories.NewStudentCompanionEdge(studentID, companionID, weekday)
}

// ReplacePeopleRepositorySuiteCompanions replaces a child's companion edges.
func ReplacePeopleRepositorySuiteCompanions(ctx context.Context, links usersModels.StudentCompanionRepository, studentID int64, edges []*usersModels.StudentCompanion) error {
	return repositories.ReplaceStudentCompanions(ctx, links, studentID, edges)
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
func NewPeopleRepositorySuiteRecipients(db *bun.DB) *repositories.RetainedMessageableGuardians {
	return repositories.NewMessageableGuardianRepository(db)
}

// NewPeopleRepositorySuiteRetainedPersons is the person store without the
// account lookup.
func NewPeopleRepositorySuiteRetainedPersons(db *bun.DB) usersModels.PersonRepository {
	return repositories.NewRetainedPersonRepository(db)
}

// NewPeopleRepositorySuiteRetainedGuardianProfiles is the guardian profile
// store with only the given options bound.
func NewPeopleRepositorySuiteRetainedGuardianProfiles(db *bun.DB, options ...repositories.RetainedGuardianProfileOption) usersModels.GuardianProfileRepository {
	return repositories.NewRetainedGuardianProfileRepository(db, options...)
}

// NewPeopleRepositorySuiteRetainedRelationships is the relationship store
// with only the given options bound.
func NewPeopleRepositorySuiteRetainedRelationships(db *bun.DB, options ...repositories.RetainedGuardianRelationshipOption) usersModels.StudentGuardianRepository {
	return repositories.NewRetainedGuardianRelationshipRepository(db, options...)
}

// NewPeopleRepositorySuiteRetainedRecipients is the recipient lookup over the
// given membership lookup.
func NewPeopleRepositorySuiteRetainedRecipients(db *bun.DB, memberships repositories.RetainedSchoolMembershipLookup) *repositories.RetainedMessageableGuardians {
	return repositories.NewRetainedMessageableGuardianRepository(db, memberships)
}

// NewPeopleRepositorySuiteRetainedColleagues is the colleague lookup over the
// given owner queries.
func NewPeopleRepositorySuiteRetainedColleagues(db *bun.DB, staffAccounts repositories.RetainedStaffAccountsFunc, identity repositories.RetainedStaffMessageIdentity) *repositories.RetainedMessageableStaff {
	return repositories.NewRetainedMessageableStaffRepository(db, staffAccounts, identity)
}

// PeopleRepositorySuiteWithPortalMemberships binds the reachability query.
func PeopleRepositorySuiteWithPortalMemberships(query repositories.RetainedPortalMembershipQuery) repositories.RetainedGuardianProfileOption {
	return repositories.RetainedWithPortalMemberships(query)
}

// PeopleRepositorySuiteWithMemberships binds the relationship membership lookup.
func PeopleRepositorySuiteWithMemberships(query repositories.RetainedSchoolMembershipLookup) repositories.RetainedGuardianRelationshipOption {
	return repositories.RetainedWithGuardianRelationshipMemberships(query)
}

// PeopleRepositorySuiteWithOwners binds the relationship's owner halves.
func PeopleRepositorySuiteWithOwners(pickup repositories.RetainedGuardianPickupCommand, access repositories.RetainedGuardianAccessCommand) repositories.RetainedGuardianRelationshipOption {
	return repositories.RetainedWithGuardianRelationshipOwners(pickup, access)
}

// PeopleRepositorySuiteWithDefaultRole binds the default role preset.
func PeopleRepositorySuiteWithDefaultRole() repositories.RetainedGuardianRelationshipOption {
	return repositories.RetainedWithGuardianDefaultRole(applyDefaultGuardianRole)
}

// VerifyPeopleRepositorySuiteStudentSchema runs the students table check.
func VerifyPeopleRepositorySuiteStudentSchema(ctx context.Context, db *bun.DB) error {
	return repositories.VerifyRetainedStudentSchema(ctx, db)
}

// RepositoryTestFactory is the repository factory test graphs compose, next to
// the People Directory and School Membership repositories that left it
// (#2743).
type RepositoryTestFactory = PeopleRepositorySuiteFactory

// NewRepositoryTestFactory composes the test graph over deps.
func NewRepositoryTestFactory(db *bun.DB, deps repositories.TimetableDependencies, clocks ...func() time.Time) *RepositoryTestFactory {
	return newPeopleRepositorySuiteFactory(db, deps, clocks...)
}

// NewRepositoryTestFactoryWithPeopleDirectory is NewRepositoryTestFactory for
// suites that handle a composition error.
func NewRepositoryTestFactoryWithPeopleDirectory(db *bun.DB, deps repositories.TimetableDependencies, clocks ...func() time.Time) (*RepositoryTestFactory, error) {
	return newPeopleRepositorySuiteFactory(db, deps, clocks...), nil
}
