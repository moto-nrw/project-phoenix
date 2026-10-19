package compose

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	ports "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// lifecyclePresets stands in for the Security Runtime policy: parents default
// to the legal guardian preset, everything else to custom, and only the legal
// guardian preset grants permissions.
func lifecyclePresets() ports.GuardianRolePresetFuncs {
	return ports.GuardianRolePresetFuncs{
		Default: func(relationshipType string, _, _, _ bool) string {
			if relationshipType == "Parent" || relationshipType == "parent" {
				return "legal_guardian"
			}
			return "custom"
		},
		Presets: func(role string) (string, []string) {
			if role == "legal_guardian" {
				return role, []string{"parent_portal.access", "parent_portal.notes.write"}
			}
			return role, nil
		},
	}
}

func buildLifecycleModule(t *testing.T, db *bun.DB) *peopledirectory.Module {
	t.Helper()
	module, err := NewWithGuardianMemberships(Dependencies{
		StudentDirectory: testpkg.StudentDirectory{}, DB: db, Observe: func(Observation) {},
		GuardianLinkOwners: testGuardianLinkOwners{db: db}, GuardianLinkRows: testpkg.GuardianLinks{},
		GuardianRolePresets: lifecyclePresets(),
	}, testPortalMemberships(db))
	require.NoError(t, err)
	return module
}

func legalGuardianPermissions() map[string]any {
	return map[string]any{"parent_portal.access": true, "parent_portal.notes.write": true}
}

func TestLinkInvitedGuardianWritesTheDefaultPresetOnce(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildLifecycleModule(t, db)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Invited", "Child", "1a")
	guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Invited", "Parent", "lifecycle-invited")
	account := testpkg.CreateTestAccount(t, db, "lifecycle-invited-account")
	require.NoError(t, module.LinkGuardianProfileToAccount(ctx, guardian.ID, account.ID))
	_, err := db.NewRaw(`UPDATE users.guardian_profiles SET childless_since = NOW() - INTERVAL '120 days' WHERE id = ?`, guardian.ID).Exec(ctx)
	require.NoError(t, err)

	created, err := module.LinkInvitedGuardian(ctx, peopledirectory.InvitedGuardianLink{
		StudentID: student.ID, GuardianProfileID: guardian.ID, RelationshipType: "Parent",
	})
	require.NoError(t, err)
	require.True(t, created)
	assert.Nil(t, childlessSince(t, db, guardian.ID), "a new child link starts a future childless period anew")
	require.Equal(t, 3, countLinkHalves(t, db, student.ID, guardian.ID))
	stored := readGuardianLink(t, db, student.ID, guardian.ID)
	assert.Equal(t, tenantID, stored.TenantID)
	assert.Equal(t, "parent", stored.RelationshipType)
	assert.Equal(t, "legal_guardian", stored.GuardianRole, "the default preset of the type as the invitation named it")
	assert.Equal(t, legalGuardianPermissions(), stored.Permissions)
	assert.Equal(t, 1, stored.EmergencyPriority)
	assert.False(t, stored.IsPrimary)
	assert.False(t, stored.IsEmergencyContact)
	assert.False(t, stored.CanPickup)
	assert.False(t, stored.IsPayer)
	var accessAccount *int64
	require.NoError(t, db.NewRaw(`SELECT account_id FROM auth.guardian_student_access WHERE tenant_id = ? AND relationship_id = ?`,
		tenantID, stored.ID).Scan(context.Background(), &accessAccount))
	require.NotNil(t, accessAccount, "the portal access binds the guardian's account")
	assert.Equal(t, account.ID, *accessAccount)

	_, err = db.NewRaw(`UPDATE users.student_guardian_relationships SET guardian_role = 'pickup_only' WHERE id = ?`, stored.ID).
		Exec(context.Background())
	require.NoError(t, err)
	created, err = module.LinkInvitedGuardian(ctx, peopledirectory.InvitedGuardianLink{
		StudentID: student.ID, GuardianProfileID: guardian.ID, RelationshipType: "parent", EmergencyPriority: 1,
	})
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, 3, countLinkHalves(t, db, student.ID, guardian.ID))
	assert.Equal(t, "pickup_only", readGuardianLink(t, db, student.ID, guardian.ID).GuardianRole, "an existing link stays untouched")
}

