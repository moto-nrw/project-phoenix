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

func TestResolvePresenceModeForTenant_NilService(t *testing.T) {
	assert.Equal(t, configModel.PresenceModeDetailed, configSvc.ResolvePresenceModeForTenant(context.Background(), nil, 42, nil))
}

func TestResolvePresenceModeForTenant_OverrideBinary(t *testing.T) {
	svc := newFakeSettingsService(fakeSettingsServiceOpts{strVal: configModel.PresenceModeBinary})
	assert.Equal(t, configModel.PresenceModeBinary, configSvc.ResolvePresenceModeForTenant(context.Background(), svc, 42, nil))
}

func TestResolvePresenceModeForTenant_EmptyString_FallsBackToDetailed(t *testing.T) {
	svc := newFakeSettingsService(fakeSettingsServiceOpts{strVal: ""})
	assert.Equal(t, configModel.PresenceModeDetailed, configSvc.ResolvePresenceModeForTenant(context.Background(), svc, 42, nil))
}

func TestResolvePresenceModeForTenant_ResolveError_FallsBackToDetailed(t *testing.T) {
	svc := newFakeSettingsService(fakeSettingsServiceOpts{strErr: errors.New("tenant not found")})
	assert.Equal(t, configModel.PresenceModeDetailed, configSvc.ResolvePresenceModeForTenant(context.Background(), svc, 42, nil))
}

// The test below exercises the `logger != nil` branch inside the resolver's
// error-logging path. Earlier tests passed nil for simplicity, leaving that
// branch uncovered; this uses a real logger writing to a buffer so we can also
// assert the warning message is emitted with the expected key/value shape.

func TestResolvePresenceModeForTenant_ErrorWithLogger_LogsWarning(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	svc := newFakeSettingsService(fakeSettingsServiceOpts{strErr: errors.New("tenant 42 not found")})

	mode := configSvc.ResolvePresenceModeForTenant(context.Background(), svc, 42, logger)

	assert.Equal(t, configModel.PresenceModeDetailed, mode)
	assert.Contains(t, buf.String(), "presence_mode resolve failed")
	assert.Contains(t, buf.String(), "tenant_id=42", "tenant ID must appear as a structured field")
}
