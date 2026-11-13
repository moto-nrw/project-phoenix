package behavior_test

import (
	"context"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A session commits only with its authentication event (#6844). When the
// ledger refuses the entry, the login issues no session and the refresh
// keeps the presented token valid: both roll back with the failed entry.
func TestSessionAuditFailureRollsBackTheSession(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	service := setupAuthService(t, db)
	tenantID, _ := newSchoolTenant(t, db)
	email, accountID := createLehrkraftAccount(t, db, service, "session-audit", tenantID)
	sessions := func() int64 {
		count, err := db.NewSelect().TableExpr("auth.tokens").Where("account_id = ?", accountID).Count(ctx)
		require.NoError(t, err)
		return count
	}
	rejectEvent := func(eventType string) func() {
		_, err := db.ExecContext(ctx, `ALTER TABLE audit.auth_events ADD CONSTRAINT reject_session_audit_test CHECK (event_type <> ?)`, eventType)
		require.NoError(t, err)
		return func() {
			_, err := db.ExecContext(ctx, `ALTER TABLE audit.auth_events DROP CONSTRAINT reject_session_audit_test`)
			require.NoError(t, err)
		}
	}

	accept := rejectEvent("login")
	_, err := service.LoginSchoolWithMFAGate(ctx, email, testPassword, switchIP, switchUserAgent, "")
	require.Error(t, err, "a login whose event cannot be written fails")
	assert.Zero(t, sessions(), "the session rolls back with its event")
	accept()

	login, err := service.LoginSchoolWithMFAGate(ctx, email, testPassword, switchIP, switchUserAgent, "")
	require.NoError(t, err)
	require.Equal(t, identityaccess.LoginStatusAuthenticated, login.Status)
	requireAuthEventEventually(t, db, accountID, tenantID, "login")
	before := sessions()

	accept = rejectEvent("token_refresh")
	_, _, err = service.RefreshTokenWithAudit(ctx, login.RefreshToken, "", switchIP, switchUserAgent)
	require.Error(t, err, "a refresh whose event cannot be written fails")
	assert.Equal(t, before, sessions(), "the rotation rolls back with its event")
	accept()

	_, renewed, err := service.RefreshTokenWithAudit(ctx, login.RefreshToken, "", switchIP, switchUserAgent)
	require.NoError(t, err, "the presented refresh token stays valid after the failed rotation")
	assert.NotEmpty(t, renewed)
}
