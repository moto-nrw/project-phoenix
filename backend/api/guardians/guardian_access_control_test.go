package guardians_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/users"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func guardianStaffClaims(accountID int64, accountPermissions ...string) jwt.AppClaims {
	claims := testutil.TeacherTestClaims(int(accountID))
	claims.Permissions = accountPermissions
	return claims
}

func executeGuardianWrite(t *testing.T, ctx *testContext, claims jwt.AppClaims, method, path string, body any) int {
	t.Helper()
	req := testutil.NewAuthenticatedRequest(t, method, path, body, bearer(t, claims))
	return testutil.ExecuteRequest(ctx.resource.Router(), req).Code
}

func seedGuardianRelationship(t *testing.T, ctx *testContext, role string) (*users.Student, *users.GuardianProfile, *users.StudentGuardian) {
	t.Helper()
	student := testpkg.CreateTestStudent(t, ctx.db, "Access", "Child", "1a")
	guardian := testpkg.CreateTestGuardianProfile(t, ctx.db, "guardian-access")
	relationship, err := ctx.services.Guardian.LinkGuardianToStudent(testpkg.Ctx(t), usersSvc.StudentGuardianCreateRequest{
		StudentID:         student.ID,
		GuardianProfileID: guardian.ID,
		RelationshipType:  "parent",
		GuardianRole:      role,
		EmergencyPriority: 1,
	})
	require.NoError(t, err)
	return student, guardian, relationship
}

func requireRelationshipState(t *testing.T, ctx *testContext, relationshipID int64, role string, canPickup, emergency, portalAccess bool) {
	t.Helper()
	relationship, err := ctx.services.Guardian.GetStudentGuardianRelationship(testpkg.Ctx(t), relationshipID)
	require.NoError(t, err)
	assert.Equal(t, role, relationship.GuardianRole)
	assert.Equal(t, canPickup, relationship.CanPickup)
	assert.Equal(t, emergency, relationship.IsEmergencyContact)
	assert.Equal(t, portalAccess, authorize.StudentGuardianHasPermission(relationship, authorize.GuardianPermissionPortalAccess))
}

func requireNoGuardians(t *testing.T, ctx *testContext, studentID int64) {
	t.Helper()
	guardians, err := ctx.services.Guardian.GetStudentGuardians(testpkg.Ctx(t), studentID)
	require.NoError(t, err)
	assert.Empty(t, guardians)
}

func requireGuardianEmailCount(t *testing.T, ctx *testContext, email string, want int) {
	t.Helper()
	count, err := ctx.db.NewSelect().
		TableExpr(`users.guardian_profiles AS "guardian_profile"`).
		Where(`"guardian_profile".tenant_id = ?`, testpkg.Tenant(t)).
		Where(`"guardian_profile".email = ?`, email).
		Count(t.Context())
	require.NoError(t, err)
	assert.Equal(t, want, count)
}

func TestGuardianCreateRoutesRejectStaffWithoutPermissionsAndPreserveState(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)
	_, account := testpkg.CreateTestStaffWithAccount(t, ctx.db, "ReadOnly", "Staff")
	claims := guardianStaffClaims(account.ID)
	unlinkedStudent := testpkg.CreateTestStudent(t, ctx.db, "Unlinked", "Child", "1b")
	createdEmail := fmt.Sprintf("denied-create-%d@example.test", time.Now().UnixNano())
	batchEmail := fmt.Sprintf("denied-batch-%d@example.test", time.Now().UnixNano())

	t.Run("create guardian", func(t *testing.T) {
		status := executeGuardianWrite(t, ctx, claims, http.MethodPost, "/", map[string]any{
			"first_name": "Denied", "last_name": "Create", "email": createdEmail,
		})
		require.Equal(t, http.StatusForbidden, status)
		requireGuardianEmailCount(t, ctx, createdEmail, 0)
	})

	t.Run("batch create and link", func(t *testing.T) {
		status := executeGuardianWrite(t, ctx, claims, http.MethodPost,
			fmt.Sprintf("/students/%d/guardians/batch", unlinkedStudent.ID), batchBody(batchEmail))
		require.Equal(t, http.StatusForbidden, status)
		requireNoGuardians(t, ctx, unlinkedStudent.ID)
		requireGuardianEmailCount(t, ctx, batchEmail, 0)
	})

	t.Run("batch create requires create permission", func(t *testing.T) {
		status := executeGuardianWrite(t, ctx, guardianStaffClaims(account.ID, permissions.UsersUpdate), http.MethodPost,
			fmt.Sprintf("/students/%d/guardians/batch", unlinkedStudent.ID), batchBody(batchEmail))
		require.Equal(t, http.StatusForbidden, status)
		requireNoGuardians(t, ctx, unlinkedStudent.ID)
		requireGuardianEmailCount(t, ctx, batchEmail, 0)
	})
}

