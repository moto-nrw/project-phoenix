package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/rotation"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	emailPkg "github.com/moto-nrw/project-phoenix/email"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingAuditCommand struct{}

func (failingAuditCommand) Append(context.Context, any) error {
	return errors.New("forced audit failure")
}

func TestLogoutPersistsRevocationAuditWithoutRawFamilyID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("revocation-audit")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	t.Cleanup(func() {
		testpkg.CleanupAuthFixtures(t, db, account.ID)
		testpkg.CleanupTestTenant(t, db, tenantID)
	})

	_, refreshToken, err := service.Login(ctx, email, testPassword)
	require.NoError(t, err)
	var persisted authModels.Token
	require.NoError(t, db.NewSelect().Model(&persisted).ModelTableExpr(`auth.tokens AS "token"`).
		Where(`"token".account_id = ?`, account.ID).Scan(ctx))
	require.Equal(t, authModels.PortalScopeTenant, persisted.PortalScope)

	require.NoError(t, service.LogoutWithAudit(ctx, refreshToken, "192.0.2.10", "revocation-test"))

	var event auditModels.AuthEvent
	require.NoError(t, db.NewSelect().Model(&event).ModelTableExpr(`audit.auth_events AS "auth_event"`).
		Where(`"auth_event".account_id = ?`, account.ID).
		Where(`"auth_event".event_type = ?`, auditModels.EventTypeTokenRevoked).
		OrderExpr(`"auth_event".id DESC`).Limit(1).Scan(ctx))
	assert.Equal(t, tenantID, event.TenantID)
	assert.Equal(t, "192.0.2.10", event.IPAddress)
	assert.Equal(t, "revocation-test", event.UserAgent)
	assert.Equal(t, authModels.PortalScopeTenant, event.Metadata["portal_scope"])
	assert.Equal(t, "logout", event.Metadata["reason"])
	assert.Equal(t, float64(1), event.Metadata["revoked_token_count"])
	assert.Equal(t, rotation.FamilyFingerprint(persisted.FamilyID), event.Metadata["family_fingerprint"])
	assert.NotContains(t, event.Metadata["family_fingerprint"], persisted.FamilyID)
}

func TestRevocationAuditFailureRollsBackAndRetryIsIdempotent(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)
	workingService := setupAuthService(t, db)
	email, username := uniqueTestCredentials("revocation-rollback")
	account, err := workingService.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	t.Cleanup(func() {
		testpkg.CleanupAuthFixtures(t, db, account.ID)
		testpkg.CleanupTestTenant(t, db, tenantID)
	})
	_, _, err = workingService.Login(ctx, email, testPassword)
	require.NoError(t, err)

	repoFactory := repositories.NewFactory(db)
	config, err := authService.NewServiceConfig(nil, emailPkg.Email{}, "http://localhost:3000", time.Hour)
	require.NoError(t, err)
	config.Audit = failingAuditCommand{}
	service, err := authService.NewService(repoFactory, config, db, nil)
	require.NoError(t, err)
	testpkg.SetTenantRuntime(t, service, db)

	err = service.RevokeAllTokensWithReason(testpkg.TenantContext(tenantID), int(account.ID), "password_reset")
	require.ErrorContains(t, err, "forced audit failure")

	count, err := db.NewSelect().TableExpr("auth.tokens").Where("account_id = ?", account.ID).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "token deletion must roll back with the failed audit insert")

	// A caller may retry after the fail-closed transaction. The retry revokes
	// the still-present token and emits exactly one ledger row; another retry
	// sees no producer rows and must not duplicate the event.
	require.NoError(t, workingService.RevokeAllTokensWithReason(ctx, int(account.ID), "password_reset"))
	require.NoError(t, workingService.RevokeAllTokensWithReason(ctx, int(account.ID), "password_reset"))

	count, err = db.NewSelect().TableExpr("auth.tokens").Where("account_id = ?", account.ID).Count(ctx)
	require.NoError(t, err)
	assert.Zero(t, count)
	eventCount, err := db.NewSelect().TableExpr("audit.auth_events").
		Where("account_id = ?", account.ID).
		Where("event_type = ?", auditModels.EventTypeTokenRevoked).
		Where("metadata->>'reason' = ?", "password_reset").
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, eventCount, "retry must not append a duplicate revocation event")
}

func TestSessionCapAuditsEvictedTokenFamily(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("session-cap-audit")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	t.Cleanup(func() {
		testpkg.CleanupAuthFixtures(t, db, account.ID)
		testpkg.CleanupTestTenant(t, db, tenantID)
	})

	for range 6 {
		_, _, err = service.Login(ctx, email, testPassword)
		require.NoError(t, err)
	}

	tokenCount, err := db.NewSelect().TableExpr("auth.tokens").Where("account_id = ?", account.ID).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 5, tokenCount)

	var event auditModels.AuthEvent
	require.NoError(t, db.NewSelect().Model(&event).ModelTableExpr(`audit.auth_events AS "auth_event"`).
		Where(`"auth_event".account_id = ?`, account.ID).
		Where(`"auth_event".event_type = ?`, auditModels.EventTypeTokenRevoked).
		Where(`"auth_event".metadata->>'reason' = 'session_cap'`).
		OrderExpr(`"auth_event".id DESC`).Limit(1).Scan(ctx))
	assert.Equal(t, authModels.PortalScopeTenant, event.Metadata["portal_scope"])
	assert.Equal(t, float64(1), event.Metadata["revoked_token_count"])
}

// Deliberately NOT parallel: unscoped sweep — CleanupExpiredTokens runs the
// orphan-push and pending-wipe sweeps across every account and tenant, so
// beside a parallel test it deletes that test's unbound push rows and
// tokens (#2419).
func TestCleanupExpiredTokensRetainsPendingWipeReason(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("pending-wipe-reason")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	t.Cleanup(func() {
		testpkg.CleanupAuthFixtures(t, db, account.ID)
		testpkg.CleanupTestTenant(t, db, tenantID)
	})

	_, _, err = service.Login(ctx, email, testPassword)
	require.NoError(t, err)
	_, err = db.NewRaw(`
		INSERT INTO audit.auth_events (tenant_id, account_id, event_type, success, ip_address, metadata, created_at)
		VALUES (?, ?, 'token_revoked', true, '0.0.0.0', jsonb_build_object('reason', 'password_reset', 'pending_account_wide_wipe', true), ?)
	`, tenantID, account.ID, time.Now()).Exec(context.Background())
	require.NoError(t, err)

	_, err = service.CleanupExpiredTokens(ctx)
	require.NoError(t, err)

	var event auditModels.AuthEvent
	require.NoError(t, db.NewSelect().Model(&event).ModelTableExpr(`audit.auth_events AS "auth_event"`).
		Where(`"auth_event".account_id = ?`, account.ID).
		Where(`"auth_event".event_type = ?`, auditModels.EventTypeTokenRevoked).
		Where(`"auth_event".metadata->>'reason' = 'password_reset'`).
		Where(`COALESCE("auth_event".metadata->>'pending_account_wide_wipe', 'false') <> 'true'`).
		OrderExpr(`"auth_event".id DESC`).Limit(1).Scan(ctx))
	assert.Equal(t, "password_reset", event.Metadata["reason"])

	count, err := db.NewSelect().TableExpr("auth.tokens").Where("account_id = ?", account.ID).Count(ctx)
	require.NoError(t, err)
	assert.Zero(t, count)
}
