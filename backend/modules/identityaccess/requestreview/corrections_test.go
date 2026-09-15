package requestreview

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCorrectionAccessUsesWildcardOrVerifiedStaff(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	require.True(t, CorrectionsAllowed(ctx, true, nil))
	require.False(t, CorrectionsAllowed(ctx, false, nil))
	require.True(t, CorrectionsAllowed(ctx, false, func(context.Context) (bool, error) { return true, nil }))
	require.False(t, CorrectionsAllowed(ctx, false, func(context.Context) (bool, error) { return false, nil }))
	require.False(t, CorrectionsAllowed(ctx, false, func(context.Context) (bool, error) { return true, errors.New("staff unavailable") }))
}
