package testutil

import (
	"context"
	"time"

	"github.com/uptrace/bun"

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
var NewPeopleRepositorySuiteAnnouncements = services.NewPeopleRepositorySuiteAnnouncements

// NewPeopleRepositorySuiteCaregiverBindingLocker is the caregiver binding
// lock over the four owners that hold caregiver capability.
var NewPeopleRepositorySuiteCaregiverBindingLocker = services.NewPeopleRepositorySuiteCaregiverBindingLocker

// NewPeopleRepositorySuiteCompanions is the companion store over Care Plan.
var NewPeopleRepositorySuiteCompanions = services.NewPeopleRepositorySuiteCompanions

// NewPeopleRepositorySuiteCompanionEdge validates one companion edge.
var NewPeopleRepositorySuiteCompanionEdge = services.NewPeopleRepositorySuiteCompanionEdge

// ReplacePeopleRepositorySuiteCompanions replaces a child's companion edges.
var ReplacePeopleRepositorySuiteCompanions = services.ReplacePeopleRepositorySuiteCompanions

// NewPeopleRepositorySuiteRelationships is the relationship store with its
// owners bound, as the composition root binds it.
var NewPeopleRepositorySuiteRelationships = services.NewPeopleRepositorySuiteRelationships

// NewPeopleRepositorySuiteGuardianProfiles is the guardian profile store as
// the composition root binds it.
var NewPeopleRepositorySuiteGuardianProfiles = services.NewPeopleRepositorySuiteGuardianProfiles

// NewPeopleRepositorySuiteRecipients is the parent-message recipient lookup
// as the composition root binds it.
func NewPeopleRepositorySuiteRecipients(db *bun.DB) *services.PeopleRepositorySuiteRecipients {
	return services.NewPeopleRepositorySuiteRecipients(db)
}

// NewPeopleRepositorySuiteRetainedPersons is the person store without the
// account lookup.
var NewPeopleRepositorySuiteRetainedPersons = services.NewPeopleRepositorySuiteRetainedPersons

// NewPeopleRepositorySuiteRetainedGuardianProfiles is the guardian profile
// store with only the given options bound.
var NewPeopleRepositorySuiteRetainedGuardianProfiles = services.NewPeopleRepositorySuiteRetainedGuardianProfiles

// NewPeopleRepositorySuiteRetainedRelationships is the relationship store
// with only the given options bound.
var NewPeopleRepositorySuiteRetainedRelationships = services.NewPeopleRepositorySuiteRetainedRelationships

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
