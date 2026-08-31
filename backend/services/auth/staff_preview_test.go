package auth_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/services/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// assignSeededRoleForTenant links an account to a migration-seeded role
// (looked up by name) inside the test's tenant.
func assignSeededRoleForTenant(t *testing.T, db *bun.DB, accountID, tenantID int64, roleName string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var roleID int64
	err := db.NewSelect().
		ColumnExpr("id").
		TableExpr("auth.roles").
		Where("name = ?", roleName).
		Scan(ctx, &roleID)
	require.NoErrorf(t, err, "seeded role %q must exist", roleName)

	_, err = db.ExecContext(ctx, `
		INSERT INTO auth.account_roles (account_id, role_id, tenant_id, created_at, updated_at)
		VALUES (?, ?, ?, NOW(), NOW())
		ON CONFLICT DO NOTHING`, accountID, roleID, tenantID)
	require.NoError(t, err, "failed to assign role")
}

func decodePreviewTokenPayload(t *testing.T, token string) map[string]any {
	t.Helper()
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var claims map[string]any
	require.NoError(t, json.Unmarshal(payload, &claims))
	return claims
}

func TestAuthService_StartStaffPreview(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)

	admin := testpkg.CreateTestAccount(t, db, "preview-admin")

	t.Run("mints a read-only token carrying the target's identity", func(t *testing.T) {
		_, target := testpkg.CreateTestCalendarStaff(t, db, "Erika", "Beispiel")

		session, err := service.StartStaffPreview(ctx, admin.ID, tenantID, target.ID, "127.0.0.1", "go-test")
		require.NoError(t, err)
		require.NotNil(t, session)

		assert.Equal(t, target.ID, session.TargetAccountID)
		assert.Equal(t, "Erika Beispiel", session.TargetName)
		assert.Positive(t, session.ExpiresIn)

		claims := decodePreviewTokenPayload(t, session.AccessToken)
		assert.EqualValues(t, target.ID, claims["id"])
		assert.Equal(t, true, claims["read_only"])
		assert.EqualValues(t, admin.ID, claims["acting_admin_id"])
		assert.EqualValues(t, tenantID, claims["tenant_id"])
		assert.Equal(t, target.Email, claims["sub"])
		// access-only: a preview token must never carry a refresh-token family
		_, hasFamily := claims["family_id"]
		assert.False(t, hasFamily)
		// the roles are the TARGET's tenant-scoped roles
		roles, ok := claims["roles"].([]any)
		require.True(t, ok)
		assert.Contains(t, roles, "user")

		// audit lands on the ADMIN's account with the target in the metadata
		require.Eventually(t, func() bool {
			var count int
			err := db.NewSelect().
				ColumnExpr("COUNT(*)").
				TableExpr("audit.auth_events").
				Where("account_id = ?", admin.ID).
				Where("event_type = ?", "staff_preview_started").
				Where("metadata->>'target_account_id' = ?", fmt.Sprint(target.ID)).
				Scan(ctx, &count)
			return err == nil && count == 1
		}, 5*time.Second, 100*time.Millisecond, "staff_preview_started audit event missing")
	})

	t.Run("refuses previewing yourself", func(t *testing.T) {
		_, err := service.StartStaffPreview(ctx, admin.ID, tenantID, admin.ID, "", "")
		requirePreviewErr(t, err, auth.ErrPreviewSelf)
	})

	t.Run("refuses a non-existent target", func(t *testing.T) {
		_, err := service.StartStaffPreview(ctx, admin.ID, tenantID, 999999999, "", "")
		requirePreviewErr(t, err, auth.ErrAccountNotFound)
	})

	t.Run("refuses an inactive target", func(t *testing.T) {
		_, target := testpkg.CreateTestCalendarStaff(t, db, "Inaktiv", "Person")
		_, dbErr := db.Exec("UPDATE auth.accounts SET active = false WHERE id = ?", target.ID)
		require.NoError(t, dbErr)

		_, err := service.StartStaffPreview(ctx, admin.ID, tenantID, target.ID, "", "")
		requirePreviewErr(t, err, auth.ErrAccountInactive)
	})

	t.Run("refuses a target without a mapping at this school", func(t *testing.T) {
		target := testpkg.CreateTestAccount(t, db, "preview-unmapped")
		testpkg.UnclaimTestAccount(t, db, target.ID)
		// Unclaiming makes the account tenant-less shared state — the subject
		// of this subtest — so it has to go away again (leftover gate).
		t.Cleanup(func() {
			_, err := db.Exec("DELETE FROM auth.accounts WHERE id = ?", target.ID)
			require.NoError(t, err)
		})

		_, err := service.StartStaffPreview(ctx, admin.ID, tenantID, target.ID, "", "")
		requirePreviewErr(t, err, auth.ErrTenantAccessDenied)
	})

	t.Run("refuses a guardian-only target", func(t *testing.T) {
		target := testpkg.CreateTestAccount(t, db, "preview-guardian")
		assignSeededRoleForTenant(t, db, target.ID, tenantID, "guardian")

		_, err := service.StartStaffPreview(ctx, admin.ID, tenantID, target.ID, "", "")
		requirePreviewErr(t, err, auth.ErrPreviewTargetNotStaff)
	})

	t.Run("refuses a target without any role at this school", func(t *testing.T) {
		target := testpkg.CreateTestAccount(t, db, "preview-roleless")

		_, err := service.StartStaffPreview(ctx, admin.ID, tenantID, target.ID, "", "")
		requirePreviewErr(t, err, auth.ErrPreviewTargetNotStaff)
	})

	t.Run("refuses a Lehrkraft-only target", func(t *testing.T) {
		target := testpkg.CreateTestAccount(t, db, "preview-lehrkraft")
		testpkg.AssignLehrkraftSystemRole(t, db, target.ID, tenantID)

		_, err := service.StartStaffPreview(ctx, admin.ID, tenantID, target.ID, "", "")
		requirePreviewErr(t, err, auth.ErrMustUseSchoolPortal)
	})
}

