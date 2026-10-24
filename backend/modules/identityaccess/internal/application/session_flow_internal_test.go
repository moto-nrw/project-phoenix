package application

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The tenant-portal session lifecycle over the module's ports: a login
// mints a session and its token pair, a refresh rotates the session, a
// logout revokes the family. Each step leaves its audit evidence.
func TestTenantSessionLifecycle(t *testing.T) {
	t.Parallel()
	f := newAuthFixture(t)
	f.seedStaff(1, 10, "admin")
	f.store.permissions[mappingKey{1, 10}] = []string{"students:read"}
	f.persons.names[personKey{1, 10}] = [2]string{"Ada", "Lovelace"}
	ctx := context.Background()

	access, refresh, err := f.auth.loginWithAudit(ctx, "user1@example.com", "secret", "192.0.2.1", "tests", "")
	require.NoError(t, err)

	accessClaims := decodeAccess(t, access)
	assert.Equal(t, int64(1), accessClaims.AccountID)
	assert.Equal(t, int64(10), accessClaims.TenantID)
	assert.Equal(t, int64(100), accessClaims.OrgID)
	assert.Equal(t, domain.ScopeTenant, accessClaims.Scope)
	assert.Equal(t, []string{"admin"}, accessClaims.Roles)
	assert.Equal(t, []string{"students:read"}, accessClaims.Permissions)
	assert.Equal(t, "Ada", accessClaims.FirstName)
	assert.True(t, accessClaims.IsAdmin)
	assert.NotEmpty(t, accessClaims.FamilyID)

	refreshClaims := decodeRefresh(t, refresh)
	assert.Equal(t, int64(1), refreshClaims.AccountID)
	assert.Equal(t, int64(10), refreshClaims.TenantID)

	sessions := f.store.sessionsOf(1)
	require.Len(t, sessions, 1)
	assert.Equal(t, refreshClaims.Token, sessions[0].Token)
	assert.Equal(t, int64(10), sessions[0].TenantID, "the tenant comes from the database resolution, not the context")
	assert.Equal(t, domain.PortalScopeTenant, sessions[0].PortalScope)
	assert.Equal(t, accessClaims.FamilyID, sessions[0].FamilyID)
	require.Len(t, f.audit.eventsOfType(domain.AuthEventLogin), 1)
	assert.True(t, f.audit.eventsOfType(domain.AuthEventLogin)[0].Success)

	_, rotated, err := f.auth.refreshTokenWithAudit(ctx, refresh, "192.0.2.1", "tests")
	require.NoError(t, err)
	rotatedClaims := decodeRefresh(t, rotated)
	assert.NotEqual(t, refreshClaims.Token, rotatedClaims.Token, "a refresh rotates the session handle")
	require.Len(t, f.audit.eventsOfType(domain.AuthEventTokenRefresh), 1)

	require.NoError(t, f.auth.LogoutWithAudit(ctx, rotated, "192.0.2.1", "tests"))
	assert.Empty(t, f.store.sessionsOf(1), "logout revokes the presented family")
	require.Len(t, f.audit.eventsOfType(domain.AuthEventTokenRevoked), 1)
	assert.Equal(t, int64(10), f.audit.eventsOfType(domain.AuthEventTokenRevoked)[0].TenantID)
}

func TestLoginWithAudit_WrongPasswordIsInvalidCredentials(t *testing.T) {
	t.Parallel()
	f := newAuthFixture(t)
	f.seedStaff(1, 10, "user")

	_, _, err := f.auth.loginWithAudit(context.Background(), "user1@example.com", "wrong", "192.0.2.1", "tests", "")
	require.ErrorIs(t, err, identityaccess.ErrInvalidCredentials)
	assert.Empty(t, f.store.sessionsOf(1))
	events := f.audit.eventsOfType(domain.AuthEventLogin)
	require.Len(t, events, 1)
	assert.False(t, events[0].Success)
}

