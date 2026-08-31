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

// assignCustomRoleNamed creates a tenant-scoped CUSTOM role with exactly the
// given name (no unique suffix — the point is the name collision with a system
// role) and assigns it to the account.
func assignCustomRoleNamed(t *testing.T, db *bun.DB, accountID, tenantID int64, roleName string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var roleID int64
	err := db.QueryRowContext(ctx, `
		INSERT INTO auth.roles (name, description, is_system, base_role, tenant_id, created_at, updated_at)
		VALUES (?, 'Eigene Rolle der Schule', false, 'user', ?, NOW(), NOW())
		RETURNING id`, roleName, tenantID).Scan(&roleID)
	require.NoError(t, err, "failed to create custom role")

	_, err = db.ExecContext(ctx, `
		INSERT INTO auth.account_roles (account_id, role_id, tenant_id, created_at, updated_at)
		VALUES (?, ?, ?, NOW(), NOW())
		ON CONFLICT DO NOTHING`, accountID, roleID, tenantID)
	require.NoError(t, err, "failed to assign custom role")
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

		session, err := service.StartStaffPreview(ctx, admin.ID, tenantID, target.ID, "", "127.0.0.1", "go-test")
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
		_, err := service.StartStaffPreview(ctx, admin.ID, tenantID, admin.ID, "", "", "")
		requirePreviewErr(t, err, auth.ErrPreviewSelf)
	})

	t.Run("refuses a non-existent target", func(t *testing.T) {
		_, err := service.StartStaffPreview(ctx, admin.ID, tenantID, 999999999, "", "", "")
		requirePreviewErr(t, err, auth.ErrAccountNotFound)
	})

	t.Run("refuses an inactive target", func(t *testing.T) {
		_, target := testpkg.CreateTestCalendarStaff(t, db, "Inaktiv", "Person")
		_, dbErr := db.Exec("UPDATE auth.accounts SET active = false WHERE id = ?", target.ID)
		require.NoError(t, dbErr)

		_, err := service.StartStaffPreview(ctx, admin.ID, tenantID, target.ID, "", "", "")
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

		_, err := service.StartStaffPreview(ctx, admin.ID, tenantID, target.ID, "", "", "")
		requirePreviewErr(t, err, auth.ErrTenantAccessDenied)
	})

	t.Run("refuses a guardian-only target", func(t *testing.T) {
		target := testpkg.CreateTestAccount(t, db, "preview-guardian")
		assignSeededRoleForTenant(t, db, target.ID, tenantID, "guardian")

		_, err := service.StartStaffPreview(ctx, admin.ID, tenantID, target.ID, "", "", "")
		requirePreviewErr(t, err, auth.ErrPreviewTargetNotStaff)
	})

	t.Run("refuses a target without any role at this school", func(t *testing.T) {
		target := testpkg.CreateTestAccount(t, db, "preview-roleless")

		_, err := service.StartStaffPreview(ctx, admin.ID, tenantID, target.ID, "", "", "")
		requirePreviewErr(t, err, auth.ErrPreviewTargetNotStaff)
	})

	t.Run("refuses a Lehrkraft-only target", func(t *testing.T) {
		target := testpkg.CreateTestAccount(t, db, "preview-lehrkraft")
		testpkg.AssignLehrkraftSystemRole(t, db, target.ID, tenantID)

		_, err := service.StartStaffPreview(ctx, admin.ID, tenantID, target.ID, "", "", "")
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

// A school's own role may carry the label "Lehrkraft" without being the
// lehrkraft SYSTEM role. StartStaffPreview accepts such an account, so the
// picker must list it — otherwise the admin sees a colleague in the staff list
// that the preview simply refuses to offer.
func TestAuthService_ListStaffPreviewCandidates_CustomLehrkraftRoleIsListed(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)

	admin := testpkg.CreateTestAccount(t, db, "custom-lehrkraft-admin")
	assignSeededRoleForTenant(t, db, admin.ID, tenantID, "admin")

	customLehrkraft := testpkg.CreateTestAccount(t, db, "custom-lehrkraft-target")
	assignCustomRoleNamed(t, db, customLehrkraft.ID, tenantID, "Lehrkraft")

	candidates, err := service.ListStaffPreviewCandidates(ctx, tenantID, admin.ID)
	require.NoError(t, err)

	listed := false
	for _, candidate := range candidates {
		if candidate.AccountID == customLehrkraft.ID {
			listed = true
		}
	}
	assert.True(t, listed, "an account with a school-defined role named Lehrkraft must be selectable")

	// And the picker agrees with the start path.
	_, err = service.StartStaffPreview(ctx, admin.ID, tenantID, customLehrkraft.ID, "", "", "")
	require.NoError(t, err)
}

func TestAuthService_EndStaffPreview(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)

	admin := testpkg.CreateTestAccount(t, db, "preview-end-admin")
	_, target := testpkg.CreateTestCalendarStaff(t, db, "Ende", "Person")

	session, err := service.StartStaffPreview(ctx, admin.ID, tenantID, target.ID, "", "", "")
	require.NoError(t, err)

	t.Run("reads the previewed account from the token and audits it", func(t *testing.T) {
		endedTarget, endErr := service.EndStaffPreview(ctx, admin.ID, tenantID, session.AccessToken, "127.0.0.1", "go-test")
		require.NoError(t, endErr)
		assert.Equal(t, target.ID, endedTarget)

		require.Eventually(t, func() bool {
			var count int
			countErr := db.NewSelect().
				ColumnExpr("COUNT(*)").
				TableExpr("audit.auth_events").
				Where("account_id = ?", admin.ID).
				Where("event_type = ?", "staff_preview_ended").
				Where("metadata->>'target_account_id' = ?", fmt.Sprint(target.ID)).
				Scan(ctx, &count)
			return countErr == nil && count == 1
		}, 5*time.Second, 100*time.Millisecond, "staff_preview_ended audit event missing")
	})

	// Without this, an admin could stamp the audit trail with a preview of a
	// colleague they never opened.
	t.Run("refuses a token that is not this admin's preview at this school", func(t *testing.T) {
		otherAdmin := testpkg.CreateTestAccount(t, db, "preview-end-other-admin")
		foreign, foreignErr := service.StartStaffPreview(ctx, otherAdmin.ID, tenantID, target.ID, "", "", "")
		require.NoError(t, foreignErr)

		_, endErr := service.EndStaffPreview(ctx, admin.ID, tenantID, foreign.AccessToken, "", "")
		requirePreviewErr(t, endErr, auth.ErrPreviewTokenInvalid)

		_, endErr = service.EndStaffPreview(ctx, admin.ID, tenantID+1, session.AccessToken, "", "")
		requirePreviewErr(t, endErr, auth.ErrPreviewTokenInvalid)

		_, endErr = service.EndStaffPreview(ctx, admin.ID, tenantID, "not-a-token", "", "")
		requirePreviewErr(t, endErr, auth.ErrPreviewTokenInvalid)
	})
}

