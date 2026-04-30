package config_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
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

// The two tests below exercise the `logger != nil` branches inside the
// resolver's error-logging paths. Earlier tests passed nil for simplicity,
// leaving those branches uncovered; these use a real logger writing to a
// buffer so we can also assert the warning message is emitted with the
// expected key/value shape.

func TestResolvePresenceMode_ErrorWithLogger_LogsWarning(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	svc := &fakeSettingsService{strErr: errors.New("db gone")}

	mode := configSvc.ResolvePresenceMode(context.Background(), svc, logger)

	assert.Equal(t, configModel.PresenceModeDetailed, mode, "still falls back to detailed")
	assert.Contains(t, buf.String(), "presence_mode resolve failed")
	assert.Contains(t, buf.String(), "db gone")
}

func TestResolvePresenceModeForTenant_ErrorWithLogger_LogsWarning(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	svc := &fakeSettingsService{strErr: errors.New("tenant 42 not found")}

	mode := configSvc.ResolvePresenceModeForTenant(context.Background(), svc, 42, logger)

	assert.Equal(t, configModel.PresenceModeDetailed, mode)
	assert.Contains(t, buf.String(), "presence_mode resolve failed")
	assert.Contains(t, buf.String(), "tenant_id=42", "tenant ID must appear as a structured field")
}
