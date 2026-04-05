package config_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/stretchr/testify/assert"
)

// fakeSettingsService is a minimal stub of configSvc.SettingsService that lets
// tests control the behavior of HasTenantOverride and the Resolve* methods.
// It implements only what helpers.go uses.
type fakeSettingsService struct {
	hasOverride    bool
	hasOverrideErr error
	boolVal        bool
	boolErr        error
	intVal         int
	intErr         error
	strVal         string
	strErr         error
}

func (f *fakeSettingsService) GetSchema(context.Context, []string) (*configSvc.SettingsSchema, error) {
	return nil, nil
}
func (f *fakeSettingsService) Resolve(context.Context, string) (any, error) {
	return nil, nil
}
func (f *fakeSettingsService) ResolveString(context.Context, string) (string, error) {
	return f.strVal, f.strErr
}
func (f *fakeSettingsService) ResolveStringForTenant(context.Context, int64, string) (string, error) {
	return f.strVal, f.strErr
}
func (f *fakeSettingsService) ResolveBool(context.Context, string) (bool, error) {
	return f.boolVal, f.boolErr
}
func (f *fakeSettingsService) ResolveInt(context.Context, string) (int, error) {
	return f.intVal, f.intErr
}
func (f *fakeSettingsService) HasTenantOverride(context.Context, string) (bool, error) {
	return f.hasOverride, f.hasOverrideErr
}
func (f *fakeSettingsService) SetValue(context.Context, string, any, *int64, []string) error {
	return nil
}
func (f *fakeSettingsService) ResetValue(context.Context, string, *int64, []string) error {
	return nil
}

func TestResolveBoolOrDefault_NilService_ReturnsFallback(t *testing.T) {
	got := configSvc.ResolveBoolOrDefault(context.Background(), nil, "any.key", true, nil)
	assert.True(t, got)
}

func TestResolveBoolOrDefault_NoOverride_ReturnsFallback(t *testing.T) {
	svc := &fakeSettingsService{hasOverride: false, boolVal: true}
	got := configSvc.ResolveBoolOrDefault(context.Background(), svc, "any.key", false, slog.Default())
	assert.False(t, got, "must return fallback when HasTenantOverride is false, not the registry default")
}

func TestResolveBoolOrDefault_Override_ReturnsResolvedValue(t *testing.T) {
	svc := &fakeSettingsService{hasOverride: true, boolVal: true}
	got := configSvc.ResolveBoolOrDefault(context.Background(), svc, "any.key", false, slog.Default())
	assert.True(t, got)
}

func TestResolveBoolOrDefault_HasOverrideError_ReturnsFallback(t *testing.T) {
	svc := &fakeSettingsService{hasOverrideErr: errors.New("db down")}
	got := configSvc.ResolveBoolOrDefault(context.Background(), svc, "any.key", true, slog.Default())
	assert.True(t, got)
}

func TestResolveBoolOrDefault_ResolveError_ReturnsFallback(t *testing.T) {
	svc := &fakeSettingsService{hasOverride: true, boolErr: errors.New("bad type")}
	got := configSvc.ResolveBoolOrDefault(context.Background(), svc, "any.key", true, slog.Default())
	assert.True(t, got)
}

func TestResolveIntOrDefault_NoOverride_ReturnsFallback(t *testing.T) {
	svc := &fakeSettingsService{hasOverride: false, intVal: 99}
	got := configSvc.ResolveIntOrDefault(context.Background(), svc, "any.key", 7, slog.Default())
	assert.Equal(t, 7, got)
}

func TestResolveIntOrDefault_Override_ReturnsResolvedValue(t *testing.T) {
	svc := &fakeSettingsService{hasOverride: true, intVal: 42}
	got := configSvc.ResolveIntOrDefault(context.Background(), svc, "any.key", 7, slog.Default())
	assert.Equal(t, 42, got)
}

func TestResolveStringOrDefault_NoOverride_ReturnsFallback(t *testing.T) {
	svc := &fakeSettingsService{hasOverride: false, strVal: "override"}
	got := configSvc.ResolveStringOrDefault(context.Background(), svc, "any.key", "fallback", slog.Default())
	assert.Equal(t, "fallback", got)
}

func TestResolveStringOrDefault_EmptyOverride_ReturnsFallback(t *testing.T) {
	svc := &fakeSettingsService{hasOverride: true, strVal: ""}
	got := configSvc.ResolveStringOrDefault(context.Background(), svc, "any.key", "fallback", slog.Default())
	assert.Equal(t, "fallback", got, "empty override value should fall back")
}

func TestResolveStringOrDefault_Override_ReturnsResolvedValue(t *testing.T) {
	svc := &fakeSettingsService{hasOverride: true, strVal: "override"}
	got := configSvc.ResolveStringOrDefault(context.Background(), svc, "any.key", "fallback", slog.Default())
	assert.Equal(t, "override", got)
}
