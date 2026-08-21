package config_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	"github.com/stretchr/testify/assert"
)

// newFakeSettingsService builds a configtest.Mock that lets tests control the
// behavior of HasTenantOverride and the Resolve* methods used by helpers.go
// and presence.go (Bool/Int/String, tenant and non-tenant variants share the
// same canned value/error per test).
func newFakeSettingsService(opts fakeSettingsServiceOpts) *configtest.Mock {
	return &configtest.Mock{
		HasTenantOverrideFn: func(context.Context, string) (bool, error) {
			return opts.hasOverride, opts.hasOverrideErr
		},
		ResolveBoolFn: func(context.Context, string) (bool, error) {
			return opts.boolVal, opts.boolErr
		},
		ResolveBoolForTenantFn: func(context.Context, int64, string) (bool, error) {
			return opts.boolVal, opts.boolErr
		},
		ResolveIntFn: func(context.Context, string) (int, error) {
			return opts.intVal, opts.intErr
		},
		ResolveIntForTenantFn: func(context.Context, int64, string) (int, error) {
			return opts.intVal, opts.intErr
		},
		ResolveStringFn: func(context.Context, string) (string, error) {
			return opts.strVal, opts.strErr
		},
		ResolveStringForTenantFn: func(context.Context, int64, string) (string, error) {
			return opts.strVal, opts.strErr
		},
	}
}

// fakeSettingsServiceOpts holds the canned values/errors newFakeSettingsService
// wires into a configtest.Mock — mirrors the fields of the old hand-rolled
// fakeSettingsService stub.
type fakeSettingsServiceOpts struct {
	hasOverride    bool
	hasOverrideErr error
	boolVal        bool
	boolErr        error
	intVal         int
	intErr         error
	strVal         string
	strErr         error
}

func TestResolveBoolOrDefault_NilService_ReturnsFallback(t *testing.T) {
	t.Parallel()

	got := configSvc.ResolveBoolOrDefault(context.Background(), nil, "any.key", true, nil)
	assert.True(t, got)
}

func TestResolveBoolOrDefault_NoOverride_ReturnsFallback(t *testing.T) {
	t.Parallel()

	svc := newFakeSettingsService(fakeSettingsServiceOpts{hasOverride: false, boolVal: true})
	got := configSvc.ResolveBoolOrDefault(context.Background(), svc, "any.key", false, slog.Default())
	assert.False(t, got, "must return fallback when HasTenantOverride is false, not the registry default")
}

func TestResolveBoolOrDefault_Override_ReturnsResolvedValue(t *testing.T) {
	t.Parallel()

	svc := newFakeSettingsService(fakeSettingsServiceOpts{hasOverride: true, boolVal: true})
	got := configSvc.ResolveBoolOrDefault(context.Background(), svc, "any.key", false, slog.Default())
	assert.True(t, got)
}

func TestResolveBoolOrDefault_HasOverrideError_ReturnsFallback(t *testing.T) {
	t.Parallel()

	svc := newFakeSettingsService(fakeSettingsServiceOpts{hasOverrideErr: errors.New("db down")})
	got := configSvc.ResolveBoolOrDefault(context.Background(), svc, "any.key", true, slog.Default())
	assert.True(t, got)
}

func TestResolveBoolOrDefault_ResolveError_ReturnsFallback(t *testing.T) {
	t.Parallel()

	svc := newFakeSettingsService(fakeSettingsServiceOpts{hasOverride: true, boolErr: errors.New("bad type")})
	got := configSvc.ResolveBoolOrDefault(context.Background(), svc, "any.key", true, slog.Default())
	assert.True(t, got)
}

func TestResolveIntOrDefault_NoOverride_ReturnsFallback(t *testing.T) {
	t.Parallel()

	svc := newFakeSettingsService(fakeSettingsServiceOpts{hasOverride: false, intVal: 99})
	got := configSvc.ResolveIntOrDefault(context.Background(), svc, "any.key", 7, slog.Default())
	assert.Equal(t, 7, got)
}

