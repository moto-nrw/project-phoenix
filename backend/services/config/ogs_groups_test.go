package config_test

import (
	"context"
	"errors"
	"testing"

	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/stretchr/testify/assert"
)

func TestResolveOGSGroupsEnabled_NilService(t *testing.T) {
	assert.True(t, configSvc.ResolveOGSGroupsEnabled(context.Background(), nil, nil))
}

func TestResolveOGSGroupsEnabled_ReturnsResolvedBool(t *testing.T) {
	svc := &fakeSettingsService{boolVal: false}
	assert.False(t, configSvc.ResolveOGSGroupsEnabled(context.Background(), svc, nil))
}

func TestResolveOGSGroupsEnabled_ErrorFallsBackToEnabled(t *testing.T) {
	svc := &fakeSettingsService{boolErr: errors.New("db down")}
	assert.True(t, configSvc.ResolveOGSGroupsEnabled(context.Background(), svc, nil))
}

func TestResolveOGSGroupsEnabledForTenant_FalseStringDisables(t *testing.T) {
	svc := &fakeSettingsService{strVal: "false"}
	assert.False(t, configSvc.ResolveOGSGroupsEnabledForTenant(context.Background(), svc, 42, nil))
}

func TestResolveOGSGroupsEnabledForTenant_ErrorFallsBackToEnabled(t *testing.T) {
	svc := &fakeSettingsService{strErr: errors.New("db down")}
	assert.True(t, configSvc.ResolveOGSGroupsEnabledForTenant(context.Background(), svc, 42, nil))
}
