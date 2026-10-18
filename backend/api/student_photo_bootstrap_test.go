package api

import (
	"context"
	"log/slog"
	"testing"

	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/stretchr/testify/require"
)

// The photo lifecycle the root builds answers the feature toggle the settings
// registry dispatches (#4007).
func TestStudentPhotosRegisterTheFeatureToggle(t *testing.T) {
	t.Parallel()

	photos := NewStudentPhotos(nil, StudentPhotoRuntimeDependencies{Logger: slog.Default()})
	registry, err := settingsCompose.NewSideEffects(func(string) {},
		settingsCompose.SettingSideEffect{Key: configModel.KeyStudentPhotosEnabled, Handle: photos.HandleFeatureToggle})
	require.NoError(t, err)
	post, err := registry.Dispatch(context.Background(), 1, "operations.student_photos_enabled", "x")
	require.NoError(t, err)
	require.NotNil(t, post)
}
