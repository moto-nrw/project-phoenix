package compose

import (
	"context"
	"errors"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGuardianPortalContactsBatchProfilesAndRelationships(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	accountID, guardianID, studentID, _ := guardianRows(t, db, tenantID, "primary_guardian")
	sibling := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Sibling", "Directory", "2a")
	testpkg.CreateTestStudentGuardianLinkForTenant(t, db, tenantID, sibling.ID, guardianID, "pickup_only")
	_, loneGuardian, _, loneLink := guardianRows(t, db, tenantID, "primary_guardian")
	_, err := db.NewRaw("DELETE FROM users.students_guardians WHERE id = ?", loneLink).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw("UPDATE users.guardian_profiles SET portal_locale = 'en' WHERE id = ?", guardianID).Exec(ctx)
	require.NoError(t, err)

	contacts, err := module.ListGuardianPortalContacts(ctx, []int64{guardianID, loneGuardian}, nil)
	require.NoError(t, err)
	require.Len(t, contacts, 3)
	permitted := map[int64]bool{}
	for _, contact := range contacts {
		if contact.GuardianProfileID == loneGuardian {
			require.Nil(t, contact.StudentID, "a reachable profile without children is retained")
			require.Empty(t, contact.PortalPermission)
			continue
		}
		require.Equal(t, accountID, contact.AccountID)
		require.Equal(t, tenantID, contact.TenantID)
		require.Equal(t, "Sabine", contact.FirstName)
		require.Equal(t, "Directory", contact.LastName)
		require.NotNil(t, contact.Email)
		require.Equal(t, "en", *contact.PortalLocale)
		require.NotNil(t, contact.StudentID)
		permitted[*contact.StudentID] = string(contact.PortalPermission) == "true"
	}
	require.Equal(t, map[int64]bool{studentID: true, sibling.ID: false}, permitted,
		"portal permission belongs to each child")

	contacts, err = module.ListGuardianPortalContacts(ctx, nil, []int64{sibling.ID})
	require.NoError(t, err)
	require.Len(t, contacts, 1, "student selection must not return sibling relationships")
	require.Equal(t, sibling.ID, *contacts[0].StudentID)
	require.NotEqual(t, "true", string(contacts[0].PortalPermission))

	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	_, foreignGuardian, foreignStudent, _ := guardianRows(t, db, otherTenant, "primary_guardian")
	contacts, err = module.ListGuardianPortalContacts(ctx, []int64{foreignGuardian}, []int64{foreignStudent})
	require.NoError(t, err)
	require.Empty(t, contacts)

	_, err = db.NewRaw("UPDATE auth.account_tenants SET status = 'inactive' WHERE account_id = ? AND tenant_id = ?", accountID, tenantID).Exec(ctx)
	require.NoError(t, err)
	contacts, err = module.ListGuardianPortalContacts(ctx, []int64{guardianID}, []int64{studentID})
	require.NoError(t, err)
	require.Empty(t, contacts, "a linked profile cannot substitute for active school access")

	failure := errors.New("identity projection failed")
	failing, err := NewWithGuardianMemberships(Dependencies{DB: db, Observe: func(Observation) {}},
		func(context.Context, []int64) (map[int64][]int64, error) { return nil, failure })
	require.NoError(t, err)
	contacts, err = failing.ListGuardianPortalContacts(ctx, []int64{guardianID}, nil)
	require.ErrorIs(t, err, failure)
	require.Nil(t, contacts)
	contacts, err = failing.ListGuardianPortalContacts(ctx, nil, nil)
	require.NoError(t, err, "empty selections do not call dependencies")
	require.Empty(t, contacts)
}

func TestStudentsWithPortalGuardianNeedsAccountAndPortalAccess(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)

	// A guardian with an account and portal access.
	_, _, reachable, _ := guardianRows(t, db, tenantID, "primary_guardian")
	// An account alone is not enough: pickup_only carries no portal access.
	_, _, pickupOnly, _ := guardianRows(t, db, tenantID, "pickup_only")
	// Portal access without an account: the guardian was never invited.
	noAccount := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Nora", "Directory", "2a")
	uninvited := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Uwe", "Directory", "directory-uninvited")
	testpkg.CreateTestStudentGuardianLinkForTenant(t, db, tenantID, noAccount.ID, uninvited.ID, "primary_guardian")
	// No guardian at all.
	alone := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Omar", "Directory", "3a")

	result, err := module.StudentsWithPortalGuardian(ctx, []int64{reachable, pickupOnly, noAccount.ID, alone.ID, reachable, 0})
	require.NoError(t, err)
	assert.Equal(t, map[int64]bool{reachable: true}, result, "children without a portal guardian are absent")

	empty, err := module.StudentsWithPortalGuardian(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, empty)

	projectionFailure := errors.New("portal membership lookup failed")
	failing, err := NewWithGuardianMemberships(Dependencies{DB: db, Observe: func(Observation) {}},
		func(context.Context, []int64) (map[int64][]int64, error) { return nil, projectionFailure })
	require.NoError(t, err)
	failed, err := failing.StudentsWithPortalGuardian(ctx, []int64{reachable})
	require.ErrorIs(t, err, projectionFailure)
	require.Empty(t, failed, "failed account reachability must not return candidates")
}

func TestStudentsWithPortalGuardianStaysInsideTheTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	_, _, foreign, _ := guardianRows(t, db, otherTenant, "primary_guardian")

	result, err := module.StudentsWithPortalGuardian(testpkg.Ctx(t), []int64{foreign})
	require.NoError(t, err)
	assert.Empty(t, result, "another school's child is not visible")
}

func TestStudentsWithPortalGuardianExcludesInactiveMembership(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	tenantID := testpkg.Tenant(t)
	accountID, _, studentID, _ := guardianRows(t, db, tenantID, "primary_guardian")

	_, err := db.NewRaw(
		`UPDATE auth.account_tenants SET status = 'inactive', deactivated_at = NOW() WHERE account_id = ? AND tenant_id = ?`,
		accountID, tenantID,
	).Exec(testpkg.Ctx(t))
	require.NoError(t, err)

	result, err := module.StudentsWithPortalGuardian(testpkg.Ctx(t), []int64{studentID})
	require.NoError(t, err)
	assert.Empty(t, result, "a guardian without active school access cannot receive a parents-app reminder")
}

func TestStudentsWithPortalGuardianExcludesInactiveAccountsAndMissingRoles(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	tenantID := testpkg.Tenant(t)

	inactiveAccountID, _, inactiveStudentID, _ := guardianRows(t, db, tenantID, "primary_guardian")
	_, err := db.NewRaw(`UPDATE auth.accounts SET active = FALSE WHERE id = ?`, inactiveAccountID).Exec(testpkg.Ctx(t))
	require.NoError(t, err)

	rolelessAccountID, _, rolelessStudentID, _ := guardianRows(t, db, tenantID, "primary_guardian")
	_, err = db.NewRaw(`DELETE FROM auth.account_roles WHERE account_id = ? AND tenant_id = ?`, rolelessAccountID, tenantID).Exec(testpkg.Ctx(t))
	require.NoError(t, err)

	result, err := module.StudentsWithPortalGuardian(testpkg.Ctx(t), []int64{inactiveStudentID, rolelessStudentID})
	require.NoError(t, err)
	assert.Empty(t, result, "only active accounts with the guardian role can receive a parents-app reminder")
}
