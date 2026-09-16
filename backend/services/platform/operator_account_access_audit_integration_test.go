package platform_test

import (
	"context"
	"encoding/json"
	"testing"

	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The school-access flow hands its audit evidence to the root as typed
// values (#3252). The platform operator ledger and the school's auth event
// ledger must keep the JSON keys they always stored for each action.

// jsonColumn reads one JSON column of the newest matching row as text and
// decodes it; bun would treat a map scan target as a column map.
func jsonColumn(t *testing.T, query *bun.SelectQuery) map[string]any {
	t.Helper()
	var raw string
	require.NoError(t, query.OrderExpr("id DESC").Limit(1).Scan(context.Background(), &raw))
	var values map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &values))
	return values
}

func operatorAccessChanges(t *testing.T, db *bun.DB, operatorID int64, action string) map[string]any {
	t.Helper()
	return jsonColumn(t, db.NewSelect().
		TableExpr("platform.operator_audit_log").
		ColumnExpr("changes::text").
		Where("operator_id = ? AND action = ? AND resource_type = ?", operatorID, action, "account_tenant"))
}

func accessEventMetadata(t *testing.T, db *bun.DB, accountID int64, eventType string) map[string]any {
	t.Helper()
	return jsonColumn(t, db.NewSelect().
		TableExpr("audit.auth_events").
		ColumnExpr("metadata::text").
		Where("account_id = ? AND event_type = ?", accountID, eventType))
}

func assertKeys(t *testing.T, values map[string]any, keys ...string) {
	t.Helper()
	got := make([]string, 0, len(values))
	for key := range values {
		got = append(got, key)
	}
	assert.ElementsMatch(t, keys, got)
}

func TestIntegration_AccountTenantAccess_AuditEvidenceKeepsItsKeys(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := buildProvisioningService(t, db)
	ctx := context.Background()
	account, cleanupAccount := setupAccessTestAccount(t, db)
	defer cleanupAccount()
	operator := testpkg.CreateTestOperator(t, db)
	target := accessTargetTenantID(t)
	adminRoleID := systemRoleID(t, db, "admin")
	customRole := createTenantRole(t, db, "Verwaltung", target, nil)
	defer cleanupTenantRole(t, db, customRole.ID)
	var schoolName string
	require.NoError(t, db.NewSelect().TableExpr("platform.schools").Column("name").Where("id = ?", target).Scan(ctx, &schoolName))
	require.NotEmpty(t, schoolName)

	_, err := service.GrantAccountTenantAccess(ctx, account.ID, target,
		platformSvc.GrantAccountTenantAccessRequest{RoleID: adminRoleID}, operator.ID, testClientIP)
	require.NoError(t, err)

	granted := operatorAccessChanges(t, db, operator.ID, "create")
	assertKeys(t, granted, "schoolID", "email", "roleID", "roleName")
	assert.EqualValues(t, target, granted["schoolID"])
	assert.Equal(t, account.Email, granted["email"])
	assert.EqualValues(t, adminRoleID, granted["roleID"])
	assert.Equal(t, "admin", granted["roleName"])
	grantEvent := accessEventMetadata(t, db, account.ID, "tenant_access_granted")
	assertKeys(t, grantEvent, "school_id", "school_name", "role", "operator_id")
	assert.EqualValues(t, target, grantEvent["school_id"])
	assert.Equal(t, schoolName, grantEvent["school_name"])
	assert.Equal(t, "admin", grantEvent["role"])
	assert.EqualValues(t, operator.ID, grantEvent["operator_id"])

	// admin -> custom school role: the administrative role is removed and the
	// custom role does not block the later revocation.
	_, err = service.UpdateAccountTenantRole(ctx, account.ID, target, customRole.ID, operator.ID, testClientIP)
	require.NoError(t, err)
	changed := operatorAccessChanges(t, db, operator.ID, "update")
	assertKeys(t, changed, "schoolID", "email", "roleID", "roleName", "removedRoles")
	assert.Equal(t, "Verwaltung", changed["roleName"])
	assert.Equal(t, []any{"admin"}, changed["removedRoles"])
	changeEvent := accessEventMetadata(t, db, account.ID, "tenant_role_changed")
	assertKeys(t, changeEvent, "school_id", "school_name", "role", "removed_roles", "operator_id")
	assert.Equal(t, "Verwaltung", changeEvent["role"])
	assert.Equal(t, schoolName, changeEvent["school_name"])
	assert.Equal(t, []any{"admin"}, changeEvent["removed_roles"])

	_, err = service.RevokeAccountTenantAccess(ctx, account.ID, target, operator.ID, testClientIP)
	require.NoError(t, err)
	revoked := operatorAccessChanges(t, db, operator.ID, "delete")
	assertKeys(t, revoked, "schoolID", "email", "accountDeactivated")
	assert.Equal(t, false, revoked["accountDeactivated"], "the account keeps its own school")
	revokeEvent := accessEventMetadata(t, db, account.ID, "tenant_access_revoked")
	assertKeys(t, revokeEvent, "school_id", "school_name", "account_deactivated", "operator_id")
	assert.Equal(t, false, revokeEvent["account_deactivated"])
	assert.Equal(t, schoolName, revokeEvent["school_name"])
}

