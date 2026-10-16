package compose

import (
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// The retained People Directory repository suites (database/repositories/users)
// may not import that package (#2727). They compose the retained repositories
// over the tenant runtime through the test support that forwards these
// (services, api/testutil). Every entry goes with the last suite that uses it.

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

// NewRetainedPersonRepository is the person repository without the Identity &
// Access account lookup, so FindWithAccount fails closed.
func NewRetainedPersonRepository(db *bun.DB) usersRepo.PersonRepository {
	return usersRepo.NewPersonRepository(NewLegacyRepositoryRuntime(db))
}

// NewRetainedGuardianProfileRepository is the guardian profile repository with
// only the given options bound.
func NewRetainedGuardianProfileRepository(db *bun.DB, options ...RetainedGuardianProfileOption) usersRepo.GuardianProfileRepository {
	return usersRepo.NewGuardianProfileRepository(NewLegacyRepositoryRuntime(db), options...)
}

// NewRetainedGuardianRelationshipRepository is the relationship store with
// only the given options bound.
func NewRetainedGuardianRelationshipRepository(db *bun.DB, options ...RetainedGuardianRelationshipOption) usersRepo.StudentGuardianRepository {
	return usersRepo.NewGuardianRelationshipRepository(NewLegacyRepositoryRuntime(db), options...)
}

// NewRetainedMessageableGuardianRepository is the parent-message recipient
// lookup over the given membership lookup.
func NewRetainedMessageableGuardianRepository(db *bun.DB, memberships RetainedSchoolMembershipLookup) *usersRepo.MessageableGuardianRepository {
	return usersRepo.NewMessageableGuardianRepository(NewLegacyRepositoryRuntime(db), memberships)
}

// NewRetainedMessageableStaffRepository is the colleague lookup over the given
// owner queries.
func NewRetainedMessageableStaffRepository(db *bun.DB, staffAccounts RetainedStaffAccountsFunc, identity RetainedStaffMessageIdentity) *usersRepo.MessageableStaffRepository {
	return usersRepo.NewMessageableStaffRepository(NewLegacyRepositoryRuntime(db), staffAccounts, identity)
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

// RetainedWithGuardianDefaultRole binds the given default role preset, as the
// composition root binds the authorization policy's.
func RetainedWithGuardianDefaultRole(apply func(*usersModels.StudentGuardian)) RetainedGuardianRelationshipOption {
	return usersRepo.WithGuardianDefaultRole(apply)
}
