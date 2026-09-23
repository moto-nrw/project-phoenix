package users_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The relationship's unit of work (#2756) writes three owners: People
// Directory's relationship, Care Plan's pickup permission and Identity &
// Access's portal access. These contracts inject a failure after each owner
// command and prove that none of the three halves survives, even when the
// caller handles the error and commits its own transaction, and that the
// retry then writes the link once.

var errInjectedOwnerFailure = errors.New("injected owner failure")

// sqlGuardianOwners writes the two owner halves in the caller's transaction
// and fails on demand. The owners' own commands are tested with their modules;
// here only the unit of work around them is under test.
type sqlGuardianOwners struct {
	db                               *bun.DB
	failPickupCreate, failPickupSet  bool
	failAccessGrant, failAccessPerms bool
}

func (o *sqlGuardianOwners) conn(ctx context.Context) bun.IDB {
	if transaction, ok := tenant.TransactionFromContext(ctx); ok {
		return transaction.(bun.Tx)
	}
	return o.db
}

func (o *sqlGuardianOwners) CreateGuardianPickupPermission(ctx context.Context, tenantID, relationshipID int64, canPickup bool, notes *string) error {
	if o.failPickupCreate {
		return errInjectedOwnerFailure
	}
	_, err := o.conn(ctx).NewRaw(`INSERT INTO users.student_guardian_pickup_permissions (tenant_id, relationship_id, can_pickup, pickup_notes)
		VALUES (?, ?, ?, ?)`, tenantID, relationshipID, canPickup, notes).Exec(ctx)
	return err
}