// The audit trail must pair exactly one start with exactly one end per
// preview, no matter how long the preview ran or how often the client
// retried: renewals continue the instance, and a repeated end is ignored.
func TestAuthService_StaffPreviewAuditPairsOneStartWithOneEnd(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)

	admin := testpkg.CreateTestAccount(t, db, "preview-audit-admin")
	_, target := testpkg.CreateTestCalendarStaff(t, db, "Lange", "Vorschau")

	countEvents := func(eventType, previewID string) int {
		var count int
		query := db.NewSelect().
			ColumnExpr("COUNT(*)").
			TableExpr("audit.auth_events").
			Where("account_id = ?", admin.ID).
			Where("event_type = ?", eventType)
		if previewID != "" {
			query = query.Where("metadata->>'preview_id' = ?", previewID)
		}
		require.NoError(t, query.Scan(ctx, &count))
		return count
	}

	first, err := service.StartStaffPreview(ctx, admin.ID, tenantID, target.ID, "", "127.0.0.1", "go-test")
	require.NoError(t, err)
	previewID, ok := decodePreviewTokenPayload(t, first.AccessToken)["preview_id"].(string)
	require.True(t, ok)
	require.NotEmpty(t, previewID)

	// A renewal hands the expiring token back: same instance, no second start.
	renewed, err := service.StartStaffPreview(ctx, admin.ID, tenantID, target.ID, first.AccessToken, "127.0.0.1", "go-test")
	require.NoError(t, err)
	assert.Equal(t, previewID, decodePreviewTokenPayload(t, renewed.AccessToken)["preview_id"])

	// A start without that proof is a new preview and gets its own id.
	fresh, err := service.StartStaffPreview(ctx, admin.ID, tenantID, target.ID, "", "127.0.0.1", "go-test")
	require.NoError(t, err)
	assert.NotEqual(t, previewID, decodePreviewTokenPayload(t, fresh.AccessToken)["preview_id"])

	require.Eventually(t, func() bool {
		return countEvents("staff_preview_started", previewID) == 1
	}, 5*time.Second, 100*time.Millisecond, "renewal must not write a second staff_preview_started")

	// Ending twice — a retry, a second tab — stays one audit row.
	endedTarget, err := service.EndStaffPreview(ctx, admin.ID, tenantID, renewed.AccessToken, "127.0.0.1", "go-test")
	require.NoError(t, err)
	assert.Equal(t, target.ID, endedTarget)
	require.Eventually(t, func() bool {
		return countEvents("staff_preview_ended", previewID) == 1
	}, 5*time.Second, 100*time.Millisecond, "staff_preview_ended audit event missing")

	endedTarget, err = service.EndStaffPreview(ctx, admin.ID, tenantID, renewed.AccessToken, "127.0.0.1", "go-test")
	require.NoError(t, err)
	assert.Equal(t, target.ID, endedTarget)
	assert.Never(t, func() bool {
		return countEvents("staff_preview_ended", previewID) > 1
	}, 2*time.Second, 200*time.Millisecond, "repeated end must not write a second audit event")
}

