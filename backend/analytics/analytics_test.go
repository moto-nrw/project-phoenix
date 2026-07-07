package analytics

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewNoopIsSafe(t *testing.T) {
	tracker := NewNoop()
	tracker.Capture("school:x", "some_event", map[string]any{"key": "value"})
	require.NoError(t, tracker.Close())
}

func TestNewWithoutAPIKeyReturnsNoop(t *testing.T) {
	tracker, err := New("", "", nil)
	require.NoError(t, err)
	require.IsType(t, noopTracker{}, tracker)
}

func TestNewWithAPIKeyButNoHostFails(t *testing.T) {
	tracker, err := New("phc_test", "", nil)
	require.Error(t, err)
	require.Nil(t, tracker)
}

func TestNewWithAPIKeyAndHost(t *testing.T) {
	tracker, err := New("phc_test", "https://eu.i.posthog.com", nil)
	require.NoError(t, err)
	require.NotNil(t, tracker)
	require.NoError(t, tracker.Close())
}
