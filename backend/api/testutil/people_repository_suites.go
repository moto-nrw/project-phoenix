package testutil

import (
	"context"
	"time"

	"github.com/uptrace/bun"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services"
)

// The compositions the retained People Directory repository suites
// (database/repositories/users) drive (#2727), composed in services so the
// suites import neither the retained repositories nor the retained models
// and services. Every entry goes with the last suite that uses it.

type (
	PeopleRepositorySuitePeople                = services.PeopleRepositorySuitePeople
	PeopleRepositorySuiteSchoolStructure       = services.PeopleRepositorySuiteSchoolStructure
	PeopleRepositorySuiteCarePlan              = services.PeopleRepositorySuiteCarePlan
	PeopleRepositorySuiteCompanionLinks        = services.PeopleRepositorySuiteCompanionLinks
	PeopleRepositorySuiteFactory               = services.PeopleRepositorySuiteFactory
	PeopleRepositorySuiteParentMessaging       = services.PeopleRepositorySuiteParentMessaging
	PeopleRepositorySuiteCareLifecycle         = services.PeopleRepositorySuiteCareLifecycle
	PeopleRepositorySuiteAuditCommand          = services.PeopleRepositorySuiteAuditCommand
	PeopleRepositorySuiteAnnouncementAudience  = services.PeopleRepositorySuiteAnnouncementAudience
	PeopleRepositorySuiteGuardianProfileOption = services.PeopleRepositorySuiteGuardianProfileOption
	PeopleRepositorySuiteRelationshipOption    = services.PeopleRepositorySuiteRelationshipOption
	PeopleRepositorySuitePortalMemberships     = services.PeopleRepositorySuitePortalMemberships
	PeopleRepositorySuiteSchoolMemberships     = services.PeopleRepositorySuiteSchoolMemberships
	PeopleRepositorySuiteStaffAccounts         = services.PeopleRepositorySuiteStaffAccounts
	PeopleRepositorySuiteStaffMessageIdentity  = services.PeopleRepositorySuiteStaffMessageIdentity
	PeopleRepositorySuiteSchoolRoleClass       = services.PeopleRepositorySuiteSchoolRoleClass
	PeopleRepositorySuitePickupCommand         = services.PeopleRepositorySuitePickupCommand
	PeopleRepositorySuiteAccessCommand         = services.PeopleRepositorySuiteAccessCommand
	PeopleRepositorySuiteRecipients            = services.PeopleRepositorySuiteRecipients
	PeopleRepositorySuiteColleagues            = services.PeopleRepositorySuiteColleagues
)

// NewPeopleRepositorySuiteFactory is the legacy repository factory over the
// unobserved timetable dependencies.
func NewPeopleRepositorySuiteFactory(db *bun.DB, clocks ...func() time.Time) *services.PeopleRepositorySuiteFactory {
	return services.NewPeopleRepositorySuiteFactory(db, clocks...)
}

// NewPeopleRepositorySuitePeople is the People Directory owner.
func NewPeopleRepositorySuitePeople(db *bun.DB) services.PeopleRepositorySuitePeople {
	return services.NewPeopleRepositorySuitePeople(db)
}

// NewPeopleRepositorySuiteSchoolStructure is the School Structure owner.
func NewPeopleRepositorySuiteSchoolStructure(db *bun.DB) (services.PeopleRepositorySuiteSchoolStructure, error) {
	return services.NewPeopleRepositorySuiteSchoolStructure(db)
}

// NewPeopleRepositorySuiteCareLifecycle composes the care lifecycle; a nil
// command records into the test audit store.
func NewPeopleRepositorySuiteCareLifecycle(db *bun.DB, command services.PeopleRepositorySuiteAuditCommand) (services.PeopleRepositorySuiteCareLifecycle, error) {
	return services.NewPeopleRepositorySuiteCareLifecycle(db, command)
}

// NewPeopleRepositorySuiteParentMessaging composes the parent messaging stores.
func NewPeopleRepositorySuiteParentMessaging(db *bun.DB) (services.PeopleRepositorySuiteParentMessaging, error) {
	return services.NewPeopleRepositorySuiteParentMessaging(db)
}

// NewPeopleRepositorySuiteAnnouncements is the parent announcement store over
// the given Enrollment audience queries.
func NewPeopleRepositorySuiteAnnouncements(db *bun.DB, audience services.PeopleRepositorySuiteAnnouncementAudience, clocks ...func() time.Time) usersModels.ParentAnnouncementRepository {
	return services.NewPeopleRepositorySuiteAnnouncements(db, audience, clocks...)
}

// NewPeopleRepositorySuiteCaregiverBindingLocker is the caregiver binding
// lock over the four owners that hold caregiver capability.
func NewPeopleRepositorySuiteCaregiverBindingLocker(db *bun.DB) usersModels.CaregiverBindingLocker {
	return services.NewPeopleRepositorySuiteCaregiverBindingLocker(db)
}

// NewPeopleRepositorySuiteCompanions is the companion store over Care Plan.
func NewPeopleRepositorySuiteCompanions(capability services.PeopleRepositorySuiteCarePlan) usersModels.StudentCompanionRepository {
	return services.NewPeopleRepositorySuiteCompanions(capability)
}

