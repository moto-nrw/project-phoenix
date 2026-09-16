package application

import (
	"context"
	"testing"

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

	access, refresh, err := f.auth.LoginWithAudit(ctx, "user1@example.com", "secret", "192.0.2.1", "tests", "")
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

	_, rotated, err := f.auth.RefreshTokenWithAudit(ctx, refresh, "192.0.2.1", "tests")
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

	_, _, err := f.auth.LoginWithAudit(context.Background(), "user1@example.com", "wrong", "192.0.2.1", "tests", "")
	require.ErrorIs(t, err, domain.ErrInvalidCredentials)
	assert.Empty(t, f.store.sessionsOf(1))
	events := f.audit.eventsOfType(domain.AuthEventLogin)
	require.Len(t, events, 1)
	assert.False(t, events[0].Success)
}

func TestLoginWithAudit_GuardianOnlyAccountMustUseParentPortal(t *testing.T) {
	t.Parallel()
	f := newAuthFixture(t)
	f.seedStaff(1, 10, "guardian")

	_, _, err := f.auth.LoginWithAudit(context.Background(), "user1@example.com", "secret", "192.0.2.1", "tests", "")
	require.ErrorIs(t, err, domain.ErrParentMustUseParentPortal, "wrong-portal rejection")
	assert.Empty(t, f.store.sessionsOf(1))
}