func TestLinkGuardianProfileToAccountRefusesAnErasedAccount(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildLifecycleModule(t, db)
	ctx := testpkg.Ctx(t)
	guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, testpkg.Tenant(t), "Erased", "Account", "lifecycle-erased-account")
	account := testpkg.CreateTestAccount(t, db, "lifecycle-erased-account")
	_, err := db.NewRaw(`UPDATE auth.accounts SET personal_data_erased_at = NOW() WHERE id = ?`, account.ID).Exec(ctx)
	require.NoError(t, err)

	require.ErrorIs(t, module.LinkGuardianProfileToAccount(ctx, guardian.ID, account.ID), peopledirectory.ErrGuardianAccountConflict)
	assert.Nil(t, readGuardianProfile(t, db, guardian.ID).AccountID)
}

func TestLinkInvitedGuardianReportsADeletedProfile(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildLifecycleModule(t, db)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudentForTenant(t, db, testpkg.Tenant(t), "Deleted", "Profile", "1a")
	guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, testpkg.Tenant(t), "Deleted", "Profile", "lifecycle-deleted-profile")
	_, err := db.NewRaw(`DELETE FROM users.guardian_profiles WHERE id = ?`, guardian.ID).Exec(ctx)
	require.NoError(t, err)

	_, err = module.LinkInvitedGuardian(ctx, peopledirectory.InvitedGuardianLink{
		StudentID: student.ID, GuardianProfileID: guardian.ID, RelationshipType: "parent",
	})
	require.ErrorIs(t, err, peopledirectory.ErrGuardianNotFound)
}

func TestGuardianLifecycleRoleWritesRefuseWithoutPresets(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Unbound", "Child", "1a")
	guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Unbound", "Parent", "lifecycle-unbound")

	_, err := module.LinkInvitedGuardian(ctx, peopledirectory.InvitedGuardianLink{
		StudentID: student.ID, GuardianProfileID: guardian.ID, RelationshipType: "parent",
	})
	require.Error(t, err)
	assert.Zero(t, countLinkHalves(t, db, student.ID, guardian.ID))

	linkID, _, err := module.LinkGuardianContact(ctx, peopledirectory.GuardianContactLink{
		StudentID: student.ID, GuardianProfileID: guardian.ID, RelationshipType: "relative", GuardianRole: "pickup_only",
	})
	require.NoError(t, err)
	require.Error(t, module.PromoteGuardianLinkToLegalGuardian(ctx, linkID))
	assert.Equal(t, "pickup_only", readGuardianLink(t, db, student.ID, guardian.ID).GuardianRole)
}

func TestPromoteGuardianLinkToLegalGuardianWritesOnlyRoleAndPermissions(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildLifecycleModule(t, db)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Promoted", "Child", "1a")
	guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Promoted", "Contact", "lifecycle-promoted")
	linkID, _, err := module.LinkGuardianContact(ctx, peopledirectory.GuardianContactLink{
		StudentID: student.ID, GuardianProfileID: guardian.ID, RelationshipType: "relative", GuardianRole: "pickup_only",
		IsEmergencyContact: true, CanPickup: true, PickupNotes: textPtr("nur dienstags"), EmergencyPriority: 2,
		Permissions: json.RawMessage(`{"parent_portal.sick_note.submit": true}`),
	})
	require.NoError(t, err)

	require.NoError(t, module.PromoteGuardianLinkToLegalGuardian(ctx, linkID))
	stored := readGuardianLink(t, db, student.ID, guardian.ID)
	assert.Equal(t, "legal_guardian", stored.GuardianRole)
	assert.Equal(t, legalGuardianPermissions(), stored.Permissions, "exactly the preset's permissions")
	assert.Equal(t, "relative", stored.RelationshipType)
	assert.True(t, stored.IsEmergencyContact)
	assert.True(t, stored.CanPickup)
	require.NotNil(t, stored.PickupNotes)
	assert.Equal(t, "nur dienstags", *stored.PickupNotes)
	assert.Equal(t, 2, stored.EmergencyPriority)

	otherCtx, _ := otherTenantContext(t, db)
	require.ErrorIs(t, buildLifecycleModule(t, db).PromoteGuardianLinkToLegalGuardian(otherCtx, linkID),
		peopledirectory.ErrGuardianLinkNotFound, "another school cannot reach the relationship")
	require.NoError(t, module.DeleteStudentGuardianLink(ctx, linkID))
	require.ErrorIs(t, module.PromoteGuardianLinkToLegalGuardian(ctx, linkID), peopledirectory.ErrGuardianLinkNotFound)
}

