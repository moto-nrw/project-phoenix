package application

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type configurationSourceStub struct {
	tenantID int64
	err      error
}

func (s *configurationSourceStub) LoadConfiguration(_ context.Context, tenantID int64) (devicescan.Configuration, error) {
	s.tenantID = tenantID
	return devicescan.Configuration{}, s.err
}

func TestKioskQueriesUseAuthenticatedDeviceTenant(t *testing.T) {
	t.Parallel()
	device := testDevice()
	device.TenantID = 777
	principals := fakePrincipals{device: device}
	source := &configurationSourceStub{}
	query := NewConfiguration(source, principals)
	_, err := query.DeviceConfiguration(t.Context())
	require.NoError(t, err)
	assert.Equal(t, int64(777), source.tenantID)
	school := NewSchoolName(func(_ context.Context, id int64) (string, error) {
		assert.Equal(t, int64(777), id)
		return "Test School", nil
	}, principals)
	name, err := school.DeviceSchoolName(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "Test School", name)
}

func TestKioskQueriesRejectMissingDeviceAndKeepConfigurationErrors(t *testing.T) {
	t.Parallel()
	source := &configurationSourceStub{err: errors.New("settings unavailable")}
	_, err := NewConfiguration(source, fakePrincipals{}).DeviceConfiguration(t.Context())
	assert.ErrorIs(t, err, devicescan.ErrDeviceUnauthorized)
	assert.Zero(t, source.tenantID)
	_, err = NewSchoolName(func(context.Context, int64) (string, error) { t.Fatal("unauthenticated school lookup"); return "", nil }, fakePrincipals{}).DeviceSchoolName(t.Context())
	assert.ErrorIs(t, err, devicescan.ErrDeviceUnauthorized)
	_, err = NewConfiguration(source, fakePrincipals{device: testDevice()}).DeviceConfiguration(t.Context())
	failure, ok := devicescan.IsFailure(err)
	require.True(t, ok)
	assert.Equal(t, "failed to resolve device configuration", failure.Message)
	assert.ErrorIs(t, err, source.err)
}
