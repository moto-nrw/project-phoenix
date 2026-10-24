package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDemoSeedAdapterUsesInternalCodeReaderWithoutDiskCache(t *testing.T) {
	t.Parallel()
	adapter := demoSeedAdapterFactory(func(context.Context, string, time.Time) (string, error) {
		return "example-code", nil
	})("http://127.0.0.1:8080", false)
	require.Nil(t, adapter.inner.tenantSessions, "the demo sidecar's filesystem is read-only")
	code, err := adapter.inner.fetchLatestMFACode(context.Background(), "demo@example.test", time.Now())
	require.NoError(t, err)
	require.Equal(t, "example-code", code)
}
