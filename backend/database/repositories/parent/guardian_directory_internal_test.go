package parent

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestParentReadsFailClosedWithoutMembershipQuery pins that a parent
// repository composed without the Identity & Access membership query (#2721)
// reports an error before any read instead of widening the school scope.
func TestParentReadsFailClosedWithoutMembershipQuery(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	accountID := int64(len(t.Name()))

	_, err := NewChildRepository(nil).ListByAccount(ctx, accountID)
	require.ErrorIs(t, err, errMembershipQueryRequired)

	phases := &EnrollablePhaseRepository{}
	_, err = phases.GuardianSubmitStatus(ctx, accountID, accountID)
	require.ErrorIs(t, err, errMembershipQueryRequired)
}

func TestParentReadsFailClosedOnMembershipFailure(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	accountID := int64(len(t.Name()))
	failure := errors.New("identity membership unavailable")
	memberships := func(gotCtx context.Context, gotID int64) ([]int64, error) {
		require.Equal(t, ctx, gotCtx)
		require.Equal(t, accountID, gotID)
		return nil, failure
	}
	children, err := NewChildRepository(memberships).ListByAccount(ctx, accountID)
	require.ErrorIs(t, err, failure)
	require.Nil(t, children)
	phases := &EnrollablePhaseRepository{memberships: memberships}
	status, err := phases.GuardianSubmitStatus(ctx, accountID, accountID)
	require.ErrorIs(t, err, failure)
	require.Nil(t, status)
}
