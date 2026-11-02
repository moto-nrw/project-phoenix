package realtime

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRelayNormalizesTopicsAndActiveGroup(t *testing.T) {
	t.Parallel()
	for topic, want := range map[string]string{
		"42": "42", "0042": "42", "edu:42": "edu:42", "edu:0042": "edu:42",
		"": "", "0": "", "-3": "", "group": "", "edu:": "", "edu:x": "", "42 ": "", "edu:42:1": "", "<b>title</b>": "",
	} {
		assert.Equal(t, want, NormalizeTopic(topic), topic)
	}

	var payload string
	hub, err := NewRelayedHub(nil, func(s string) error { payload = s; return nil })
	require.NoError(t, err)
	require.NoError(t, hub.BroadcastToGroups(7, []string{"0042", "edu:9", "free text", "42"}, NewEvent(EventStudentCheckIn, "free text", EventData{})))
	var wire signal
	require.NoError(t, json.Unmarshal([]byte(payload), &wire))
	assert.Equal(t, []string{"42", "edu:9"}, wire.Topics)
	assert.Empty(t, wire.ActiveGroupID, "an invalid active group never travels")

	require.Error(t, hub.BroadcastToGroup(7, "free text", NewEvent(EventActivityEnd, "", EventData{})), "a group signal needs a valid topic")
}

func TestReceivedSignalUsesOnlyNormalizedTopics(t *testing.T) {
	t.Parallel()
	hub, err := NewRelayedHub(nil, func(string) error { return nil })
	require.NoError(t, err)
	client := &Client{Channel: make(chan Event, 4), SubscribedGroups: make(map[string]bool)}
	hub.Register(client, 7, []string{"42"})

	forged, err := json.Marshal(map[string]any{
		"school_id": 7, "audience": "group", "topic": "0042", "origin": "other", "type": "activity_end", "active_group_id": "<b>forged</b>",
	})
	require.NoError(t, err)
	require.NoError(t, hub.ReceiveSignal(string(forged)))
	event := <-client.Channel
	assert.Equal(t, EventActivityEnd, event.Type)
	assert.Empty(t, event.ActiveGroupID)

	invalid, err := json.Marshal(map[string]any{
		"school_id": 7, "audience": "group", "topic": "free text", "origin": "other", "type": "activity_end",
	})
	require.NoError(t, err)
	require.Error(t, hub.ReceiveSignal(string(invalid)))
	assert.Empty(t, client.Channel)
}

func TestNotificationEventWithoutStoredCopyStaysLocal(t *testing.T) {
	t.Parallel()
	published, payload := false, ""
	hub, err := NewRelayedHub(nil, func(s string) error { published, payload = true, s; return nil })
	require.NoError(t, err)
	title := "Titel"
	event := NewEvent(EventNotification, "", EventData{Title: &title})
	require.NoError(t, hub.BroadcastToTenant(7, event))
	assert.False(t, published)
	require.NoError(t, hub.BroadcastToTenant(7, event.WithNotificationID(0)))
	assert.False(t, published)
	require.NoError(t, hub.BroadcastToTenant(7, NewEvent(EventStudentUpdated, "", EventData{}).WithNotificationID(5)))
	assert.True(t, published)
	assert.NotContains(t, payload, "notification_id", "only notifications carry a stored ID")
}
