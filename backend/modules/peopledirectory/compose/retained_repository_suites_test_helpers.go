//go:build testhooks

package compose

import (
	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	usersRepo "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/adapters/peoplepostgres"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/uptrace/bun"
)

// Retained People Directory repository suites compose through root test hooks
// because they may not import database/repositories/users (#2727).

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
func NewRetainedPersonRepository(db *bun.DB) peoplePorts.PersonRepository {
	return usersRepo.NewPersonRepository(NewLegacyRepositoryRuntime(db))
}

// NewRetainedGuardianProfileRepository is the guardian profile repository with
// only the given options bound.
func NewRetainedGuardianProfileRepository(db *bun.DB, students usersRepo.StudentDirectoryRows, links usersRepo.GuardianLinkRows, options ...RetainedGuardianProfileOption) peoplePorts.GuardianProfileRepository {
	return application.NewGuardianProfileCommands(usersRepo.NewGuardianProfileRepository(NewLegacyRepositoryRuntime(db), students, links, options...), NewLegacyRepositoryRuntime(db))
}

// NewRetainedGuardianRelationshipRepository is the relationship store with
// only the given options bound.
func NewRetainedGuardianRelationshipRepository(db *bun.DB, students usersRepo.StudentDirectoryRows, links usersRepo.GuardianLinkRows, options ...RetainedGuardianRelationshipOption) peoplePorts.StudentGuardianRepository {
	return application.NewGuardianRelationshipCommands(usersRepo.NewGuardianRelationshipRepository(NewLegacyRepositoryRuntime(db), students, links, options...), NewLegacyRepositoryRuntime(db))
}

// NewRetainedMessageableGuardianRepository is the parent-message recipient
// lookup over the given membership lookup.
func NewRetainedMessageableGuardianRepository(db *bun.DB, links usersRepo.GuardianLinkRows, memberships RetainedSchoolMembershipLookup) *usersRepo.MessageableGuardianRepository {
	return usersRepo.NewMessageableGuardianRepository(NewLegacyRepositoryRuntime(db), links, memberships)
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
func RetainedWithGuardianDefaultRole(apply func(*peopleValues.GuardianRelationshipSnapshot)) RetainedGuardianRelationshipOption {
	return usersRepo.WithGuardianDefaultRole(apply)
}

// DisabledStudentPhotos is the photo runtime of a root that serves no
// photos: every photo route reports the feature as disabled and a consent
// change fails.
func DisabledStudentPhotos() StudentPhotoRuntime { return disabledStudentPhotos{} }