func TestResolveIntOrDefault_Override_ReturnsResolvedValue(t *testing.T) {
	t.Parallel()

	svc := newFakeSettingsService(fakeSettingsServiceOpts{hasOverride: true, intVal: 42})
	got := configSvc.ResolveIntOrDefault(context.Background(), svc, "any.key", 7, slog.Default())
	assert.Equal(t, 42, got)
}

func TestResolveIntOrDefault_NilService_ReturnsFallback(t *testing.T) {
	t.Parallel()

	got := configSvc.ResolveIntOrDefault(context.Background(), nil, "any.key", 99, nil)
	assert.Equal(t, 99, got)
}

func TestResolveIntOrDefault_HasOverrideError_ReturnsFallback(t *testing.T) {
	t.Parallel()

	svc := newFakeSettingsService(fakeSettingsServiceOpts{hasOverrideErr: errors.New("db down")})
	got := configSvc.ResolveIntOrDefault(context.Background(), svc, "any.key", 10, slog.Default())
	assert.Equal(t, 10, got)
}

func TestResolveIntOrDefault_ResolveError_ReturnsFallback(t *testing.T) {
	t.Parallel()

	svc := newFakeSettingsService(fakeSettingsServiceOpts{hasOverride: true, intErr: errors.New("bad type")})
	got := configSvc.ResolveIntOrDefault(context.Background(), svc, "any.key", 10, slog.Default())
	assert.Equal(t, 10, got)
}

func TestResolveStringOrDefault_NilService_ReturnsFallback(t *testing.T) {
	t.Parallel()

	got := configSvc.ResolveStringOrDefault(context.Background(), nil, "any.key", "fb", nil)
	assert.Equal(t, "fb", got)
}

func TestResolveStringOrDefault_HasOverrideError_ReturnsFallback(t *testing.T) {
	t.Parallel()

	svc := newFakeSettingsService(fakeSettingsServiceOpts{hasOverrideErr: errors.New("db down")})
	got := configSvc.ResolveStringOrDefault(context.Background(), svc, "any.key", "fb", slog.Default())
	assert.Equal(t, "fb", got)
}

func TestResolveStringOrDefault_ResolveError_ReturnsFallback(t *testing.T) {
	t.Parallel()

	svc := newFakeSettingsService(fakeSettingsServiceOpts{hasOverride: true, strErr: errors.New("bad")})
	got := configSvc.ResolveStringOrDefault(context.Background(), svc, "any.key", "fb", slog.Default())
	assert.Equal(t, "fb", got)
}

func TestResolveStringOrDefault_NoOverride_ReturnsFallback(t *testing.T) {
	t.Parallel()

	svc := newFakeSettingsService(fakeSettingsServiceOpts{hasOverride: false, strVal: "override"})
	got := configSvc.ResolveStringOrDefault(context.Background(), svc, "any.key", "fallback", slog.Default())
	assert.Equal(t, "fallback", got)
}

func TestResolveStringOrDefault_EmptyOverride_ReturnsFallback(t *testing.T) {
	t.Parallel()

	svc := newFakeSettingsService(fakeSettingsServiceOpts{hasOverride: true, strVal: ""})
	got := configSvc.ResolveStringOrDefault(context.Background(), svc, "any.key", "fallback", slog.Default())
	assert.Equal(t, "fallback", got, "empty override value should fall back")
}

func TestResolveStringOrDefault_Override_ReturnsResolvedValue(t *testing.T) {
	t.Parallel()

	svc := newFakeSettingsService(fakeSettingsServiceOpts{hasOverride: true, strVal: "override"})
	got := configSvc.ResolveStringOrDefault(context.Background(), svc, "any.key", "fallback", slog.Default())
	assert.Equal(t, "override", got)
}