func TestGuardianRelationshipRoutesRejectStaffWithoutPermissionsAndPreserveState(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)
	_, account := testpkg.CreateTestStaffWithAccount(t, ctx.db, "ReadOnly", "RelationshipStaff")
	claims := guardianStaffClaims(account.ID)
	student, guardian, relationship := seedGuardianRelationship(t, ctx, authorize.GuardianRolePickupOnly)
	unlinkedStudent := testpkg.CreateTestStudent(t, ctx.db, "Unlinked", "RelationshipChild", "1b")
	unlinkedGuardian := testpkg.CreateTestGuardianProfile(t, ctx.db, "unlinked-relationship-access")

	t.Run("link guardian", func(t *testing.T) {
		status := executeGuardianWrite(t, ctx, claims, http.MethodPost,
			fmt.Sprintf("/students/%d/guardians", unlinkedStudent.ID), relationshipBody(unlinkedGuardian.ID, authorize.GuardianRolePrimaryGuardian))
		require.Equal(t, http.StatusForbidden, status)
		requireNoGuardians(t, ctx, unlinkedStudent.ID)
	})

	t.Run("grant parent rights and pickup authority", func(t *testing.T) {
		status := executeGuardianWrite(t, ctx, claims, http.MethodPut,
			fmt.Sprintf("/relationships/%d", relationship.ID), relationshipUpdateBody(authorize.GuardianRolePrimaryGuardian, true))
		require.Equal(t, http.StatusForbidden, status)
		requireRelationshipState(t, ctx, relationship.ID, authorize.GuardianRolePickupOnly, false, false, false)
	})

	t.Run("remove relationship", func(t *testing.T) {
		status := executeGuardianWrite(t, ctx, claims, http.MethodDelete,
			fmt.Sprintf("/students/%d/guardians/%d", student.ID, guardian.ID), nil)
		require.Equal(t, http.StatusForbidden, status)
		requireRelationshipState(t, ctx, relationship.ID, authorize.GuardianRolePickupOnly, false, false, false)
	})
}

func relationshipBody(guardianID int64, role string) map[string]any {
	return map[string]any{
		"guardian_profile_id": guardianID,
		"relationship_type":   "parent",
		"guardian_role":       role,
		"emergency_priority":  1,
	}
}

func relationshipUpdateBody(role string, enabled bool) map[string]any {
	return map[string]any{
		"guardian_role":        role,
		"can_pickup":           enabled,
		"is_emergency_contact": enabled,
	}
}

func batchBody(email string) map[string]any {
	return map[string]any{"guardians": []map[string]any{{
		"first_name": "Batch", "last_name": "Guardian", "email": email,
		"relationship_type": "parent", "guardian_role": authorize.GuardianRolePrimaryGuardian,
		"can_pickup": true, "is_emergency_contact": true, "emergency_priority": 1,
	}}}
}

