package facilities

import (
	"context"
	"errors"
	"testing"

	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"

	"github.com/stretchr/testify/assert"
)

// The Settings Platform registry keys the side effects register under. The
// literals pin the registry contract the production registration names.
const (
	checkoutSchulhofEnabledKey = "checkout.schulhof_enabled"
	checkoutWCEnabledKey       = "checkout.wc_enabled"
)

// recordedSettingsRegistry is the registry double: the handlers bound
// under their keys.
type recordedSettingsRegistry map[string]facilitiesModule.SettingHandler

func (r recordedSettingsRegistry) dispatch(ctx context.Context, key string, value any) (func(), error) {
	return r[key](ctx, 1, value)
}

type recordedSchulhofProvisioner struct {
	called bool
	err    error
}

func (s *recordedSchulhofProvisioner) EnsureInfrastructure(context.Context, int64) (*facilitiesModule.SystemActivity, error) {
	s.called = true
	return nil, s.err
}

type recordedWCProvisioner struct {
	called bool
	err    error
}

func (w *recordedWCProvisioner) EnsureInfrastructure(context.Context) (*facilitiesModule.SystemActivity, error) {
	w.called = true
	return nil, w.err
}

func TestSettingHandlersPreserveProvisioningContracts(t *testing.T) {
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
		{name: "Schulhof enabled", key: checkoutSchulhofEnabledKey, value: true, wantSchulhof: true},
		{name: "Schulhof disabled", key: checkoutSchulhofEnabledKey, value: false},
		{name: "Schulhof malformed", key: checkoutSchulhofEnabledKey, value: "true"},
		{name: "Schulhof error", key: checkoutSchulhofEnabledKey, value: true, schulhofErr: wantErr, wantSchulhof: true, wantErr: wantErr},
		{name: "WC enabled", key: checkoutWCEnabledKey, value: true, wantWC: true},
		{name: "WC disabled", key: checkoutWCEnabledKey, value: false},
		{name: "WC malformed", key: checkoutWCEnabledKey, value: "true"},
		{name: "WC error", key: checkoutWCEnabledKey, value: true, wcErr: wantErr, wantWC: true, wantErr: wantErr},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			schulhof := &recordedSchulhofProvisioner{err: test.schulhofErr}
			wc := &recordedWCProvisioner{err: test.wcErr}
			schulhofHandler, wcHandler := SettingHandlers(schulhof, wc)
			registry := recordedSettingsRegistry{checkoutSchulhofEnabledKey: schulhofHandler, checkoutWCEnabledKey: wcHandler}

			postCommit, err := registry.dispatch(context.Background(), test.key, test.value)

			assert.Nil(t, postCommit)
			assert.ErrorIs(t, err, test.wantErr)
			assert.Equal(t, test.wantSchulhof, schulhof.called)
			assert.Equal(t, test.wantWC, wc.called)
		})
	}
}