func requirePreviewErr(t *testing.T, err error, want error) {
	t.Helper()
	require.Error(t, err)
	var authErr *auth.AuthError
	require.ErrorAs(t, err, &authErr)
	assert.ErrorIs(t, authErr.Err, want)
}

func TestAuthService_ListStaffPreviewCandidates(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)

	admin := testpkg.CreateTestAccount(t, db, "candidates-admin")
	assignSeededRoleForTenant(t, db, admin.ID, tenantID, "admin")

	_, eligible := testpkg.CreateTestCalendarStaff(t, db, "Wählbar", "Betreuung")

	guardianOnly := testpkg.CreateTestAccount(t, db, "candidates-guardian")
	assignSeededRoleForTenant(t, db, guardianOnly.ID, tenantID, "guardian")

	lehrkraftOnly := testpkg.CreateTestAccount(t, db, "candidates-lehrkraft")
	testpkg.AssignLehrkraftSystemRole(t, db, lehrkraftOnly.ID, tenantID)

	roleless := testpkg.CreateTestAccount(t, db, "candidates-roleless")

	candidates, err := service.ListStaffPreviewCandidates(ctx, tenantID, admin.ID)
	require.NoError(t, err)

	ids := make(map[int64]auth.StaffPreviewCandidate, len(candidates))
	for _, c := range candidates {
		ids[c.AccountID] = c
	}

	require.Contains(t, ids, eligible.ID, "staff with the user role must be selectable")
	assert.Equal(t, "Wählbar", ids[eligible.ID].FirstName)
	assert.Contains(t, ids[eligible.ID].Roles, "user")

	assert.NotContains(t, ids, admin.ID, "the caller must not preview themselves")
	assert.NotContains(t, ids, guardianOnly.ID, "guardian-only accounts have no tenant-portal surface")
	assert.NotContains(t, ids, lehrkraftOnly.ID, "Lehrkraft-only accounts live in moto schule")
	assert.NotContains(t, ids, roleless.ID, "accounts without a role cannot be previewed")
}