func TestLinkGuardianProfileToAccountMovesOnlyAChildlessBinding(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildLifecycleModule(t, db)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	account := testpkg.CreateTestAccount(t, db, "lifecycle-account")
	orphan := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Orphan", "Contact", "lifecycle-orphan")
	parent := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "New", "Contact", "lifecycle-parent")
	third := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Third", "Contact", "lifecycle-third")

	require.NoError(t, module.LinkGuardianProfileToAccount(ctx, orphan.ID, account.ID))
	require.NoError(t, module.LinkGuardianProfileToAccount(ctx, orphan.ID, account.ID), "linking again is a no-op")
	require.NoError(t, module.LinkGuardianProfileToAccount(ctx, parent.ID, account.ID))
	released := readGuardianProfile(t, db, orphan.ID)
	assert.Nil(t, released.AccountID, "the childless profile hands the account over")
	assert.False(t, released.HasAccount)
	linked := readGuardianProfile(t, db, parent.ID)
	require.NotNil(t, linked.AccountID)
	assert.Equal(t, account.ID, *linked.AccountID)
	assert.True(t, linked.HasAccount)

	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Linked", "Child", "1a")
	_, err := module.LinkInvitedGuardian(ctx, peopledirectory.InvitedGuardianLink{
		StudentID: student.ID, GuardianProfileID: parent.ID, RelationshipType: "parent",
	})
	require.NoError(t, err)
	err = module.LinkGuardianProfileToAccount(ctx, third.ID, account.ID)
	require.ErrorIs(t, err, peopledirectory.ErrGuardianAccountConflict, "a holder with children keeps the account")
	assert.EqualError(t, err, "guardian account link conflicts with an existing profile")
	assert.Nil(t, readGuardianProfile(t, db, third.ID).AccountID)
	assert.Equal(t, account.ID, *readGuardianProfile(t, db, parent.ID).AccountID)

	other := testpkg.CreateTestAccount(t, db, "lifecycle-other-account")
	require.ErrorIs(t, module.LinkGuardianProfileToAccount(ctx, parent.ID, other.ID), peopledirectory.ErrGuardianAccountConflict,
		"a profile never trades its account")
	require.ErrorIs(t, module.LinkGuardianProfileToAccount(ctx, 0, account.ID), peopledirectory.ErrGuardianAccountConflict)

	otherCtx, _ := otherTenantContext(t, db)
	require.ErrorIs(t, module.LinkGuardianProfileToAccount(otherCtx, third.ID, other.ID), peopledirectory.ErrGuardianNotFound)
}

func TestDeleteGuardianProfileAndLinkStayInTheSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildLifecycleModule(t, db)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Revoked", "Child", "1a")
	guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Revoked", "Parent", "lifecycle-revoked")
	orphan := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Rejected", "Contact", "lifecycle-rejected")
	_, err := module.LinkInvitedGuardian(ctx, peopledirectory.InvitedGuardianLink{
		StudentID: student.ID, GuardianProfileID: guardian.ID, RelationshipType: "parent",
	})
	require.NoError(t, err)
	link := readGuardianLink(t, db, student.ID, guardian.ID)

	otherCtx, _ := otherTenantContext(t, db)
	require.NoError(t, module.DeleteStudentGuardianLink(otherCtx, link.ID))
	assert.Equal(t, 3, countLinkHalves(t, db, student.ID, guardian.ID), "another school deletes nothing")
	require.NoError(t, module.DeleteStudentGuardianLink(ctx, link.ID))
	assert.Zero(t, countLinkHalves(t, db, student.ID, guardian.ID), "the pickup and access halves follow")
	require.NoError(t, module.DeleteStudentGuardianLink(ctx, link.ID), "a missing relationship is not an error")

	require.ErrorIs(t, module.DeleteGuardianProfile(otherCtx, orphan.ID), peopledirectory.ErrGuardianNotFound)
	require.NoError(t, module.DeleteGuardianProfile(ctx, orphan.ID))
	var remaining int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.guardian_profiles WHERE id = ?`, orphan.ID).
		Scan(context.Background(), &remaining))
	assert.Zero(t, remaining)
	require.ErrorIs(t, module.DeleteGuardianProfile(ctx, orphan.ID), peopledirectory.ErrGuardianNotFound)
}
