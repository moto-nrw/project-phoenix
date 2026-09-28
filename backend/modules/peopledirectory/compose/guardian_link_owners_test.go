package compose

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

var errInjectedLinkOwnerFailure = errors.New("injected link owner failure")

// failingGuardianLinkOwners fails one owner command after the others wrote.
type failingGuardianLinkOwners struct {
	testGuardianLinkOwners
	failPickupCreate, failPickupChange, failAccessGrant bool
}

func (o failingGuardianLinkOwners) CreateGuardianPickupPermission(ctx context.Context, tenantID, relationshipID int64, canPickup bool, notes *string) error {
	if o.failPickupCreate {
		return errInjectedLinkOwnerFailure
	}
	return o.testGuardianLinkOwners.CreateGuardianPickupPermission(ctx, tenantID, relationshipID, canPickup, notes)
}

func (o failingGuardianLinkOwners) ChangeGuardianPickupPermission(ctx context.Context, tenantID, relationshipID int64, canPickup *bool, setNotes bool, notes *string) (bool, error) {
	if o.failPickupChange {
		return false, errInjectedLinkOwnerFailure
	}
	return o.testGuardianLinkOwners.ChangeGuardianPickupPermission(ctx, tenantID, relationshipID, canPickup, setNotes, notes)
}

func (o failingGuardianLinkOwners) GrantGuardianStudentAccess(ctx context.Context, tenantID, relationshipID int64, accountID *int64, permissions json.RawMessage) error {
	if o.failAccessGrant {
		return errInjectedLinkOwnerFailure
	}
	return o.testGuardianLinkOwners.GrantGuardianStudentAccess(ctx, tenantID, relationshipID, accountID, permissions)
}

func buildModuleWithLinkOwners(t *testing.T, db *bun.DB, owners GuardianLinkOwners) *peopledirectory.Module {
	t.Helper()
	module, err := NewWithGuardianMemberships(Dependencies{DB: db, Observe: func(Observation) {}, GuardianLinkOwners: owners}, testPortalMemberships(db))
	require.NoError(t, err)
	return module
}

func countLinkHalves(t *testing.T, db *bun.DB, studentID, guardianID int64) int {
	t.Helper()
	var count int
	require.NoError(t, db.NewRaw(`SELECT
		(SELECT count(*) FROM users.student_guardian_relationships r WHERE r.student_id = ?0 AND r.guardian_profile_id = ?1)
		+ (SELECT count(*) FROM users.student_guardian_pickup_permissions p JOIN users.student_guardian_relationships r
			ON r.tenant_id = p.tenant_id AND r.id = p.relationship_id WHERE r.student_id = ?0 AND r.guardian_profile_id = ?1)
		+ (SELECT count(*) FROM auth.guardian_student_access a JOIN users.student_guardian_relationships r
			ON r.tenant_id = a.tenant_id AND r.id = a.relationship_id WHERE r.student_id = ?0 AND r.guardian_profile_id = ?1)`,
		studentID, guardianID).Scan(context.Background(), &count))
	return count
}

