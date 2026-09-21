package application

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The window slides (#3466): a request counts for one hour after it was made.
func TestDemoRequestWindowSlidesOverTheHour(t *testing.T) {
	t.Parallel()
	window := newDemoRequestWindow(3)
	start := time.Date(2026, 9, 30, 9, 0, 0, 0, time.UTC)

	require.NoError(t, window.admit("kim@ogs.de", start))
	require.NoError(t, window.admit("kim@ogs.de", start.Add(10*time.Minute)))
	require.NoError(t, window.admit("kim@ogs.de", start.Add(20*time.Minute)))

	err := window.admit("kim@ogs.de", start.Add(30*time.Minute))
	var limited *domain.DemoAccessRateLimitedError
	require.ErrorAs(t, err, &limited)
	assert.Equal(t, start.Add(time.Hour), limited.RetryAt, "the oldest request leaves the window first")
	require.NoError(t, window.admit("other@ogs.de", start.Add(30*time.Minute)), "each key has its own window")

	require.ErrorAs(t, window.admit("kim@ogs.de", start.Add(time.Hour-time.Second)), &limited, "a request counts for a full hour")
	require.NoError(t, window.admit("kim@ogs.de", start.Add(time.Hour)), "at RetryAt the next request is admitted")
	require.ErrorAs(t, window.admit("kim@ogs.de", start.Add(time.Hour+time.Second)), &limited,
		"rejected requests were not counted, the admitted one was")
	assert.Equal(t, start.Add(70*time.Minute), limited.RetryAt)
}

func TestDemoRequestWindowForgetsIdleKeys(t *testing.T) {
	t.Parallel()
	window := newDemoRequestWindow(3)
	start := time.Date(2026, 9, 30, 9, 0, 0, 0, time.UTC)
	require.NoError(t, window.admit("kim@ogs.de", start))

	require.NoError(t, window.admit("other@ogs.de", start.Add(2*time.Hour)))
	assert.NotContains(t, window.seen, "kim@ogs.de")
	assert.Contains(t, window.seen, "other@ogs.de")
}
