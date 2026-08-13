package auth_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/rotation"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingAuthEventRepository struct {
	auditModels.AuthEventRepository
}

func (failingAuthEventRepository) Create(context.Context, *auditModels.AuthEvent) error {
	return errors.New("forced audit failure")
}

func TestLogoutPersistsRevocationAuditWithoutRawFamilyID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
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

func TestRevocationRollsBackWhenAuditInsertFails(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
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
	repoFactory.AuthEvent = failingAuthEventRepository{AuthEventRepository: repoFactory.AuthEvent}
	serviceFactory, err := services.NewFactory(repoFactory, db, slog.Default())
	require.NoError(t, err)

	err = serviceFactory.Auth.RevokeAllTokensWithReason(tenant.WithTenantID(context.Background(), tenantID), int(account.ID), "password_reset")
	require.ErrorContains(t, err, "forced audit failure")

	count, err := db.NewSelect().TableExpr("auth.tokens").Where("account_id = ?", account.ID).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "token deletion must roll back with the failed audit insert")
}

func TestSessionCapAuditsEvictedTokenFamily(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
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
