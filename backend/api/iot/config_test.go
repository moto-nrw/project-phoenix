package iot

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type configurationStub struct {
	value devicescan.Configuration
	err   error
}

func (s configurationStub) DeviceConfiguration(context.Context) (devicescan.Configuration, error) {
	return s.value, s.err
}

func TestGetDeviceConfigPreservesWireEnvelope(t *testing.T) {
	t.Parallel()
	rs := NewResource(configurationStub{value: devicescan.Configuration{PresenceMode: "binary"}}, nil, testRuntime())
	req := httptest.NewRequest("GET", "/config", nil)
	testutil.WithDeviceIdentity(9, "config-device")(req)
	w := httptest.NewRecorder()
	rs.getDeviceConfig(w, req)
	require.Equal(t, 200, w.Code)
	assert.JSONEq(t, `{"status":"success","data":{"checkout":{"raumwechsel_enabled":false,"schulhof_enabled":false,"wc_enabled":false,"daily_checkout_time":null},"feedback":{"enabled":false},"presence_mode":"binary"},"message":"Device configuration retrieved"}`, w.Body.String())
}

func TestGetDeviceConfigPreservesFailureAndAuthentication(t *testing.T) {
	t.Parallel()
	rs := NewResource(configurationStub{err: errors.New("settings unavailable")}, nil, testRuntime())
	req := httptest.NewRequest("GET", "/config", nil)
	w := httptest.NewRecorder()
	rs.getDeviceConfig(w, req)
	assert.Equal(t, 401, w.Code)
	testutil.WithDeviceIdentity(9, "config-device")(req)
	w = httptest.NewRecorder()
	rs.getDeviceConfig(w, req)
	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "failed to resolve device configuration")
	assert.NotContains(t, w.Body.String(), "settings unavailable")
}
