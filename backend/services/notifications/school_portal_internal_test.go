package notifications

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/models/iot"
)

// The school portal (#2208) borrows from the staff catalogue: only types that
// opt in with SchoolPortal are offered, and a decision made there lands on
// the same (account, type) row.
func TestSchoolPortalCatalogue(t *testing.T) {
	t.Parallel()

	keys := make([]string, 0)
	for _, def := range TypesForPortal(PortalSchool) {
		keys = append(keys, def.Key)
	}
	assert.Equal(t, []string{TypeStaffMessage}, keys)

	staffMessage, _ := GetType(TypeStaffMessage)
	pickup, _ := GetType(TypePickupUpcoming)
	assert.True(t, OfferedInPortal(staffMessage, PortalSchool))
	assert.True(t, OfferedInPortal(staffMessage, PortalStaff), "still offered in the OGS portal")
	assert.False(t, OfferedInPortal(pickup, PortalSchool), "supervision reminders never address a Lehrkraft")
}

func TestSubscribeSchoolRecordsSchoolPortal(t *testing.T) {
	t.Parallel()
	repo := &recordingPushRepository{}
	service := NewPushSubscriptionService(nil, repo, nil, testVAPID(), nil)

	require.NoError(t, service.SubscribeSchool(context.Background(), 42, validPushInput()))
	require.Len(t, repo.upserted, 1)
	assert.Equal(t, iot.PushPortalSchool, repo.upserted[0].Portal)
}

// A school device gets the school host's deep link; every other device the
// payload as marshalled. A notification without a school destination sends
// the device to the portal root instead of to a path that does not exist
// there.
func TestPortalPayloadsPickTheSchoolDeepLink(t *testing.T) {
	t.Parallel()

	event := Event{
		Type:           TypeStaffMessage,
		Title:          "Neue Nachricht aus dem Team",
		DeepLink:       "/team-chat/7",
		SchoolDeepLink: "/school/nachrichten/7",
	}
	base, err := marshalPushPayload(event)
	require.NoError(t, err)
	payloads := &portalPayloads{event: event, base: base}

	staffWire, err := payloads.forSubscription(&iot.PushSubscription{Portal: iot.PushPortalStaff})
	require.NoError(t, err)
	assert.Equal(t, "/team-chat/7", deepLinkOf(t, staffWire))

	schoolWire, err := payloads.forSubscription(&iot.PushSubscription{Portal: iot.PushPortalSchool})
	require.NoError(t, err)
	assert.Equal(t, "/school/nachrichten/7", deepLinkOf(t, schoolWire))

	noSchool := &portalPayloads{event: Event{Type: "test", Title: "t", DeepLink: "/dashboard"}, base: base}
	wire, err := noSchool.forSubscription(&iot.PushSubscription{Portal: iot.PushPortalSchool})
	require.NoError(t, err)
	assert.Equal(t, "/school", deepLinkOf(t, wire))
}

func TestSchoolPushDeliveryIsLimitedToSupportedTypes(t *testing.T) {
	t.Parallel()

	staffSub := testSub(1, 41, "https://fcm.googleapis.com/staff")
	staffSub.AccountID = 42
	schoolSub := testSub(2, 41, "https://fcm.googleapis.com/school")
	schoolSub.AccountID = 42
	schoolSub.Portal = iot.PushPortalSchool
	channel := testChannel(&fakePushRepo{
		staffAccounts:  map[int64][]*iot.PushSubscription{42: {staffSub}},
		schoolAccounts: map[int64][]*iot.PushSubscription{42: {schoolSub}},
	}, &fakeSender{})

	unsupported, err := channel.resolveEventSubscriptions(context.Background(), Event{
		Type:     TypePickupUpcoming,
		Audience: Audience{Scope: ScopeStaff, StaffAccountIDs: []int64{42}},
	})
	require.NoError(t, err)
	assert.Equal(t, []*iot.PushSubscription{staffSub}, unsupported)

	supported, err := channel.resolveEventSubscriptions(context.Background(), Event{
		Type:     TypeStaffMessage,
		Audience: Audience{Scope: ScopeStaff, StaffAccountIDs: []int64{42}},
	})
	require.NoError(t, err)
	assert.ElementsMatch(t, []*iot.PushSubscription{staffSub, schoolSub}, supported)
}

func deepLinkOf(t *testing.T, wire []byte) string {
	t.Helper()
	var payload map[string]any
	require.NoError(t, json.Unmarshal(wire, &payload))
	link, _ := payload["deepLink"].(string)
	return link
}
