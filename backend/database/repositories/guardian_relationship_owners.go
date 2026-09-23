package repositories

import (
	"context"
	"encoding/json"
	"fmt"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/uptrace/bun"
)

// The student-guardian relationship has three owners since Cutover #2756.
// People Directory writes the relationship and runs the unit of work; Care
// Plan's pickup permission and Identity & Access's portal access reach it
// through the consumer-owned ports bound here. Both halves are constructible
// from the database alone, so every graph that builds the relationship store
// also gets its owners.

// newGuardianRelationshipOwners composes the Care Plan and Identity & Access
// commands behind the relationship's unit of work.
func newGuardianRelationshipOwners(db *bun.DB, observeIdentity IdentityAccessObserver) (guardianPickupPort, guardianAccessPort) {
	pickup, err := careplanCompose.NewGuardianPickupPermissions(db, func(careplanCompose.Observation) {})
	if err != nil {
		panic(fmt.Sprintf("repository factory: compose guardian pickup permissions: %v", err))
	}
	identityObserver := func(identityCompose.Observation) {}
	if observeIdentity != nil {
		identityObserver = func(observation identityCompose.Observation) {
			observeIdentity(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows,
				observation.Stats.StatementDuration, identityaccess.ErrorCode(observation.Err), observation.Err)
		}
	}
	access, err := identityCompose.NewGuardianStudentAccess(db, identityObserver)
	if err != nil {
		panic(fmt.Sprintf("repository factory: compose guardian student access: %v", err))
	}
	return guardianPickupPort{owner: pickup}, guardianAccessPort{owner: access}
}

// newGuardianRelationships is the relationship store with its membership
// lookup and both owner halves bound.
func newGuardianRelationships(db *bun.DB, memberships usersRepo.SchoolMembershipLookup, observeIdentity IdentityAccessObserver) usersModels.StudentGuardianRepository {
	pickup, access := newGuardianRelationshipOwners(db, observeIdentity)
	return usersRepo.NewGuardianRelationshipRepository(db,
		usersRepo.WithGuardianRelationshipMemberships(memberships),
		usersRepo.WithGuardianRelationshipOwners(pickup, access))
}

type guardianPickupPort struct {
	owner careplan.GuardianPickupPermissions
}

func (p guardianPickupPort) CreateGuardianPickupPermission(ctx context.Context, tenantID, relationshipID int64, canPickup bool, notes *string) error {
	return p.owner.CreateGuardianPickupPermission(ctx, careplan.GuardianPickupPermission{
		TenantID: tenantID, RelationshipID: relationshipID, CanPickup: canPickup, PickupNotes: notes,
	})
}

func (p guardianPickupPort) ChangeGuardianPickupPermission(ctx context.Context, tenantID, relationshipID int64, canPickup *bool, setNotes bool, notes *string) (bool, error) {
	return p.owner.ChangeGuardianPickupPermission(ctx, careplan.GuardianPickupPermissionChange{
		TenantID: tenantID, RelationshipID: relationshipID, CanPickup: canPickup, SetPickupNotes: setNotes, PickupNotes: notes,
	})
}

type guardianAccessPort struct {
	owner identityaccess.GuardianStudentAccess
}

func (p guardianAccessPort) GrantGuardianStudentAccess(ctx context.Context, tenantID, relationshipID int64, accountID *int64, permissions json.RawMessage) error {
	return p.owner.GrantGuardianStudentAccess(ctx, identityaccess.GuardianStudentAccessGrant{
		TenantID: tenantID, RelationshipID: relationshipID, AccountID: accountID, Permissions: permissions,
	})
}

func (p guardianAccessPort) SetGuardianStudentPermissions(ctx context.Context, tenantID, relationshipID int64, permissions json.RawMessage) (bool, error) {
	return p.owner.SetGuardianStudentPermissions(ctx, tenantID, relationshipID, permissions)
}

// guardianLinkOwners serves People Directory's parents-portal link writes the
// same two owner halves the relationship store writes.
type guardianLinkOwners struct {
	guardianPickupPort
	guardianAccessPort
}

func newGuardianLinkOwners(db *bun.DB) guardianLinkOwners {
	pickup, access := newGuardianRelationshipOwners(db, nil)
	return guardianLinkOwners{guardianPickupPort: pickup, guardianAccessPort: access}
}
