package api

// The registry routing of the student-photo settings key is tested where the
// root composition wires it; the lifecycle behind it lives with the People
// Directory owner (#3349).

import (
	"context"
	"sync/atomic"
	"testing"

	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeStudentPhotoService struct {
	calls int32
}

func (f *fakeStudentPhotoService) HandleFeatureToggle(_ context.Context, _ int64, _ any) (func(), error) {
	atomic.AddInt32(&f.calls, 1)
	return func() {}, nil
}

func TestRegisterStudentPhotoSettingsSideEffectsDispatchesToTheService(t *testing.T) {
	t.Parallel()

	service := &fakeStudentPhotoService{}

	registry, err := settingsCompose.NewSideEffects(func(string) {},
		settingsCompose.SettingSideEffect{Key: configModel.KeyStudentPhotosEnabled, Handle: service.HandleFeatureToggle})
	require.NoError(t, err)

	post, err := registry.Dispatch(context.Background(), 1, "operations.student_photos_enabled", true)
	require.NoError(t, err)
	require.NotNil(t, post)
	assert.Equal(t, int32(1), atomic.LoadInt32(&service.calls),
		"registry must route the photo feature key to the service")
}

func TestRegisterStudentPhotoSettingsSideEffectsIgnoresOtherKeys(t *testing.T) {
	t.Parallel()

	service := &fakeStudentPhotoService{}

	registry, err := settingsCompose.NewSideEffects(func(string) {},
		settingsCompose.SettingSideEffect{Key: configModel.KeyStudentPhotosEnabled, Handle: service.HandleFeatureToggle})
	require.NoError(t, err)

	post, err := registry.Dispatch(context.Background(), 1, "operations.session_end_time", "18:00")
	require.NoError(t, err)
	assert.Nil(t, post, "unrelated keys must return a nil post-commit callback")
	assert.Equal(t, int32(0), atomic.LoadInt32(&service.calls),
		"only the photos key may invoke the photo handler")
}
