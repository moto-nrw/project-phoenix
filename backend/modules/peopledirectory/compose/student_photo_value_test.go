package compose

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The photo runtime passed as a value serves the photo lifecycle directly;
// passing it together with the resolver is refused.
func TestStudentPhotoPort_TakesTheRuntimeValue(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	runtime := &enabledPhotoRuntime{}
	port := studentPhotoPort(Dependencies{StudentPhotos: runtime})
	enabled, err := port.PhotoFeatureEnabled(ctx)
	require.NoError(t, err)
	require.True(t, enabled)
	port.UnlinkStoredPhoto(t.Name())
	require.Equal(t, []string{t.Name()}, runtime.unlinked)

	err = checkDependencies(Dependencies{
		DB: testpkg.SetupTestDB(t), Observe: func(Observation) {}, StudentDirectory: testpkg.StudentDirectory{},
		StudentPhotos: runtime, StudentPhotoRuntime: func() StudentPhotoRuntime { return runtime },
	})
	require.ErrorContains(t, err, "exclusive")
}

// The disabled runtime behaves like the unbound resolver a root without
// photos composed with before: the feature reports disabled and nothing is
// unlinked or acted for.
func TestDisabledStudentPhotos_MatchesTheUnboundRuntime(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	disabled := studentPhotoPort(Dependencies{StudentPhotos: DisabledStudentPhotos()})
	unbound := studentPhotoPort(Dependencies{})
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
	require.ErrorIs(t, err, errStudentPhotoRuntimeUnbound)
	require.EqualError(t, err, "people directory: student photo runtime is not bound")
}