func TestGuardianWriteRoutesAllowStaffWithRequiredPermissions(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)
	_, account := testpkg.CreateTestStaffWithAccount(t, ctx.db, "Authorised", "Staff")
	createClaims := guardianStaffClaims(account.ID, permissions.UsersCreate)
	updateClaims := guardianStaffClaims(account.ID, permissions.UsersUpdate)
	createAndUpdateClaims := guardianStaffClaims(account.ID, permissions.UsersCreate, permissions.UsersUpdate)
	student := testpkg.CreateTestStudent(t, ctx.db, "Authorised", "Child", "1a")
	guardian := testpkg.CreateTestGuardianProfile(t, ctx.db, "authorised-access")

	createdEmail := fmt.Sprintf("allowed-create-%d@example.test", time.Now().UnixNano())
	require.Equal(t, http.StatusCreated, executeGuardianWrite(t, ctx, createClaims, http.MethodPost, "/", map[string]any{
		"first_name": "Allowed", "last_name": "Create", "email": createdEmail,
	}))
	requireGuardianEmailCount(t, ctx, createdEmail, 1)

	require.Equal(t, http.StatusCreated, executeGuardianWrite(t, ctx, updateClaims, http.MethodPost,
		fmt.Sprintf("/students/%d/guardians", student.ID), relationshipBody(guardian.ID, authorize.GuardianRolePickupOnly)))
	guardians, err := ctx.services.Guardian.GetStudentGuardians(testpkg.Ctx(t), student.ID)
	require.NoError(t, err)
	require.Len(t, guardians, 1)
	relationshipID := guardians[0].Relationship.ID

	require.Equal(t, http.StatusOK, executeGuardianWrite(t, ctx, updateClaims, http.MethodPut,
		fmt.Sprintf("/relationships/%d", relationshipID), relationshipUpdateBody(authorize.GuardianRolePrimaryGuardian, true)))
	requireRelationshipState(t, ctx, relationshipID, authorize.GuardianRolePrimaryGuardian, true, true, true)

	require.Equal(t, http.StatusOK, executeGuardianWrite(t, ctx, updateClaims, http.MethodDelete,
		fmt.Sprintf("/students/%d/guardians/%d", student.ID, guardian.ID), nil))
	requireNoGuardians(t, ctx, student.ID)

	existingBatchStudent := testpkg.CreateTestStudent(t, ctx.db, "Batch", "Existing", "1b")
	existingBatchGuardian := testpkg.CreateTestGuardianProfile(t, ctx.db, "allowed-batch-existing")
	require.Equal(t, http.StatusCreated, executeGuardianWrite(t, ctx, updateClaims, http.MethodPost,
		fmt.Sprintf("/students/%d/guardians/batch", existingBatchStudent.ID), map[string]any{"guardians": []map[string]any{{
			"guardian_profile_id": existingBatchGuardian.ID,
			"relationship_type":   "parent",
			"guardian_role":       authorize.GuardianRolePickupOnly,
			"emergency_priority":  1,
		}}}))
	existingLinked, err := ctx.services.Guardian.GetStudentGuardians(testpkg.Ctx(t), existingBatchStudent.ID)
	require.NoError(t, err)
	require.Len(t, existingLinked, 1)

	batchStudent := testpkg.CreateTestStudent(t, ctx.db, "Batch", "Allowed", "1b")
	require.Equal(t, http.StatusCreated, executeGuardianWrite(t, ctx, createAndUpdateClaims, http.MethodPost,
		fmt.Sprintf("/students/%d/guardians/batch", batchStudent.ID), batchBody(fmt.Sprintf("allowed-batch-%d@example.test", time.Now().UnixNano()))))
	linked, err := ctx.services.Guardian.GetStudentGuardians(testpkg.Ctx(t), batchStudent.ID)
	require.NoError(t, err)
	require.Len(t, linked, 1)
	assert.True(t, authorize.StudentGuardianHasPermission(linked[0].Relationship, authorize.GuardianPermissionPortalAccess))
}

