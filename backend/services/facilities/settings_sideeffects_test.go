package facilities

import (
	"context"
	"errors"
	"testing"

	activityModels "github.com/moto-nrw/project-phoenix/models/activities"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services/config/sideeffects"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSchulhofService records EnsureInfrastructure calls; the other
// SchulhofService methods are unused by the side-effect handler so they
// stub out as panics to surface accidental use.
type fakeSchulhofService struct {
	called    bool
	createdBy int64
	err       error
}

func (f *fakeSchulhofService) EnsureInfrastructure(_ context.Context, createdBy int64) (*activityModels.Group, error) {
	f.called = true
	f.createdBy = createdBy
	return nil, f.err
}

func (f *fakeSchulhofService) GetSchulhofStatus(_ context.Context, _ int64) (*SchulhofStatus, error) {
	panic("unused")
}

type fakeWCService struct {
	called bool
	err    error
}

func (f *fakeWCService) EnsureInfrastructure(_ context.Context) (*activityModels.Group, error) {
	f.called = true
	return nil, f.err
}

// TestRegisterSettingsSideEffects_SchulhofTrueProvisions covers the
// happy-path: flipping checkout.schulhof_enabled to true must call the
// service that creates the system room.
func TestRegisterSettingsSideEffects_SchulhofTrueProvisions(t *testing.T) {
	registry := sideeffects.NewRegistry()
	schulhof := &fakeSchulhofService{}
	wc := &fakeWCService{}

	RegisterSettingsSideEffects(registry, schulhof, wc)

	cb, err := registry.Dispatch(context.Background(), 1, configModel.KeyCheckoutSchulhofEnabled, true)
	require.NoError(t, err)
	assert.Nil(t, cb, "schulhof handler must not schedule post-commit work")
	assert.True(t, schulhof.called)
	assert.False(t, wc.called, "WC handler must not run when only the schulhof key flips")
}

// TestRegisterSettingsSideEffects_SchulhofFalseSkipsProvision pins the
// "only on enable" guard. Disabling must NOT trigger room provisioning.
func TestRegisterSettingsSideEffects_SchulhofFalseSkipsProvision(t *testing.T) {
	registry := sideeffects.NewRegistry()
	schulhof := &fakeSchulhofService{}
	RegisterSettingsSideEffects(registry, schulhof, &fakeWCService{})

	cb, err := registry.Dispatch(context.Background(), 1, configModel.KeyCheckoutSchulhofEnabled, false)
	assert.NoError(t, err)
	assert.Nil(t, cb)
	assert.False(t, schulhof.called)
}

// TestRegisterSettingsSideEffects_SchulhofNonBoolSkipsProvision covers
// the type-assertion guard. Garbage data from a malformed write must not
// panic the handler.
func TestRegisterSettingsSideEffects_SchulhofNonBoolSkipsProvision(t *testing.T) {
	registry := sideeffects.NewRegistry()
	schulhof := &fakeSchulhofService{}
	RegisterSettingsSideEffects(registry, schulhof, &fakeWCService{})

	cb, err := registry.Dispatch(context.Background(), 1, configModel.KeyCheckoutSchulhofEnabled, "not-a-bool")
	assert.NoError(t, err)
	assert.Nil(t, cb)
	assert.False(t, schulhof.called)
}

// TestRegisterSettingsSideEffects_SchulhofErrorPropagates ensures a
// service-level provisioning failure surfaces as a Dispatch error so the
// settings tx rolls back.
func TestRegisterSettingsSideEffects_SchulhofErrorPropagates(t *testing.T) {
	registry := sideeffects.NewRegistry()
	want := errors.New("provisioning failed")
	schulhof := &fakeSchulhofService{err: want}
	RegisterSettingsSideEffects(registry, schulhof, &fakeWCService{})

	cb, err := registry.Dispatch(context.Background(), 1, configModel.KeyCheckoutSchulhofEnabled, true)
	assert.Same(t, want, err)
	assert.Nil(t, cb)
	assert.True(t, schulhof.called)
}

// TestRegisterSettingsSideEffects_WCTrueProvisions covers the WC variant
// of the happy path; symmetric to the Schulhof case.
func TestRegisterSettingsSideEffects_WCTrueProvisions(t *testing.T) {
	registry := sideeffects.NewRegistry()
	schulhof := &fakeSchulhofService{}
	wc := &fakeWCService{}

	RegisterSettingsSideEffects(registry, schulhof, wc)

	cb, err := registry.Dispatch(context.Background(), 1, configModel.KeyCheckoutWCEnabled, true)
	require.NoError(t, err)
	assert.Nil(t, cb)
	assert.True(t, wc.called)
	assert.False(t, schulhof.called)
}

// TestRegisterSettingsSideEffects_WCFalseSkipsProvision pins the
// enable-only guard for WC, matching the schulhof contract.
func TestRegisterSettingsSideEffects_WCFalseSkipsProvision(t *testing.T) {
	registry := sideeffects.NewRegistry()
	wc := &fakeWCService{}
	RegisterSettingsSideEffects(registry, &fakeSchulhofService{}, wc)

	cb, err := registry.Dispatch(context.Background(), 1, configModel.KeyCheckoutWCEnabled, false)
	assert.NoError(t, err)
	assert.Nil(t, cb)
	assert.False(t, wc.called)
}
