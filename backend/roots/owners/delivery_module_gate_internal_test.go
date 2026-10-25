package owners

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	deliveryModels "github.com/moto-nrw/project-phoenix/modules/delivery/subscriptions"
)

// cancelledModules books every module except the listed keys per school.
type cancelledModules struct {
	off map[int64]string
	err error
}

func (c cancelledModules) ResolveBoolForTenant(_ context.Context, tenantID int64, key string) (bool, error) {
	if c.err != nil {
		return false, c.err
	}
	return c.off[tenantID] != key, nil
}

func newTestDeliveryGate(t *testing.T, reader cancelledModules) deliveryModuleGate {
	t.Helper()
	gate, err := newDeliveryModuleGate(reader, []string{"parent_announcement"}, []string{"enrollment_approved"})
	require.NoError(t, err)
	return gate
}

func mailIntent(tenantID int64, kind string) deliveryContract.ClaimedIntent {
	return deliveryContract.ClaimedIntent{TenantID: tenantID, Transport: deliveryContract.TransportEmail, Template: kind}
}

func pushIntent(tenantID int64, portal string) deliveryContract.ClaimedIntent {
	return deliveryContract.ClaimedIntent{TenantID: tenantID, Transport: deliveryContract.TransportPush, Template: "staff_message",
		PushRecipient: deliveryContract.PushRecipient{Portal: portal}}
}

func TestDeliveryGateStopsMailOfACancelledModule(t *testing.T) {
	t.Parallel()
	gate := newTestDeliveryGate(t, cancelledModules{off: map[int64]string{42: "module.eltern_app", 43: "module.online_anmeldung"}})
	ctx := context.Background()

	for _, tc := range []struct {
		name   string
		intent deliveryContract.ClaimedIntent
		want   bool
	}{
		{"parent mail without Eltern-App", mailIntent(42, "parent_announcement"), false},
		{"parent mail of another school", mailIntent(41, "parent_announcement"), true},
		{"enrollment mail without Online-Anmeldung", mailIntent(43, "enrollment_approved"), false},
		{"enrollment mail with Online-Anmeldung", mailIntent(42, "enrollment_approved"), true},
		{"Basis mail", mailIntent(42, "transactional_mail"), true},
		{"parent push without Eltern-App", pushIntent(42, deliveryModels.PushPortalParent), false},
		{"staff push without Eltern-App", pushIntent(42, deliveryModels.PushPortalStaff), true},
	} {
		deliverable, err := gate.Deliverable(ctx, tc.intent)
		require.NoError(t, err, tc.name)
		assert.Equal(t, tc.want, deliverable, tc.name)
	}
}

func TestDeliveryGateStopsSchoolPortalPushWithoutUnterrichtVormittag(t *testing.T) {
	t.Parallel()
	gate := newTestDeliveryGate(t, cancelledModules{off: map[int64]string{42: "module.unterricht_vormittag"}})

	deliverable, err := gate.Deliverable(context.Background(), pushIntent(42, deliveryModels.PushPortalSchool))
	require.NoError(t, err)
	assert.False(t, deliverable)

	deliverable, err = gate.Deliverable(context.Background(), pushIntent(41, deliveryModels.PushPortalSchool))
	require.NoError(t, err)
	assert.True(t, deliverable)
}

func TestDeliveryGateRetriesOnReadErrors(t *testing.T) {
	t.Parallel()
	readErr := errors.New("settings unavailable")
	gate := newTestDeliveryGate(t, cancelledModules{err: readErr})

	_, err := gate.Deliverable(context.Background(), mailIntent(42, "parent_announcement"))
	require.ErrorIs(t, err, readErr, "an unreadable switch is neither booked nor cancelled")
}
