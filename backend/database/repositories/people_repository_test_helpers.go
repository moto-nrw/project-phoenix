package repositories

import (
	"context"
	"time"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// The retained People Directory repository suites (database/repositories/users)
// may import neither that package nor this one (#2727). They compose the
// retained repositories over the tenant runtime through the test support that
// forwards these (services, api/testutil). Every entry goes with the last
// suite that uses it.

// Option and value types of the retained People Directory repositories.
type (
	RetainedGuardianProfileOption      = usersRepo.GuardianProfileOption
	RetainedGuardianRelationshipOption = usersRepo.GuardianRelationshipOption
	RetainedPortalMembershipQuery      = usersRepo.PortalMembershipQuery
	RetainedSchoolMembershipLookup     = usersRepo.SchoolMembershipLookup
	RetainedStaffAccountsFunc          = usersRepo.StaffAccountsFunc
	RetainedStaffMessageIdentity       = usersRepo.StaffMessageIdentity
	RetainedSchoolRoleClass            = usersRepo.SchoolRoleClass
	RetainedGuardianPickupCommand      = usersRepo.GuardianPickupPermissionCommand
	RetainedGuardianAccessCommand      = usersRepo.GuardianStudentAccessCommand
	RetainedMessageableGuardians       = usersRepo.MessageableGuardianRepository
	RetainedMessageableStaff           = usersRepo.MessageableStaffRepository
)

// NewRetainedCaregiverBindingLocker is the caregiver binding lock over the
// four owners that hold caregiver capability.
func NewRetainedCaregiverBindingLocker(db *bun.DB) usersModels.CaregiverBindingLocker {
	return usersRepo.NewCaregiverBindingLocker(NewCaregiverBindingOwners(NewUnobservedTimetableDependencies(db), NewStudentPresenceForTests(db)))
}

// NewRetainedRepositoryFactory is the repository factory over the unobserved
// timetable dependencies.
func NewRetainedRepositoryFactory(db *bun.DB, clocks ...func() time.Time) *Factory {
	return NewFactory(db, NewUnobservedTimetableDependencies(db), clocks...)
}

// NewRetainedPersonRepository is the person repository without the Identity &
// Access account lookup, so FindWithAccount fails closed.
func NewRetainedPersonRepository(db *bun.DB) usersModels.PersonRepository {
	return usersRepo.NewPersonRepository(peopleRuntime(db))
}

// NewRetainedGuardianProfileRepository is the guardian profile repository with
// only the given options bound.
func NewRetainedGuardianProfileRepository(db *bun.DB, options ...RetainedGuardianProfileOption) usersModels.GuardianProfileRepository {
	return usersRepo.NewGuardianProfileRepository(peopleRuntime(db), options...)
}

// NewRetainedGuardianRelationshipRepository is the relationship store with
// only the given options bound.
func NewRetainedGuardianRelationshipRepository(db *bun.DB, options ...RetainedGuardianRelationshipOption) usersModels.StudentGuardianRepository {
	return usersRepo.NewGuardianRelationshipRepository(peopleRuntime(db), options...)
}

// NewRetainedMessageableGuardianRepository is the parent-message recipient
// lookup over the given membership lookup.
func NewRetainedMessageableGuardianRepository(db *bun.DB, memberships RetainedSchoolMembershipLookup) *usersRepo.MessageableGuardianRepository {
	return usersRepo.NewMessageableGuardianRepository(peopleRuntime(db), memberships)
}

// NewRetainedMessageableStaffRepository is the colleague lookup over the given
// owner queries.
func NewRetainedMessageableStaffRepository(db *bun.DB, staffAccounts RetainedStaffAccountsFunc, identity RetainedStaffMessageIdentity) *usersRepo.MessageableStaffRepository {
	return usersRepo.NewMessageableStaffRepository(peopleRuntime(db), staffAccounts, identity)
}

// RetainedWithPortalMemberships binds the guardian profile reachability query.
func RetainedWithPortalMemberships(query RetainedPortalMembershipQuery) RetainedGuardianProfileOption {
	return usersRepo.WithPortalMemberships(query)
}

// RetainedWithGuardianRelationshipMemberships binds the relationship store's
// membership lookup.
func RetainedWithGuardianRelationshipMemberships(query RetainedSchoolMembershipLookup) RetainedGuardianRelationshipOption {
	return usersRepo.WithGuardianRelationshipMemberships(query)
}

// RetainedWithGuardianRelationshipOwners binds the relationship store's Care
// Plan and Identity & Access halves.
func RetainedWithGuardianRelationshipOwners(pickup RetainedGuardianPickupCommand, access RetainedGuardianAccessCommand) RetainedGuardianRelationshipOption {
	return usersRepo.WithGuardianRelationshipOwners(pickup, access)
}

// RetainedWithGuardianDefaultRole binds the authorization policy's default
// role preset, as the composition root does.
func RetainedWithGuardianDefaultRole() RetainedGuardianRelationshipOption {
	return usersRepo.WithGuardianDefaultRole(applyDefaultGuardianRole)
}

// VerifyRetainedStudentSchema runs the startup check of the students table.
func VerifyRetainedStudentSchema(ctx context.Context, db *bun.DB) error {
	return usersRepo.VerifyStudentSchema(ctx, db)
}