func (o *sqlGuardianOwners) ChangeGuardianPickupPermission(ctx context.Context, tenantID, relationshipID int64, canPickup *bool, setNotes bool, notes *string) (bool, error) {
	if o.failPickupSet {
		return false, errInjectedOwnerFailure
	}
	result, err := o.conn(ctx).NewRaw(`UPDATE users.student_guardian_pickup_permissions
		SET can_pickup = COALESCE(?, can_pickup), pickup_notes = CASE WHEN ? THEN ? ELSE pickup_notes END
		WHERE tenant_id = ? AND relationship_id = ?`, canPickup, setNotes, notes, tenantID, relationshipID).Exec(ctx)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (o *sqlGuardianOwners) GrantGuardianStudentAccess(ctx context.Context, tenantID, relationshipID int64, accountID *int64, permissions json.RawMessage) error {
	if o.failAccessGrant {
		return errInjectedOwnerFailure
	}
	_, err := o.conn(ctx).NewRaw(`INSERT INTO auth.guardian_student_access (tenant_id, relationship_id, account_id, permissions)
		VALUES (?, ?, ?, ?::jsonb)`, tenantID, relationshipID, accountID, string(permissions)).Exec(ctx)
	return err
}

func (o *sqlGuardianOwners) SetGuardianStudentPermissions(ctx context.Context, tenantID, relationshipID int64, permissions json.RawMessage) (bool, error) {
	if o.failAccessPerms {
		return false, errInjectedOwnerFailure
	}
	result, err := o.conn(ctx).NewRaw(`UPDATE auth.guardian_student_access SET permissions = ?::jsonb
		WHERE tenant_id = ? AND relationship_id = ?`, string(permissions), tenantID, relationshipID).Exec(ctx)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func newRelationshipStore(db *bun.DB, owners *sqlGuardianOwners) users.StudentGuardianRepository {
	return usersRepo.NewGuardianRelationshipRepository(db, usersRepo.WithGuardianRelationshipOwners(owners, owners))
}

// linkHalves counts the rows of each owner for one pair.
func linkHalves(t *testing.T, db *bun.DB, studentID, guardianID int64) (relationships, pickups, accesses int) {
	t.Helper()
	require.NoError(t, db.NewRaw(`SELECT
		(SELECT count(*) FROM users.student_guardian_relationships r WHERE r.student_id = ?0 AND r.guardian_profile_id = ?1),
		(SELECT count(*) FROM users.student_guardian_pickup_permissions p JOIN users.student_guardian_relationships r
			ON r.tenant_id = p.tenant_id AND r.id = p.relationship_id WHERE r.student_id = ?0 AND r.guardian_profile_id = ?1),
		(SELECT count(*) FROM auth.guardian_student_access a JOIN users.student_guardian_relationships r
			ON r.tenant_id = a.tenant_id AND r.id = a.relationship_id WHERE r.student_id = ?0 AND r.guardian_profile_id = ?1)`,
		studentID, guardianID).Scan(context.Background(), &relationships, &pickups, &accesses))
	return relationships, pickups, accesses
}

func newLink(studentID, guardianID int64) *users.StudentGuardian {
	return &users.StudentGuardian{
		StudentID: studentID, GuardianProfileID: guardianID, RelationshipType: "parent",
		GuardianRole: "legal_guardian", IsPrimary: true, CanPickup: true, PickupNotes: textPointer("Mama"),
		Permissions: map[string]interface{}{"parent_portal.access": true},
	}
}

func textPointer(value string) *string { return &value }

func TestGuardianRelationshipLinkRollsBackAfterEveryOwnerCommand(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Unit", "OfWork", "2a")
	guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Unit", "Guardian", "unit-of-work-link")

	for _, owners := range []*sqlGuardianOwners{
		{db: db, failPickupCreate: true},
		{db: db, failAccessGrant: true},
	} {
		// The caller handles the failure and commits its own transaction; the
		// relationship half must not survive with it.
		require.NoError(t, testpkg.WithTenantTx(t, ctx, db, tenantID, func(ctx context.Context, _ testpkg.Tx) error {
			_, err := newRelationshipStore(db, owners).LinkIfNotExists(ctx, newLink(student.ID, guardian.ID))
			require.ErrorIs(t, err, errInjectedOwnerFailure)
			return nil
		}))
		relationships, pickups, accesses := linkHalves(t, db, student.ID, guardian.ID)
		require.Zero(t, relationships+pickups+accesses, "no half of a failed link survives: %+v", owners)
	}

	// Without a caller transaction the unit of work opens its own.
	_, err := newRelationshipStore(db, &sqlGuardianOwners{db: db, failAccessGrant: true}).LinkIfNotExists(ctx, newLink(student.ID, guardian.ID))
	require.ErrorIs(t, err, errInjectedOwnerFailure)
	relationships, pickups, accesses := linkHalves(t, db, student.ID, guardian.ID)
	require.Zero(t, relationships+pickups+accesses)

	// The retry writes the link once; a second retry is a no-op.
	store := newRelationshipStore(db, &sqlGuardianOwners{db: db})
	for attempt, want := range []bool{true, false} {
		inserted, err := store.LinkIfNotExists(ctx, newLink(student.ID, guardian.ID))
		require.NoError(t, err)
		require.Equal(t, want, inserted, "attempt %d", attempt)
		relationships, pickups, accesses = linkHalves(t, db, student.ID, guardian.ID)
		require.Equal(t, []int{1, 1, 1}, []int{relationships, pickups, accesses})
	}
	links, err := store.FindByStudentID(ctx, student.ID)
	require.NoError(t, err)
	require.Len(t, links, 1)
	require.True(t, links[0].CanPickup)
	require.Equal(t, "Mama", *links[0].PickupNotes)
	require.Equal(t, true, links[0].Permissions["parent_portal.access"])
}

func TestGuardianRelationshipUpdateRollsBackAfterEveryOwnerCommand(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Update", "OfWork", "2a")
	guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Update", "Guardian", "unit-of-work-update")
	store := newRelationshipStore(db, &sqlGuardianOwners{db: db})
	link := newLink(student.ID, guardian.ID)
	require.NoError(t, store.Create(ctx, link))
	before, err := store.FindByID(ctx, link.ID)
	require.NoError(t, err)

	for _, owners := range []*sqlGuardianOwners{
		{db: db, failPickupSet: true},
		{db: db, failAccessPerms: true},
	} {
		changed := *before
		changed.RelationshipType = "relative"
		changed.IsEmergencyContact = true
		changed.CanPickup = false
		changed.PickupNotes = nil
		changed.Permissions = map[string]interface{}{"parent_portal.access": false}
		require.NoError(t, testpkg.WithTenantTx(t, ctx, db, tenantID, func(ctx context.Context, _ testpkg.Tx) error {
			require.ErrorIs(t, newRelationshipStore(db, owners).Update(ctx, &changed), errInjectedOwnerFailure)
			return nil
		}))
		after, err := store.FindByID(ctx, link.ID)
		require.NoError(t, err)
		require.Equal(t, before.RelationshipType, after.RelationshipType, "%+v", owners)
		require.Equal(t, before.IsEmergencyContact, after.IsEmergencyContact)
		require.Equal(t, before.CanPickup, after.CanPickup)
		require.Equal(t, before.PickupNotes, after.PickupNotes)
		require.Equal(t, before.Permissions, after.Permissions)
	}

	changed := *before
	changed.CanPickup = false
	changed.Permissions = map[string]interface{}{"parent_portal.access": false}
	require.NoError(t, store.Update(ctx, &changed))
	after, err := store.FindByID(ctx, link.ID)
	require.NoError(t, err)
	require.False(t, after.CanPickup)
	require.Equal(t, false, after.Permissions["parent_portal.access"])
}

func TestGuardianRelationshipPromotionDemotesThePreviousPrimary(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Primary", "Child", "2a")
	first := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Primary", "First", "unit-of-work-first")
	second := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Primary", "Second", "unit-of-work-second")
	store := newRelationshipStore(db, &sqlGuardianOwners{db: db})
	firstLink := newLink(student.ID, first.ID)
	require.NoError(t, store.Create(ctx, firstLink))
	secondLink := newLink(student.ID, second.ID)
	secondLink.IsPrimary = false
	require.NoError(t, store.Create(ctx, secondLink))

	secondLink.IsPrimary = true
	updated, err := store.UpdateColumns(ctx, secondLink, "is_primary")
	require.NoError(t, err)
	require.EqualValues(t, 1, updated)
	primaries, err := store.List(ctx, map[string]any{"student_id": student.ID, "is_primary": true})
	require.NoError(t, err)
	require.Len(t, primaries, 1)
	require.Equal(t, secondLink.ID, primaries[0].ID)

	require.NoError(t, store.SetPrimary(ctx, firstLink.ID, true))
	primaries, err = store.List(ctx, map[string]any{"student_id": student.ID, "is_primary": true})
	require.NoError(t, err)
	require.Len(t, primaries, 1)
	require.Equal(t, firstLink.ID, primaries[0].ID)
}

// A re-link naming a primary must not touch the child's other relationships:
// the insert writes nothing, so the demotion must not run either. The old
// table's BEFORE INSERT trigger demoted before the conflict check and could
// leave the child without a primary.
func TestGuardianRelationshipRelinkLeavesThePrimaryAlone(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Relink", "Child", "2a")
	primary := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Relink", "Primary", "unit-of-work-relink-primary")
	other := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Relink", "Other", "unit-of-work-relink-other")
	store := newRelationshipStore(db, &sqlGuardianOwners{db: db})
	primaryLink := newLink(student.ID, primary.ID)
	require.NoError(t, store.Create(ctx, primaryLink))
	otherLink := newLink(student.ID, other.ID)
	otherLink.IsPrimary = false
	require.NoError(t, store.Create(ctx, otherLink))

	// Re-linking the primary guardian itself.
	inserted, err := store.LinkIfNotExists(ctx, newLink(student.ID, primary.ID))
	require.NoError(t, err)
	require.False(t, inserted)
	primaries, err := store.List(ctx, map[string]any{"student_id": student.ID, "is_primary": true})
	require.NoError(t, err)
	require.Len(t, primaries, 1, "the child keeps its primary guardian")
	require.Equal(t, primaryLink.ID, primaries[0].ID)

	// Re-linking the other guardian as primary leaves the stored flags alone.
	relink := newLink(student.ID, other.ID)
	relink.IsPrimary = true
	inserted, err = store.LinkIfNotExists(ctx, relink)
	require.NoError(t, err)
	require.False(t, inserted)
	primaries, err = store.List(ctx, map[string]any{"student_id": student.ID, "is_primary": true})
	require.NoError(t, err)
	require.Len(t, primaries, 1)
	require.Equal(t, primaryLink.ID, primaries[0].ID)

	// A new pair still demotes: the insert writes a row.
	third := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Relink", "Third", "unit-of-work-relink-third")
	newPair := newLink(student.ID, third.ID)
	inserted, err = store.LinkIfNotExists(ctx, newPair)
	require.NoError(t, err)
	require.True(t, inserted)
	primaries, err = store.List(ctx, map[string]any{"student_id": student.ID, "is_primary": true})
	require.NoError(t, err)
	require.Len(t, primaries, 1)
	require.Equal(t, newPair.ID, primaries[0].ID)
}

func TestGuardianRelationshipStoreStaysInsideTheTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Tenant", "Child", "2a")
	guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Tenant", "Guardian", "unit-of-work-tenant")
	store := newRelationshipStore(db, &sqlGuardianOwners{db: db})
	link := newLink(student.ID, guardian.ID)
	require.NoError(t, store.Create(ctx, link))

	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	otherCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), otherTenantID)
	require.NoError(t, testpkg.WithTenantTx(t, otherCtx, db, otherTenantID, func(ctx context.Context, _ testpkg.Tx) error {
		links, err := store.FindByStudentID(ctx, student.ID)
		require.NoError(t, err)
		require.Empty(t, links, "another school sees none of the three halves")
		foreign := *link
		foreign.RelationshipType = "other"
		updated, err := store.UpdateColumns(ctx, &foreign, "relationship_type", "can_pickup", "permissions")
		require.NoError(t, err)
		require.Zero(t, updated)
		require.NoError(t, store.Delete(ctx, link.ID))
		return nil
	}))
	links, err := store.FindByStudentID(ctx, student.ID)
	require.NoError(t, err)
	require.Len(t, links, 1, "the other school's delete matched nothing")
	require.Equal(t, "parent", links[0].RelationshipType)
	require.True(t, links[0].CanPickup)
}