// A token of a preview that has already ENDED must not revive that instance.
// If it did, the new preview would inherit the closed id: no start event
// would be written for it, and its own end would be swallowed by the
// uniqueness index — a preview running with no trace in the audit trail.
func TestAuthService_StartStaffPreviewIgnoresEndedPreviewToken(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)

	admin := testpkg.CreateTestAccount(t, db, "preview-stale-admin")
	_, target := testpkg.CreateTestCalendarStaff(t, db, "Alte", "Vorschau")

	countStarts := func(previewID string) int {
		var count int
		require.NoError(t, db.NewSelect().
			ColumnExpr("COUNT(*)").
			TableExpr("audit.auth_events").
			Where("account_id = ?", admin.ID).
			Where("event_type = ?", "staff_preview_started").
			Where("metadata->>'preview_id' = ?", previewID).
			Scan(ctx, &count))
		return count
	}

	first, err := service.StartStaffPreview(ctx, admin.ID, tenantID, target.ID, "", "127.0.0.1", "go-test")
	require.NoError(t, err)
	endedID, ok := decodePreviewTokenPayload(t, first.AccessToken)["preview_id"].(string)
	require.True(t, ok)

	_, err = service.EndStaffPreview(ctx, admin.ID, tenantID, first.AccessToken, "127.0.0.1", "go-test")
	require.NoError(t, err)

	// Same signed token, handed back as "previous" — the instance is closed,
	// so this is a NEW preview with a new id and its own start event.
	restarted, err := service.StartStaffPreview(ctx, admin.ID, tenantID, target.ID, first.AccessToken, "127.0.0.1", "go-test")
	require.NoError(t, err)
	newID, ok := decodePreviewTokenPayload(t, restarted.AccessToken)["preview_id"].(string)
	require.True(t, ok)
	require.NotEqual(t, endedID, newID)

	require.Eventually(t, func() bool {
		return countStarts(newID) == 1
	}, 5*time.Second, 100*time.Millisecond, "restart after an ended preview must write its own start event")
}

// Two tabs ending the same preview in the same moment must produce exactly
// one audit row. A read-then-write guard cannot promise that — both callers
// would read "not ended yet" before either row lands — so uniqueness lives in
// the database and this test is what pins it.
func TestAuthService_EndStaffPreviewConcurrentlyWritesOneEvent(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)

	admin := testpkg.CreateTestAccount(t, db, "preview-end-race-admin")
	_, target := testpkg.CreateTestCalendarStaff(t, db, "Gleichzeitig", "Beendet")

	session, err := service.StartStaffPreview(ctx, admin.ID, tenantID, target.ID, "", "127.0.0.1", "go-test")
	require.NoError(t, err)
	previewID, ok := decodePreviewTokenPayload(t, session.AccessToken)["preview_id"].(string)
	require.True(t, ok)

	const callers = 5
	start := make(chan struct{})
	errs := make(chan error, callers)
	for i := 0; i < callers; i++ {
		go func() {
			<-start
			_, endErr := service.EndStaffPreview(ctx, admin.ID, tenantID, session.AccessToken, "127.0.0.1", "go-test")
			errs <- endErr
		}()
	}
	close(start)
	for i := 0; i < callers; i++ {
		require.NoError(t, <-errs)
	}

	var count int
	require.NoError(t, db.NewSelect().
		ColumnExpr("COUNT(*)").
		TableExpr("audit.auth_events").
		Where("account_id = ?", admin.ID).
		Where("event_type = ?", "staff_preview_ended").
		Where("metadata->>'preview_id' = ?", previewID).
		Scan(ctx, &count))
	assert.Equal(t, 1, count, "concurrent ends must write exactly one audit event")
}