func TestLoginWithAudit_GuardianOnlyAccountMustUseParentPortal(t *testing.T) {
	t.Parallel()
	f := newAuthFixture(t)
	f.seedStaff(1, 10, "guardian")

	_, _, err := f.auth.loginWithAudit(context.Background(), "user1@example.com", "secret", "192.0.2.1", "tests", "")
	require.ErrorIs(t, err, identityaccess.ErrParentMustUseParentPortal, "wrong-portal rejection")
	assert.Empty(t, f.store.sessionsOf(1))
}

// A rejected refresh commits its family revocation, but never continues into
// successor creation, login recording, or token minting.
func TestRefreshToken_RejectionsRevokeFamilyWithoutIssuingTokens(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		want    error
		prepare func(*authFixture, *domain.AccountSession)
	}{
		{
			name: "claim_mismatch", want: identityaccess.ErrInvalidToken,
			prepare: func(_ *authFixture, session *domain.AccountSession) { session.TenantID++ },
		},
		{
			name: "token_expired", want: identityaccess.ErrTokenExpired,
			prepare: func(_ *authFixture, session *domain.AccountSession) { session.Expiry = time.Now().Add(-time.Minute) },
		},
		{
			name: "replay_detected", want: identityaccess.ErrInvalidToken,
			prepare: func(_ *authFixture, session *domain.AccountSession) {
				rotatedAt := time.Now().Add(-fakeRotation{}.RecoveryGrace() - time.Minute)
				session.RotatedAt = &rotatedAt
			},
		},
		{
			name: "lineage_mismatch", want: identityaccess.ErrInvalidToken,
			prepare: func(f *authFixture, session *domain.AccountSession) {
				successor := *session
				successor.ID++
				successor.Token += "-successor"
				successor.Generation++
				f.store.sessions[successor.ID] = successor
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newAuthFixture(t)
			f.seedStaff(1, 10, "user")
			ctx := context.Background()
			_, refresh, err := f.auth.loginWithAudit(ctx, "user1@example.com", "secret", "", "tests", "")
			require.NoError(t, err)
			sessions := f.store.sessionsOf(1)
			require.Len(t, sessions, 1)
			presented := sessions[0]
			tt.prepare(f, &presented)
			f.store.sessions[presented.ID] = presented
			f.store.calls = nil

			access, rotated, err := f.auth.refreshTokenWithAudit(ctx, refresh, "", "tests")

			require.ErrorIs(t, err, tt.want)
			assert.Empty(t, access)
			assert.Empty(t, rotated)
			assert.Empty(t, f.store.sessionsOf(1))
			events := f.audit.eventsOfType(domain.AuthEventTokenRevoked)
			require.Len(t, events, 1)
			assert.Equal(t, tt.name, events[0].RevokedSessions.Reason)
			assert.Contains(t, f.push.calls, fmt.Sprintf("family:1:%s", presented.FamilyID))
			assert.NotContains(t, f.store.recorded(), "RecordAccountLogin")
			assert.NotContains(t, f.store.recorded(), "InsertAccountSession")
		})
	}
}

func TestRefreshRequiresMFAAfterPrivilegeGrant(t *testing.T) {
	t.Parallel()
	f := newAuthFixture(t)
	f.seedStaff(1, 10, "custom")
	ctx := context.Background()
	_, refresh, err := f.auth.loginWithAudit(ctx, "user1@example.com", "secret", "", "tests", "")
	require.NoError(t, err)
	f.mfa.configured = true
	f.mfa.policyInTx = domain.MFAPolicyForMode(domain.MFAModeOff)
	f.store.permissions[mappingKey{1, 10}] = []string{"student_documents:health"}
	access, successor, err := f.auth.refreshTokenWithAudit(ctx, refresh, "", "tests")
	require.ErrorIs(t, err, identityaccess.ErrInvalidToken)
	assert.Empty(t, access)
	assert.Empty(t, successor)
	sessions := f.store.sessionsOf(1)
	require.Len(t, sessions, 1)
	assert.Nil(t, sessions[0].RotatedAt, "MFA rejection must not rotate the old session")
}
