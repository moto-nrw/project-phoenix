package requestreviewhttp_test

import (
	"context"
	"errors"
	"testing"

	reviewbindings "github.com/moto-ogs/project-phoenix/roots/requestreviewbinding"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"

	requestreviewhttp "github.com/moto-ogs/project-phoenix/modules/requestreview/http"
	"github.com/stretchr/testify/require"
)

// principalFacts mirrors Identity & Access's request-review principal field
// for field; the conversion stops compiling when the principal changes shape.
type principalFacts struct {
	Admin        bool
	UsersUpdate  bool
	UsersRead    bool
	UsersAbsence bool
}

func TestRequestReviewIdentityPreservesEffectivePermissions(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		grants []string
		want   principalFacts
	}{
		{"none", nil, principalFacts{}},
		{"write", []string{"users:update"}, principalFacts{UsersUpdate: true}},
		{"absence", []string{"users:absence", "users:read"}, principalFacts{UsersRead: true, UsersAbsence: true}},
		{"resource wildcard", []string{"users:*"}, principalFacts{UsersRead: true, UsersUpdate: true, UsersAbsence: true}},
		{"admin wildcard", []string{"*:*"}, principalFacts{Admin: true, UsersRead: true, UsersUpdate: true, UsersAbsence: true}},
		{"malformed wildcard", []string{"*"}, principalFacts{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), sessiontoken.CtxPermissions, tc.grants)
			require.Equal(t, tc.want, principalFacts(requestreviewhttp.RequestReviewPrincipal(ctx)))
			access, err := reviewbindings.NewAccess(requestreviewhttp.RequestReviewPrincipal, nil)
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
	ctx := context.WithValue(context.Background(), sessiontoken.CtxClaims, sessiontoken.AppClaims{IsAdmin: true})
	require.True(t, requestreviewhttp.RequestReviewPrincipal(ctx).Admin)
	require.False(t, requestreviewhttp.RequestReviewCorrectionAccess(ctx, nil))
	ctx = context.WithValue(ctx, sessiontoken.CtxPermissions, []string{"users:update"})
	require.False(t, requestreviewhttp.RequestReviewCorrectionAccess(ctx, nil))
	require.True(t, requestreviewhttp.RequestReviewCorrectionAccess(ctx, func(context.Context) (bool, error) { return true, nil }))
	ctx = context.WithValue(ctx, sessiontoken.CtxPermissions, []string{"admin:*"})
	require.True(t, requestreviewhttp.RequestReviewCorrectionAccess(ctx, nil))
}

func TestRequestReviewAccessPreservesAbsenceOnlyCapability(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), sessiontoken.CtxPermissions, []string{"users:read", "users:absence"})
	for _, level := range []string{"team", "group_leader", "none", "admin"} {
		access, err := reviewbindings.NewAccess(requestreviewhttp.RequestReviewPrincipal, func(received context.Context) (string, error) {
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
	access, err := reviewbindings.NewAccess(requestreviewhttp.RequestReviewPrincipal, func(context.Context) (string, error) {
		return "", failure
	})
	require.NoError(t, err)
	level, err := access.ReviewAccess(ctx)
	require.ErrorIs(t, err, failure)
	require.Empty(t, level)
}