// A portal link is written as its relationship, its pickup permission and its
// portal access, or not at all (#2756): a failing owner command undoes the
// relationship even when the caller handles the error and commits, and the
// retry writes the link once.
func TestGuardianPortalLinkRollsBackAfterEveryOwnerCommand(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Owner", "Failure", "1a")
	guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Owner", "Failure", "portal-owner-failure")
	link := peopledirectory.GuardianContactLink{
		StudentID: student.ID, GuardianProfileID: guardian.ID, RelationshipType: "parent", CanPickup: true,
		Permissions: json.RawMessage(`{"parent_portal.access": true}`),
	}
	healthy := testGuardianLinkOwners{db: db}
	for _, owners := range []failingGuardianLinkOwners{
		{testGuardianLinkOwners: healthy, failPickupCreate: true},
		{testGuardianLinkOwners: healthy, failAccessGrant: true},
	} {
		module := buildModuleWithLinkOwners(t, db, owners)
		require.NoError(t, testpkg.WithTenantTx(t, ctx, db, tenantID, func(ctx context.Context, _ testpkg.Tx) error {
			linkID, inserted, err := module.LinkGuardianContact(ctx, link)
			require.ErrorIs(t, err, errInjectedLinkOwnerFailure)
			require.Zero(t, linkID)
			require.False(t, inserted)
			return nil
		}))
		require.Zero(t, countLinkHalves(t, db, student.ID, guardian.ID), "no half of a failed link survives: %+v", owners)
	}

	module := buildModuleWithLinkOwners(t, db, healthy)
	linkID, inserted, err := module.LinkGuardianContact(ctx, link)
	require.NoError(t, err)
	require.True(t, inserted)
	require.Equal(t, 3, countLinkHalves(t, db, student.ID, guardian.ID))
	_, inserted, err = module.LinkGuardianContact(ctx, link)
	require.NoError(t, err)
	require.False(t, inserted)
	require.Equal(t, 3, countLinkHalves(t, db, student.ID, guardian.ID))

	// A pickup patch whose Care Plan half fails leaves the emergency flag of
	// the relationship untouched too.
	failing := buildModuleWithLinkOwners(t, db, failingGuardianLinkOwners{testGuardianLinkOwners: healthy, failPickupChange: true})
	_, err = failing.PatchGuardianLinkPickup(ctx, peopledirectory.GuardianLinkPickupPatch{
		LinkID: linkID, IsEmergencyContact: flagPtr(true), CanPickup: flagPtr(false),
	})
	require.ErrorIs(t, err, errInjectedLinkOwnerFailure)
	stored := readGuardianLink(t, db, student.ID, guardian.ID)
	require.False(t, stored.IsEmergencyContact)
	require.True(t, stored.CanPickup)
}

// A parents-portal re-link naming a primary writes no relationship, so it must
// not demote the child's primary either. The old table's BEFORE INSERT trigger
// demoted before the conflict check and could leave the child without one.
func TestGuardianPortalRelinkLeavesThePrimaryAlone(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Portal", "Relink", "1a")
	primary := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Portal", "Primary", "portal-relink-primary")
	other := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Portal", "Other", "portal-relink-other")
	module := buildModuleWithLinkOwners(t, db, testGuardianLinkOwners{db: db})

	primaryLink := peopledirectory.GuardianContactLink{
		StudentID: student.ID, GuardianProfileID: primary.ID, RelationshipType: "parent", IsPrimary: true,
	}
	_, inserted, err := module.LinkGuardianContact(ctx, primaryLink)
	require.NoError(t, err)
	require.True(t, inserted)
	otherLink := peopledirectory.GuardianContactLink{
		StudentID: student.ID, GuardianProfileID: other.ID, RelationshipType: "parent",
	}
	_, inserted, err = module.LinkGuardianContact(ctx, otherLink)
	require.NoError(t, err)
	require.True(t, inserted)

	// Re-linking either pair as primary changes nothing.
	for _, relink := range []peopledirectory.GuardianContactLink{primaryLink, {
		StudentID: student.ID, GuardianProfileID: other.ID, RelationshipType: "parent", IsPrimary: true,
	}} {
		_, inserted, err = module.LinkGuardianContact(ctx, relink)
		require.NoError(t, err)
		require.False(t, inserted)
		require.True(t, readGuardianLink(t, db, student.ID, primary.ID).IsPrimary, "the child keeps its primary guardian")
		require.False(t, readGuardianLink(t, db, student.ID, other.ID).IsPrimary)
	}

	// A new pair still demotes: the insert writes a row.
	third := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Portal", "Third", "portal-relink-third")
	_, inserted, err = module.LinkGuardianContact(ctx, peopledirectory.GuardianContactLink{
		StudentID: student.ID, GuardianProfileID: third.ID, RelationshipType: "parent", IsPrimary: true,
	})
	require.NoError(t, err)
	require.True(t, inserted)
	require.False(t, readGuardianLink(t, db, student.ID, primary.ID).IsPrimary)
	require.True(t, readGuardianLink(t, db, student.ID, third.ID).IsPrimary)
}