func TestGuardianRelationshipWritesAllowAdmin(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)
	student := testpkg.CreateTestStudent(t, ctx.db, "Admin", "Child", "1a")
	guardian := testpkg.CreateTestGuardianProfile(t, ctx.db, "admin-access")
	adminAccount := testpkg.CreateTestAccount(t, ctx.db, "guardian-access-admin")
	claims := testutil.AdminTestClaims(int(adminAccount.ID))

	require.Equal(t, http.StatusCreated, executeGuardianWrite(t, ctx, claims, http.MethodPost,
		fmt.Sprintf("/students/%d/guardians", student.ID), relationshipBody(guardian.ID, authorize.GuardianRolePickupOnly)))
	linked, err := ctx.services.Guardian.GetStudentGuardians(testpkg.Ctx(t), student.ID)
	require.NoError(t, err)
	require.Len(t, linked, 1)

	require.Equal(t, http.StatusOK, executeGuardianWrite(t, ctx, claims, http.MethodPut,
		fmt.Sprintf("/relationships/%d", linked[0].Relationship.ID), relationshipUpdateBody(authorize.GuardianRolePrimaryGuardian, true)))
	require.Equal(t, http.StatusOK, executeGuardianWrite(t, ctx, claims, http.MethodDelete,
		fmt.Sprintf("/students/%d/guardians/%d", student.ID, guardian.ID), nil))
	requireNoGuardians(t, ctx, student.ID)
}

func TestGuardianRelationshipWritesRejectForeignTenant(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)
	foreignTenantID, _ := testpkg.CreateTestTenant(t, ctx.db)
	student := testpkg.CreateTestStudentForTenant(t, ctx.db, foreignTenantID, "Foreign", "Child", "1a")
	email := fmt.Sprintf("foreign-guardian-%d@example.test", time.Now().UnixNano())
	guardian := &users.GuardianProfile{FirstName: "Foreign", LastName: "Guardian", Email: &email}
	guardian.SetTenantID(foreignTenantID)
	require.NoError(t, ctx.db.NewInsert().Model(guardian).ModelTableExpr(`users.guardian_profiles`).Scan(t.Context()))

	relationship := &users.StudentGuardian{
		StudentID: student.ID, GuardianProfileID: guardian.ID, RelationshipType: "parent",
		CanPickup: true, IsEmergencyContact: true, EmergencyPriority: 1,
	}
	relationship.SetTenantID(foreignTenantID)
	authorize.ApplyStudentGuardianRole(relationship, authorize.GuardianRolePrimaryGuardian)
	require.NoError(t, ctx.db.NewInsert().Model(relationship).ModelTableExpr(`users.students_guardians`).Scan(t.Context()))
	adminAccount := testpkg.CreateTestAccount(t, ctx.db, "guardian-foreign-admin")
	claims := testutil.AdminTestClaims(int(adminAccount.ID))

	require.Equal(t, http.StatusNotFound, executeGuardianWrite(t, ctx, claims, http.MethodPut,
		fmt.Sprintf("/relationships/%d", relationship.ID), relationshipUpdateBody(authorize.GuardianRolePickupOnly, false)))
	require.Equal(t, http.StatusForbidden, executeGuardianWrite(t, ctx, claims, http.MethodDelete,
		fmt.Sprintf("/students/%d/guardians/%d", student.ID, guardian.ID), nil))

	var persisted users.StudentGuardian
	require.NoError(t, ctx.db.NewSelect().Model(&persisted).
		ModelTableExpr(`users.students_guardians AS "student_guardian"`).
		Where(`"student_guardian".id = ?`, relationship.ID).
		Scan(t.Context()))
	assert.Equal(t, authorize.GuardianRolePrimaryGuardian, persisted.GuardianRole)
	assert.True(t, persisted.CanPickup)
	assert.True(t, persisted.IsEmergencyContact)
	assert.True(t, authorize.StudentGuardianHasPermission(&persisted, authorize.GuardianPermissionPortalAccess))
}
