package config_test

import (
	"context"
	"errors"
	"testing"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/stretchr/testify/assert"
)

func TestResolvePresenceMode_NilService(t *testing.T) {
	assert.Equal(t, configModel.PresenceModeDetailed, configSvc.ResolvePresenceMode(context.Background(), nil, nil))
}

func TestResolvePresenceMode_NoOverride_ReturnsRegistryDefault(t *testing.T) {
	// ResolveString returns the registry default ("detailed") when no tenant
	// override exists. The helper should pass that through unchanged.
	svc := &fakeSettingsService{strVal: configModel.PresenceModeDetailed}
	assert.Equal(t, configModel.PresenceModeDetailed, configSvc.ResolvePresenceMode(context.Background(), svc, nil))
}

func TestResolvePresenceMode_OverrideBinary(t *testing.T) {
	svc := &fakeSettingsService{strVal: configModel.PresenceModeBinary}
	assert.Equal(t, configModel.PresenceModeBinary, configSvc.ResolvePresenceMode(context.Background(), svc, nil))
}

func TestResolvePresenceMode_EmptyString_FallsBackToDetailed(t *testing.T) {
	svc := &fakeSettingsService{strVal: ""}
	assert.Equal(t, configModel.PresenceModeDetailed, configSvc.ResolvePresenceMode(context.Background(), svc, nil))
}

func TestResolvePresenceMode_ResolveError_FallsBackToDetailed(t *testing.T) {
	svc := &fakeSettingsService{strErr: errors.New("db down")}
	assert.Equal(t, configModel.PresenceModeDetailed, configSvc.ResolvePresenceMode(context.Background(), svc, nil))
}

func TestResolvePresenceModeForTenant_NilService(t *testing.T) {
	assert.Equal(t, configModel.PresenceModeDetailed, configSvc.ResolvePresenceModeForTenant(context.Background(), nil, 42, nil))
}

func TestResolvePresenceModeForTenant_OverrideBinary(t *testing.T) {
	svc := &fakeSettingsService{strVal: configModel.PresenceModeBinary}
	assert.Equal(t, configModel.PresenceModeBinary, configSvc.ResolvePresenceModeForTenant(context.Background(), svc, 42, nil))
}

func TestResolvePresenceModeForTenant_ResolveError_FallsBackToDetailed(t *testing.T) {
	svc := &fakeSettingsService{strErr: errors.New("tenant not found")}
	assert.Equal(t, configModel.PresenceModeDetailed, configSvc.ResolvePresenceModeForTenant(context.Background(), svc, 42, nil))
}

func TestIsBinaryMode(t *testing.T) {
	assert.True(t, configSvc.IsBinaryMode(configModel.PresenceModeBinary))
	assert.False(t, configSvc.IsBinaryMode(configModel.PresenceModeDetailed))
	assert.False(t, configSvc.IsBinaryMode(""))
	assert.False(t, configSvc.IsBinaryMode("unknown"))
}
