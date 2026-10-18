package observability_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/observability"
	"github.com/stretchr/testify/require"
)

func TestPublicObservabilityContractPropagatesAndSanitizesCorrelationIDs(t *testing.T) {
	t.Parallel()
	first, err := observability.NewCorrelationID()
	require.NoError(t, err)
	require.NotEmpty(t, first.String())
	preserved, err := observability.CorrelationIDFromRequest(first.String())
	require.NoError(t, err)
	require.Equal(t, first, preserved)
	ctx := observability.WithCorrelationID(context.Background(), preserved)
	actual, found := observability.CorrelationIDFromContext(ctx)
	require.True(t, found)
	require.Equal(t, first, actual)
	sanitized, err := observability.CorrelationIDFromRequest("untrusted header text")
	require.NoError(t, err)
	require.NotEmpty(t, sanitized.String())
	require.NotEqual(t, "untrusted header text", sanitized.String())
	require.NotEqual(t, first, sanitized)
	_, found = observability.CorrelationIDFromContext(context.Background())
	require.False(t, found)
}
