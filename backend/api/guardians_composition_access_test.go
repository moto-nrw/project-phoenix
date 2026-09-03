package api

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Access-control goldens of the guardian surface: which permission unlocks
// which write, and that a refused write leaves the rows untouched.

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
		"relationship_type": "parent", "guardian_role": "primary_guardian",
		"can_pickup": true, "is_emergency_contact": true, "emergency_priority": 1,
	}}}
}

func (c *guardianCompositionContext) requireLinkState(t *testing.T, linkID int64, role string, canPickup, emergency, portalAccess bool) {
	t.Helper()
	state := c.persistedLink(linkID)
	assert.Equal(t, role, state.Role)
	assert.Equal(t, canPickup, state.CanPickup)
	assert.Equal(t, emergency, state.IsEmergencyContact)
	assert.Equal(t, portalAccess, state.PortalAccess)
}

func TestGuardianComposition_CreateRoutesRejectStaffWithoutPermissions(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	accountID := ctx.staffAccount("ReadOnly", "Staff")
	claims := guardianStaffClaims(accountID)
	studentID, _ := ctx.createStudent("Unlinked", "Child", "1b")
	createdEmail := fmt.Sprintf("denied-create-%d@example.test", testpkg.UniqueSuffix())
	batchEmail := fmt.Sprintf("denied-batch-%d@example.test", testpkg.UniqueSuffix())

	t.Run("create guardian", func(t *testing.T) {
		require.Equal(t, http.StatusForbidden, ctx.status(t, claims, http.MethodPost, "/", map[string]any{
			"first_name": "Denied", "last_name": "Create", "email": createdEmail,
		}))
		assert.Equal(t, 0, ctx.guardianEmailCount(createdEmail))
	})

	t.Run("batch create and link", func(t *testing.T) {
		require.Equal(t, http.StatusForbidden, ctx.status(t, claims, http.MethodPost, fmt.Sprintf("/students/%d/guardians/batch", studentID), batchBody(batchEmail)))
		assert.Empty(t, ctx.studentGuardians(t, studentID))
		assert.Equal(t, 0, ctx.guardianEmailCount(batchEmail))
	})

	t.Run("batch create requires create permission", func(t *testing.T) {
		require.Equal(t, http.StatusForbidden, ctx.status(t, guardianStaffClaims(accountID, permissions.UsersUpdate), http.MethodPost,
			fmt.Sprintf("/students/%d/guardians/batch", studentID), batchBody(batchEmail)))
		assert.Empty(t, ctx.studentGuardians(t, studentID))
		assert.Equal(t, 0, ctx.guardianEmailCount(batchEmail))
	})
}

func TestGuardianComposition_RelationshipRoutesRejectStaffWithoutPermissions(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	accountID := ctx.staffAccount("ReadOnly", "RelationshipStaff")
	claims := guardianStaffClaims(accountID)
	studentID, _ := ctx.createStudent("Access", "Child", "1a")
	guardianID, _ := ctx.createGuardian("guardian-access")
	// A pickup-only link created through the API carries no pickup flag and
	// no portal access; the refused writes must leave it exactly so.
	rr := ctx.do(t, testutil.AdminTestClaims(999), http.MethodPost, fmt.Sprintf("/students/%d/guardians", studentID), relationshipBody(guardianID, "pickup_only"))
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	linkID := int64(dataObject(t, rr.Body.Bytes())["id"].(float64))
	unlinkedStudentID, _ := ctx.createStudent("Unlinked", "RelationshipChild", "1b")
	unlinkedGuardianID, _ := ctx.createGuardian("unlinked-relationship-access")

	t.Run("link guardian", func(t *testing.T) {
		require.Equal(t, http.StatusForbidden, ctx.status(t, claims, http.MethodPost,
			fmt.Sprintf("/students/%d/guardians", unlinkedStudentID), relationshipBody(unlinkedGuardianID, "primary_guardian")))
		assert.Empty(t, ctx.studentGuardians(t, unlinkedStudentID))
	})

	t.Run("grant parent rights and pickup authority", func(t *testing.T) {
		require.Equal(t, http.StatusForbidden, ctx.status(t, claims, http.MethodPut,
			fmt.Sprintf("/relationships/%d", linkID), relationshipUpdateBody("primary_guardian", true)))
		ctx.requireLinkState(t, linkID, "pickup_only", false, false, false)
	})

	t.Run("remove relationship", func(t *testing.T) {
		require.Equal(t, http.StatusForbidden, ctx.status(t, claims, http.MethodDelete,
			fmt.Sprintf("/students/%d/guardians/%d", studentID, guardianID), nil))
		ctx.requireLinkState(t, linkID, "pickup_only", false, false, false)
	})
}

