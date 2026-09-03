package legacy

import (
	"context"
	"errors"
	"testing"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/stretchr/testify/assert"
)

type recordedSettingsRegistry struct {
	handlers map[string]settingHandler
}

func (r *recordedSettingsRegistry) Register(key string, handler settingHandler) {
	r.handlers[key] = handler
}

func (r *recordedSettingsRegistry) dispatch(ctx context.Context, key string, value any) (func(), error) {
	return r.handlers[key](ctx, 1, value)
}

type recordedSchulhofProvisioner struct {
	called bool
	err    error
}

func (s *recordedSchulhofProvisioner) EnsureInfrastructure(context.Context, int64) (*provisionedActivity, error) {
	s.called = true
	return nil, s.err
}

type recordedWCProvisioner struct {
	called bool
	err    error
}

func (w *recordedWCProvisioner) EnsureInfrastructure(context.Context) (*provisionedActivity, error) {
	w.called = true
	return nil, w.err
}

func TestRegisterSettingsSideEffectsPreservesProvisioningContracts(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("provisioning failed")
	tests := []struct {
		name         string
		key          string
		value        any
		schulhofErr  error
		wcErr        error
		wantSchulhof bool
		wantWC       bool
		wantErr      error
	}{
		{name: "Schulhof enabled", key: configModels.KeyCheckoutSchulhofEnabled, value: true, wantSchulhof: true},
		{name: "Schulhof disabled", key: configModels.KeyCheckoutSchulhofEnabled, value: false},
		{name: "Schulhof malformed", key: configModels.KeyCheckoutSchulhofEnabled, value: "true"},
		{name: "Schulhof error", key: configModels.KeyCheckoutSchulhofEnabled, value: true, schulhofErr: wantErr, wantSchulhof: true, wantErr: wantErr},
		{name: "WC enabled", key: configModels.KeyCheckoutWCEnabled, value: true, wantWC: true},
		{name: "WC disabled", key: configModels.KeyCheckoutWCEnabled, value: false},
		{name: "WC malformed", key: configModels.KeyCheckoutWCEnabled, value: "true"},
		{name: "WC error", key: configModels.KeyCheckoutWCEnabled, value: true, wcErr: wantErr, wantWC: true, wantErr: wantErr},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registry := &recordedSettingsRegistry{handlers: make(map[string]settingHandler)}
			schulhof := &recordedSchulhofProvisioner{err: test.schulhofErr}
			wc := &recordedWCProvisioner{err: test.wcErr}
			RegisterSettingsSideEffects(registry, schulhof, wc)

			postCommit, err := registry.dispatch(context.Background(), test.key, test.value)

			assert.Nil(t, postCommit)
			assert.ErrorIs(t, err, test.wantErr)
			assert.Equal(t, test.wantSchulhof, schulhof.called)
			assert.Equal(t, test.wantWC, wc.called)
		})
	}
}
