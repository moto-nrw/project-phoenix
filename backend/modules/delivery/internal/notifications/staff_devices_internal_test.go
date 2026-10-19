package notifications

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/delivery/devices"
	"github.com/stretchr/testify/require"
)

type staffPreferenceProjectionStub struct {
	PreferenceService
	row *PreferenceOverview
	err error
}

func (s staffPreferenceProjectionStub) GetForAccount(context.Context, int64, string) (*PreferenceOverview, error) {
	return s.row, s.err
}

func TestStaffPreferencesPreservesWireShapesAndRawErrors(t *testing.T) {
	t.Parallel()
	nativeErr := errors.New("native preference failure")
	for _, row := range []*PreferenceOverview{nil, {}, {TenantEnabled: true, Types: []PreferenceState{}}, {TenantEnabled: true, Types: []PreferenceState{{Key: "staff_message", Label: "label", Description: "description", Group: "mitteilungen", Enabled: true, Available: true}}}} {
		facade := staffPreferences{source: staffPreferenceProjectionStub{row: row}}
		got, err := facade.GetForAccount(context.Background(), 73, PortalSchool)
		require.NoError(t, err)
		if row == nil {
			require.Nil(t, got)
			continue
		}
		require.Equal(t, row.TenantEnabled, got.TenantEnabled)
		if row.Types == nil {
			require.Nil(t, got.Types)
			continue
		}
		require.NotNil(t, got.Types)
		require.Len(t, got.Types, len(row.Types))
		for i, value := range row.Types {
			require.Equal(t, devices.PreferenceState(value), got.Types[i])
		}
	}
	facade := staffPreferences{source: staffPreferenceProjectionStub{row: &PreferenceOverview{}, err: nativeErr}}
	row, err := facade.GetForAccount(context.Background(), 73, PortalStaff)
	require.Nil(t, row)
	require.Same(t, nativeErr, err)
}

type staffPushFailureStub struct {
	PushSubscriptionService
	err error
}

func (s staffPushFailureStub) PublicKey() (string, error) { return "", s.err }
func (s staffPushFailureStub) Subscribe(context.Context, int64, PushSubscriptionInput) error {
	return s.err
}

func TestStaffDevicesPreservesRawErrorsAndIndependentFamilies(t *testing.T) {
	t.Parallel()
	nativeErr := errors.New("native push failure")
	facade := staffPushSubscriptions{source: staffPushFailureStub{err: nativeErr}}
	_, err := facade.PublicKey()
	require.Same(t, nativeErr, err)
	require.Same(t, nativeErr, facade.Subscribe(context.Background(), 73, devices.PushSubscriptionInput{}))
	joined := errors.Join(fmt.Errorf("configuration: %w", ErrWebPushNotConfigured), fmt.Errorf("input: %w", ErrInvalidPushSubscription))
	require.True(t, facade.IsNotConfigured(joined))
	require.True(t, facade.IsInvalidSubscription(joined))
	require.False(t, facade.IsNotConfigured(nativeErr))
	prefs := staffPreferences{}
	require.True(t, prefs.IsUnknownType(fmt.Errorf("type: %w", ErrUnknownNotificationType)))
	require.True(t, prefs.IsPermissionRequired(fmt.Errorf("permission: %w", ErrEmailPermissionRequired)))
	require.False(t, prefs.IsUnknownType(nativeErr))
}