func TestGuardianComposition_WriteRoutesAllowStaffWithRequiredPermissions(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	accountID := ctx.staffAccount("Authorised", "Staff")
	createClaims := guardianStaffClaims(accountID, permissions.UsersCreate)
	updateClaims := guardianStaffClaims(accountID, permissions.UsersUpdate)
	createAndUpdateClaims := guardianStaffClaims(accountID, permissions.UsersCreate, permissions.UsersUpdate)
	studentID, _ := ctx.createStudent("Authorised", "Child", "1a")
	guardianID, _ := ctx.createGuardian("authorised-access")

	createdEmail := fmt.Sprintf("allowed-create-%d@example.test", testpkg.UniqueSuffix())
	require.Equal(t, http.StatusCreated, ctx.status(t, createClaims, http.MethodPost, "/", map[string]any{
		"first_name": "Allowed", "last_name": "Create", "email": createdEmail,
	}))
	assert.Equal(t, 1, ctx.guardianEmailCount(createdEmail))

	require.Equal(t, http.StatusCreated, ctx.status(t, updateClaims, http.MethodPost,
		fmt.Sprintf("/students/%d/guardians", studentID), relationshipBody(guardianID, "pickup_only")))
	linked := ctx.studentGuardians(t, studentID)
	require.Len(t, linked, 1)
	linkID := linked[0].RelationshipID

	require.Equal(t, http.StatusOK, ctx.status(t, updateClaims, http.MethodPut,
		fmt.Sprintf("/relationships/%d", linkID), relationshipUpdateBody("primary_guardian", true)))
	ctx.requireLinkState(t, linkID, "primary_guardian", true, true, true)

	require.Equal(t, http.StatusOK, ctx.status(t, updateClaims, http.MethodDelete,
		fmt.Sprintf("/students/%d/guardians/%d", studentID, guardianID), nil))
	assert.Empty(t, ctx.studentGuardians(t, studentID))

	existingBatchStudentID, _ := ctx.createStudent("Batch", "Existing", "1b")
	existingBatchGuardianID, _ := ctx.createGuardian("allowed-batch-existing")
	require.Equal(t, http.StatusCreated, ctx.status(t, updateClaims, http.MethodPost,
		fmt.Sprintf("/students/%d/guardians/batch", existingBatchStudentID), map[string]any{"guardians": []map[string]any{{
			"guardian_profile_id": existingBatchGuardianID,
			"relationship_type":   "parent",
			"guardian_role":       "pickup_only",
			"emergency_priority":  1,
		}}}))
	require.Len(t, ctx.studentGuardians(t, existingBatchStudentID), 1)

	batchStudentID, _ := ctx.createStudent("Batch", "Allowed", "1b")
	require.Equal(t, http.StatusCreated, ctx.status(t, createAndUpdateClaims, http.MethodPost,
		fmt.Sprintf("/students/%d/guardians/batch", batchStudentID), batchBody(fmt.Sprintf("allowed-batch-%d@example.test", testpkg.UniqueSuffix()))))
	batchLinked := ctx.studentGuardians(t, batchStudentID)
	require.Len(t, batchLinked, 1)
	assert.True(t, ctx.linkGrantsPortalAccess(batchLinked[0].RelationshipID))
}

func TestGuardianComposition_RelationshipWritesAllowAdmin(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	studentID, _ := ctx.createStudent("Admin", "Child", "1a")
	guardianID, _ := ctx.createGuardian("admin-access")
	claims := testutil.AdminTestClaims(int(ctx.account("guardian-access-admin")))

	require.Equal(t, http.StatusCreated, ctx.status(t, claims, http.MethodPost,
		fmt.Sprintf("/students/%d/guardians", studentID), relationshipBody(guardianID, "pickup_only")))
	linked := ctx.studentGuardians(t, studentID)
	require.Len(t, linked, 1)

	require.Equal(t, http.StatusOK, ctx.status(t, claims, http.MethodPut,
		fmt.Sprintf("/relationships/%d", linked[0].RelationshipID), relationshipUpdateBody("primary_guardian", true)))
	require.Equal(t, http.StatusOK, ctx.status(t, claims, http.MethodDelete,
		fmt.Sprintf("/students/%d/guardians/%d", studentID, guardianID), nil))
	assert.Empty(t, ctx.studentGuardians(t, studentID))
}

func TestGuardianComposition_RelationshipWritesRejectForeignTenant(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	studentID, guardianID, linkID := ctx.foreignTenant()
	claims := testutil.AdminTestClaims(int(ctx.account("guardian-foreign-admin")))

	require.Equal(t, http.StatusNotFound, ctx.status(t, claims, http.MethodPut,
		fmt.Sprintf("/relationships/%d", linkID), relationshipUpdateBody("pickup_only", false)))
	require.Equal(t, http.StatusForbidden, ctx.status(t, claims, http.MethodDelete,
		fmt.Sprintf("/students/%d/guardians/%d", studentID, guardianID), nil))

	ctx.requireLinkState(t, linkID, "primary_guardian", true, true, true)
}
