package compose_test

import (
	"context"
	"errors"
	"testing"

	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSettingsRequiresEveryDependency(t *testing.T) {
	t.Parallel()
	_, err := settingsCompose.NewSettings(settingsCompose.SettingsDependencies{})
	require.Error(t, err)
}

func TestNewSideEffectsDispatchesTheEffectsItWasBuiltWith(t *testing.T) {
	t.Parallel()
	var failed []string
	errEffect := errors.New("effect failed")
	registry, err := settingsCompose.NewSideEffects(func(key string) { failed = append(failed, key) },
		settingsCompose.SettingSideEffect{Key: "operations.fails", Handle: func(context.Context, int64, any) (func(), error) {
			return nil, errEffect
		}})
	require.NoError(t, err)
	_, err = registry.Dispatch(context.Background(), 7, "operations.fails", true)
	require.ErrorIs(t, err, errEffect)
	assert.Equal(t, []string{"operations.fails"}, failed)

	_, err = settingsCompose.NewSideEffects(nil)
	require.Error(t, err, "the failure observer is required")
}
