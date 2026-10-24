package compose

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The photo runtime passed as a value serves the photo lifecycle directly.
func TestStudentPhotoPort_TakesTheRuntimeValue(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	runtime := &enabledPhotoRuntime{}
	port := studentPhotoPort(runtime)
	enabled, err := port.PhotoFeatureEnabled(ctx)
	require.NoError(t, err)
	require.True(t, enabled)
	port.UnlinkStoredPhoto(t.Name())
	require.Equal(t, []string{t.Name()}, runtime.unlinked)
}

// The disabled runtime behaves like a directory composed without a photo
// runtime: the feature reports disabled and nothing is
// unlinked or acted for.
func TestDisabledStudentPhotos_MatchesTheUnboundRuntime(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	disabled := studentPhotoPort(DisabledStudentPhotos())
	unbound := studentPhotoPort(nil)
	for _, port := range []interface {
		PhotoFeatureEnabled(context.Context) (bool, error)
		ActingAccountID(context.Context) int64
	}{disabled, unbound} {
		enabled, err := port.PhotoFeatureEnabled(ctx)
		require.NoError(t, err)
		require.False(t, enabled)
		require.Zero(t, port.ActingAccountID(ctx))
	}

	// A consent change fails with the error the unbound runtime reports.
	err := DisabledStudentPhotos().RecordPhotoConsent(ctx, StudentPhotoConsentSnapshot{}, StudentPhotoConsentSnapshot{}, nil, time.Now())
	require.ErrorIs(t, err, studentPhotoRuntimeUnboundForTest())
	require.EqualError(t, err, "people directory: student photo runtime is not bound")
}
