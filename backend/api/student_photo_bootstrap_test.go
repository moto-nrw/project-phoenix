package api

import (
	"context"
	"log/slog"
	"testing"

	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	"github.com/stretchr/testify/require"
)

// The photo lifecycle the root builds answers the feature toggle the settings
// registry dispatches (#4007).
func TestStudentPhotosRegisterTheFeatureToggle(t *testing.T) {
	t.Parallel()

	registry := settingsCompose.NewSideEffectRegistry()
	photos := NewStudentPhotos(nil, StudentPhotoRuntimeDependencies{Logger: slog.Default()})
	RegisterStudentPhotoSettingsSideEffects(registry, photos)
	post, err := registry.Dispatch(context.Background(), 1, "operations.student_photos_enabled", "x")
	require.NoError(t, err)
	require.NotNil(t, post)
}
