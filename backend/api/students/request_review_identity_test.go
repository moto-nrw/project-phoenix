package students

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	reviewidentity "github.com/moto-nrw/project-phoenix/modules/identityaccess/requestreview"
	reviewcompose "github.com/moto-nrw/project-phoenix/modules/requestreview/compose"
	"github.com/stretchr/testify/require"
)

func TestRequestReviewIdentityPreservesEffectivePermissions(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		grants []string
		want   reviewidentity.Principal
	}{
		{"none", nil, reviewidentity.Principal{}},
		{"write", []string{"users:update"}, reviewidentity.Principal{UsersUpdate: true}},
		{"absence", []string{"users:absence", "users:read"}, reviewidentity.Principal{UsersRead: true, UsersAbsence: true}},
		{"resource wildcard", []string{"users:*"}, reviewidentity.Principal{UsersRead: true, UsersUpdate: true, UsersAbsence: true}},
		{"admin wildcard", []string{"*:*"}, reviewidentity.Principal{Admin: true, UsersRead: true, UsersUpdate: true, UsersAbsence: true}},
		{"malformed wildcard", []string{"*"}, reviewidentity.Principal{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), jwt.CtxPermissions, tc.grants)
			require.Equal(t, tc.want, RequestReviewPrincipal(ctx))
			access, err := reviewcompose.NewAccess(RequestReviewPrincipal, nil)
			require.NoError(t, err)
			caller, err := access.Caller(ctx)
			require.NoError(t, err)
			require.Equal(t, tc.want.UsersUpdate, caller.ReviewsWriteQueues)
			level, err := access.ReviewAccess(ctx)
			require.NoError(t, err)
			require.Empty(t, level, "an unwired optional policy must omit review_access")
		})
	}
}

func TestCorrectionAccessDoesNotTreatAdminClaimAsWildcard(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), jwt.CtxClaims, jwt.AppClaims{IsAdmin: true})
	require.True(t, RequestReviewPrincipal(ctx).Admin)
	require.False(t, RequestReviewCorrectionAccess(ctx, nil))
	ctx = context.WithValue(ctx, jwt.CtxPermissions, []string{"users:update"})
	require.False(t, RequestReviewCorrectionAccess(ctx, nil))
	require.True(t, RequestReviewCorrectionAccess(ctx, func(context.Context) (bool, error) { return true, nil }))
	ctx = context.WithValue(ctx, jwt.CtxPermissions, []string{"admin:*"})
	require.True(t, RequestReviewCorrectionAccess(ctx, nil))
}

func TestRequestReviewAccessPreservesAbsenceOnlyCapability(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), jwt.CtxPermissions, []string{"users:read", "users:absence"})
	for _, level := range []string{"team", "group_leader", "none", "admin"} {
		access, err := reviewcompose.NewAccess(RequestReviewPrincipal, func(received context.Context) (string, error) {
			require.Equal(t, ctx, received)
			return level, nil
		})
		require.NoError(t, err)
		got, err := access.ReviewAccess(ctx)
		require.NoError(t, err)
		require.Equal(t, level, got)
		caller, err := access.Caller(ctx)
		require.NoError(t, err)
		require.False(t, caller.ReviewsWriteQueues, "absence review must not grant access to other queues")
	}
	failure := errors.New("review setting unavailable")
	access, err := reviewcompose.NewAccess(RequestReviewPrincipal, func(context.Context) (string, error) {
		return "", failure
	})
	require.NoError(t, err)
	level, err := access.ReviewAccess(ctx)
	require.ErrorIs(t, err, failure)
	require.Empty(t, level)
}
