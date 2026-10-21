package api

import (
	"context"
	"testing"
	"time"

	"github.com/go-chi/jwtauth/v5"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithSSETokenDeadlineAppliesAccessTokenExpiry(t *testing.T) {
	t.Parallel()

	expiresAt := time.Now().Add(time.Hour)
	token, _, err := testutil.TestTokenAuth(t).JwtAuth.Encode(map[string]any{"exp": expiresAt})
	require.NoError(t, err)

	ctx := jwtauth.NewContext(context.Background(), token, nil)
	deadlineCtx, cancel, ok := withSSETokenDeadline(ctx)
	require.True(t, ok)
	defer cancel()

	deadline, hasDeadline := deadlineCtx.Deadline()
	require.True(t, hasDeadline)
	assert.WithinDuration(t, expiresAt, deadline, time.Millisecond)
}

func TestWithSSETokenDeadlineRejectsTokenWithoutExpiry(t *testing.T) {
	t.Parallel()

	token, _, err := testutil.TestTokenAuth(t).JwtAuth.Encode(map[string]any{})
	require.NoError(t, err)

	ctx := jwtauth.NewContext(context.Background(), token, nil)
	_, cancel, ok := withSSETokenDeadline(ctx)
	defer cancel()

	assert.False(t, ok)
}

func TestPrincipalAdminScope(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		isAdmin     bool
		permissions []string
		want        bool
	}{
		{name: "literal admin", isAdmin: true, want: true},
		{name: "admin wildcard", permissions: []string{"admin:*"}, want: true},
		{name: "full wildcard", permissions: []string{"*:*"}, want: true},
		{name: "scoped caregiver", permissions: []string{"users:read"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			principal, err := permissions.NewPrincipal(permissions.PrincipalInput{AccountID: 1, TenantID: 2, Admin: tt.isAdmin, Permissions: tt.permissions})
			require.NoError(t, err)
			assert.Equal(t, tt.want, principal.HasAdminScope())
		})
	}
}
