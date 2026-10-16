package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/delivery/devices"
	parentAPI "github.com/moto-nrw/project-phoenix/workflows/parentportal/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParentDeliveryPublicErrorsKeepDiagnosticsAndCause(t *testing.T) {
	t.Parallel()
	source := deliveryCompose.NewPushSubscriptionService(nil, nil, nil, deliveryCompose.VAPIDConfig{}, nil)
	_, original := source.PublicKey()
	_, translated := bindParentPushSubscriptions(source).PublicKey()
	require.ErrorIs(t, translated, parentAPI.ErrWebPushNotConfigured)
	assert.ErrorIs(t, translated, devices.ErrWebPushNotConfigured)
	assert.ErrorIs(t, translated, original)
	assert.Equal(t, original.Error(), translated.Error())
	for _, pair := range [][2]error{
		{devices.ErrInvalidPushSubscription, parentAPI.ErrInvalidPushSubscription},
		{devices.ErrUnknownNotificationType, parentAPI.ErrUnknownNotificationType},
	} {
		original := fmt.Errorf("retained diagnostic: %w", pair[0])
		translated := parentDeliveryError(original)
		assert.ErrorIs(t, translated, pair[1])
		assert.ErrorIs(t, translated, original)
		assert.Equal(t, original.Error(), translated.Error())
	}
	unrelated := errors.New("unrelated failure")
	assert.Same(t, unrelated, parentDeliveryError(unrelated))
	assert.NoError(t, parentDeliveryError(nil))
	assert.Nil(t, bindParentPushSubscriptions(nil))
	assert.Nil(t, bindParentPreferences(nil))
}

type parentPreferenceProjectionStub struct {
	devices.NotificationPreferences
	row *devices.PreferenceOverview
}

func (s parentPreferenceProjectionStub) ParentPreferences() devices.NotificationPreferences { return s }
func (s parentPreferenceProjectionStub) GetForParent(context.Context, int64) (*devices.PreferenceOverview, error) {
	return s.row, nil
}

func TestParentPreferenceProjectionPreservesWireShape(t *testing.T) {
	t.Parallel()
	for _, row := range []*devices.PreferenceOverview{
		nil,
		{},
		{Types: []devices.PreferenceState{}},
		{TenantEnabled: true, Types: []devices.PreferenceState{{Key: "reminder", Label: "Reminder", Description: "School reminder", Group: "parent", Enabled: true, Available: true}}},
	} {
		source := parentPreferenceProjectionStub{row: row}
		var accountID int64
		got, err := bindParentPreferences(source).GetForParent(context.Background(), accountID)
		require.NoError(t, err)
		wantJSON, err := json.Marshal(row)
		require.NoError(t, err)
		gotJSON, err := json.Marshal(got)
		require.NoError(t, err)
		assert.Equal(t, string(wantJSON), string(gotJSON))
	}
}