// A Lehrkraft assignment left behind without its school mapping must not
// let a grant swap the account to another role there, as the retained role
// assignment refused.
func TestIntegration_GrantAccountTenantAccess_KeepsALeftoverLehrkraftRole(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := buildProvisioningService(t, db)
	ctx := context.Background()
	account, cleanupAccount := setupAccessTestAccount(t, db)
	defer cleanupAccount()
	operator := testpkg.CreateTestOperator(t, db)
	target := accessTargetTenantID(t)
	lehrkraftRoleID := systemRoleID(t, db, "lehrkraft")
	adminRoleID := systemRoleID(t, db, "admin")

	_, err := db.ExecContext(ctx,
		`INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)`,
		account.ID, lehrkraftRoleID, target)
	require.NoError(t, err)

	_, err = service.GrantAccountTenantAccess(ctx, account.ID, target,
		platformSvc.GrantAccountTenantAccessRequest{RoleID: adminRoleID}, operator.ID, testClientIP)
	var invalid *platformSvc.InvalidDataError
	require.ErrorAs(t, err, &invalid)

	var mappings int
	err = db.NewSelect().TableExpr("auth.account_tenants").ColumnExpr("count(*)").
		Where("account_id = ? AND tenant_id = ?", account.ID, target).Scan(ctx, &mappings)
	require.NoError(t, err)
	assert.Equal(t, 0, mappings, "the refused grant writes nothing")
}

// A role change at one school removes the replaced role only there: the
// same role the account holds at another school stays (#1021). This guard
// used to sit on the retained school-scoped role deletion, which the
// Identity & Access store now performs.
func TestIntegration_UpdateAccountTenantRole_RemovesTheRoleOnlyAtThatSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := buildProvisioningService(t, db)
	ctx := context.Background()
	account, cleanupAccount := setupAccessTestAccount(t, db)
	defer cleanupAccount()
	operator := testpkg.CreateTestOperator(t, db)
	home := testSchoolID(t)
	target := accessTargetTenantID(t)
	adminRoleID := systemRoleID(t, db, "admin")
	userRoleID := systemRoleID(t, db, "user")

	_, err := db.ExecContext(ctx,
		`INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)`,
		account.ID, adminRoleID, home)
	require.NoError(t, err)

	_, err = service.GrantAccountTenantAccess(ctx, account.ID, target,
		platformSvc.GrantAccountTenantAccessRequest{RoleID: adminRoleID}, operator.ID, testClientIP)
	require.NoError(t, err)
	entries, err := service.UpdateAccountTenantRole(ctx, account.ID, target, userRoleID, operator.ID, testClientIP)
	require.NoError(t, err)

	assert.Equal(t, []string{"user"}, roleNamesAt(entries, target), "the replaced role is gone at the changed school")
	assert.Equal(t, []string{"admin"}, roleNamesAt(entries, home), "the same role at the other school stays")
	var homeAdmins int
	err = db.NewSelect().TableExpr("auth.account_roles").ColumnExpr("count(*)").
		Where("account_id = ? AND role_id = ? AND tenant_id = ?", account.ID, adminRoleID, home).Scan(ctx, &homeAdmins)
	require.NoError(t, err)
	assert.Equal(t, 1, homeAdmins)
}