// NewPeopleRepositorySuiteCompanionEdge validates one companion edge.
func NewPeopleRepositorySuiteCompanionEdge(studentID, companionID int64, weekday int) (*usersModels.StudentCompanion, error) {
	return services.NewPeopleRepositorySuiteCompanionEdge(studentID, companionID, weekday)
}

// ReplacePeopleRepositorySuiteCompanions replaces a child's companion edges.
func ReplacePeopleRepositorySuiteCompanions(ctx context.Context, links services.PeopleRepositorySuiteCompanionLinks, studentID int64, edges []*usersModels.StudentCompanion) error {
	return services.ReplacePeopleRepositorySuiteCompanions(ctx, links, studentID, edges)
}

// NewPeopleRepositorySuiteRelationships is the relationship store with its
// owners bound, as the composition root binds it.
func NewPeopleRepositorySuiteRelationships(db *bun.DB) usersModels.StudentGuardianRepository {
	return services.NewPeopleRepositorySuiteRelationships(db)
}

// NewPeopleRepositorySuiteGuardianProfiles is the guardian profile store as
// the composition root binds it.
func NewPeopleRepositorySuiteGuardianProfiles(db *bun.DB) usersModels.GuardianProfileRepository {
	return services.NewPeopleRepositorySuiteGuardianProfiles(db)
}

// NewPeopleRepositorySuiteRecipients is the parent-message recipient lookup
// as the composition root binds it.
func NewPeopleRepositorySuiteRecipients(db *bun.DB) *services.PeopleRepositorySuiteRecipients {
	return services.NewPeopleRepositorySuiteRecipients(db)
}

// NewPeopleRepositorySuiteRetainedPersons is the person store without the
// account lookup.
func NewPeopleRepositorySuiteRetainedPersons(db *bun.DB) usersModels.PersonRepository {
	return services.NewPeopleRepositorySuiteRetainedPersons(db)
}

// NewPeopleRepositorySuiteRetainedGuardianProfiles is the guardian profile
// store with only the given options bound.
func NewPeopleRepositorySuiteRetainedGuardianProfiles(db *bun.DB, options ...services.PeopleRepositorySuiteGuardianProfileOption) usersModels.GuardianProfileRepository {
	return services.NewPeopleRepositorySuiteRetainedGuardianProfiles(db, options...)
}

// NewPeopleRepositorySuiteRetainedRelationships is the relationship store
// with only the given options bound.
func NewPeopleRepositorySuiteRetainedRelationships(db *bun.DB, options ...services.PeopleRepositorySuiteRelationshipOption) usersModels.StudentGuardianRepository {
	return services.NewPeopleRepositorySuiteRetainedRelationships(db, options...)
}

// NewPeopleRepositorySuiteRetainedRecipients is the recipient lookup over the
// given membership lookup.
func NewPeopleRepositorySuiteRetainedRecipients(db *bun.DB, memberships services.PeopleRepositorySuiteSchoolMemberships) *services.PeopleRepositorySuiteRecipients {
	return services.NewPeopleRepositorySuiteRetainedRecipients(db, memberships)
}

// NewPeopleRepositorySuiteRetainedColleagues is the colleague lookup over the
// given owner queries.
func NewPeopleRepositorySuiteRetainedColleagues(db *bun.DB, staffAccounts services.PeopleRepositorySuiteStaffAccounts, identity services.PeopleRepositorySuiteStaffMessageIdentity) *services.PeopleRepositorySuiteColleagues {
	return services.NewPeopleRepositorySuiteRetainedColleagues(db, staffAccounts, identity)
}

// PeopleRepositorySuiteWithPortalMemberships binds the reachability query.
func PeopleRepositorySuiteWithPortalMemberships(query services.PeopleRepositorySuitePortalMemberships) services.PeopleRepositorySuiteGuardianProfileOption {
	return services.PeopleRepositorySuiteWithPortalMemberships(query)
}

// PeopleRepositorySuiteWithMemberships binds the relationship membership lookup.
func PeopleRepositorySuiteWithMemberships(query services.PeopleRepositorySuiteSchoolMemberships) services.PeopleRepositorySuiteRelationshipOption {
	return services.PeopleRepositorySuiteWithMemberships(query)
}

// PeopleRepositorySuiteWithOwners binds the relationship's owner halves.
func PeopleRepositorySuiteWithOwners(pickup services.PeopleRepositorySuitePickupCommand, access services.PeopleRepositorySuiteAccessCommand) services.PeopleRepositorySuiteRelationshipOption {
	return services.PeopleRepositorySuiteWithOwners(pickup, access)
}

// PeopleRepositorySuiteWithDefaultRole binds the default role preset.
func PeopleRepositorySuiteWithDefaultRole() services.PeopleRepositorySuiteRelationshipOption {
	return services.PeopleRepositorySuiteWithDefaultRole()
}

// VerifyPeopleRepositorySuiteStudentSchema runs the students table check.
func VerifyPeopleRepositorySuiteStudentSchema(ctx context.Context, db *bun.DB) error {
	return services.VerifyPeopleRepositorySuiteStudentSchema(ctx, db)
}
