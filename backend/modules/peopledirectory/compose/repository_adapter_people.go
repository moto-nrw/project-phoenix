package compose

import (
	"context"

	usersRepo "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/adapters/peoplepostgres"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/uptrace/bun"
)

// The retained People Directory repositories (database/repositories/users) are
// built here over the owner's legacy runtime (#2743). What other owners hold —
// the account behind a person, the portal memberships of a guardian, the Care
// Plan and Identity & Access halves of a guardian link, the binding owners'
// table locks — arrives through the ports these constructors take; the
// composition root binds them over the owners' public capabilities.

// AccountLookup resolves the account a person is linked to; a missing account
// is nil, as the former LEFT JOIN was.
type AccountLookup = usersRepo.AccountLookup

// SchoolMembershipLookup maps the requested accounts to the schools in which
// they hold an active membership, bounded to the given schools.
type SchoolMembershipLookup = usersRepo.SchoolMembershipLookup

// GuardianPickupPermissionCommand is the Care Plan half of a guardian link.
type GuardianPickupPermissionCommand = usersRepo.GuardianPickupPermissionCommand

// GuardianStudentAccessCommand is the Identity & Access half of a guardian link.
type GuardianStudentAccessCommand = usersRepo.GuardianStudentAccessCommand

// CaregiverBindingOwners are the four owners whose tables can retain caregiver
// capability; each takes its own table lock.
type CaregiverBindingOwners = usersRepo.CaregiverBindingOwners

// NewPersonRepository is the person repository with the account lookup
// FindWithAccount attaches (#2720).
func NewPersonRepository(db *bun.DB, accounts AccountLookup) peoplePorts.PersonRepository {
	return usersRepo.NewPersonRepository(NewLegacyRepositoryRuntime(db), usersRepo.WithAccountLookup(accounts))
}

// NewGuardianProfileRepository is the guardian profile repository with the
// portal reachability queries its check joins: the schools in which an account
// holds an active guardian membership, and those in which it can also log in
// with a password (#2720, #2721).
func NewGuardianProfileRepository(db *bun.DB, students usersRepo.StudentDirectoryRows, links usersRepo.GuardianLinkRows, portal, loginReady func(context.Context, []int64) (map[int64][]int64, error)) peoplePorts.GuardianProfileRepository {
	return application.NewGuardianProfileCommands(usersRepo.NewGuardianProfileRepository(NewLegacyRepositoryRuntime(db), students, links,
		usersRepo.WithPortalMemberships(portal),
		usersRepo.WithPortalLoginMemberships(loginReady),
	), NewLegacyRepositoryRuntime(db))
}

// GuardianRelationshipDependencies are the owners the student-guardian
// relationship store reaches since Cutover #2756: People Directory writes the
// relationship and runs the unit of work, Care Plan's pickup permission and
// Identity & Access's portal access are written through the commands.
type GuardianRelationshipDependencies struct {
	Students    usersRepo.StudentDirectoryRows
	Links       usersRepo.GuardianLinkRows
	Memberships SchoolMembershipLookup
	Pickup      GuardianPickupPermissionCommand
	Access      GuardianStudentAccessCommand
	// RolePresets is the authorization policy whose default preset a link
	// without a role or permissions takes; nil leaves such a link as it is.
	RolePresets peoplePorts.GuardianRolePresets
}

// NewGuardianRelationshipRepository is the relationship store with its
// membership lookup, both owner halves and the default role bound.
func NewGuardianRelationshipRepository(db *bun.DB, dependencies GuardianRelationshipDependencies) peoplePorts.StudentGuardianRepository {
	return application.NewGuardianRelationshipCommands(usersRepo.NewGuardianRelationshipRepository(NewLegacyRepositoryRuntime(db), dependencies.Students, dependencies.Links,
		usersRepo.WithGuardianRelationshipMemberships(dependencies.Memberships),
		usersRepo.WithGuardianRelationshipOwners(dependencies.Pickup, dependencies.Access),
		usersRepo.WithGuardianDefaultRole(application.DefaultGuardianRole(dependencies.RolePresets))), NewLegacyRepositoryRuntime(db))
}

// NewGuardianPhoneNumberRepository is the guardian phone number repository.
func NewGuardianPhoneNumberRepository(db *bun.DB) peoplePorts.GuardianPhoneNumberRepository {
	return application.NewGuardianPhoneCommands(usersRepo.NewGuardianPhoneNumberRepository(NewLegacyRepositoryRuntime(db)), NewLegacyRepositoryRuntime(db))
}

// NewFamilyProtectionEventRepository is the family protection event repository.
func NewFamilyProtectionEventRepository(db *bun.DB) peoplePorts.FamilyProtectionRecords {
	return usersRepo.NewFamilyProtectionEventRepository(NewLegacyRepositoryRuntime(db))
}

// NewParentRequestShareEventRepository is the parent request share event
// repository.
func NewParentRequestShareEventRepository(db *bun.DB) peoplePorts.ParentRequestShareRecords {
	return usersRepo.NewParentRequestShareEventRepository(NewLegacyRepositoryRuntime(db))
}

// NewParentRequestEventRepository is the parent request event repository.
func NewParentRequestEventRepository(db *bun.DB) peoplePorts.ParentRequestEventRecords {
	return usersRepo.NewParentRequestEventRepository(NewLegacyRepositoryRuntime(db))
}

// NewGuardianFinancialDataRepository is the guardian payment data repository
// (#2608).
func NewGuardianFinancialDataRepository(db *bun.DB) peoplePorts.GuardianFinancialDataRepository {
	return usersRepo.NewGuardianFinancialDataRepository(NewLegacyRepositoryRuntime(db))
}

// NewCaregiverBindingLocker serializes the caregiver capability re-check over
// the four binding owners' table locks.
func NewCaregiverBindingLocker(owners CaregiverBindingOwners) peoplePorts.CaregiverBindingLocker {
	return usersRepo.NewCaregiverBindingLocker(owners)
}
