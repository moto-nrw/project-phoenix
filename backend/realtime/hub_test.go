package realtime

import (
	"testing"
	"time"
)

// TestHubRegister verifies client registration with group subscriptions
func TestHubRegister(t *testing.T) {
	tests := []struct {
		name                string
		activeGroupIDs      []string
		expectedClientCount int
		expectedGroupCount  map[string]int
	}{
		{
			name:                "Register single client with one group",
			activeGroupIDs:      []string{"group_1"},
			expectedClientCount: 1,
			expectedGroupCount:  map[string]int{"group_1": 1},
		},
		{
			name:                "Register client with multiple groups",
			activeGroupIDs:      []string{"group_1", "group_2", "group_3"},
			expectedClientCount: 1,
			expectedGroupCount:  map[string]int{"group_1": 1, "group_2": 1, "group_3": 1},
		},
		{
			name:                "Register client with no groups",
			activeGroupIDs:      []string{},
			expectedClientCount: 1,
			expectedGroupCount:  map[string]int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hub := NewHub()
			client := &Client{
				Channel:          make(chan Event, 10),
				UserID:           123,
				SubscribedTopics: make(map[string]bool),
			}

			hub.Register(client, tt.activeGroupIDs)

			// Verify client count
			if got := hub.GetClientCount(); got != tt.expectedClientCount {
				t.Errorf("GetClientCount() = %v, want %v", got, tt.expectedClientCount)
			}

			// Verify group subscriber counts
			for groupID, expectedCount := range tt.expectedGroupCount {
				if got := hub.GetGroupSubscriberCount(groupID); got != expectedCount {
					t.Errorf("GetGroupSubscriberCount(%s) = %v, want %v", groupID, got, expectedCount)
				}
			}

			// Verify client's subscribed topics
			for _, groupID := range tt.activeGroupIDs {
				if !client.SubscribedTopics[groupID] {
					t.Errorf("Client not subscribed to topic %s", groupID)
				}
			}
		})
	}
}

// TestHubRegisterMultipleClients verifies multiple client registrations
func TestHubRegisterMultipleClients(t *testing.T) {
	hub := NewHub()

	// Register three clients to the same group
	client1 := &Client{
		Channel:          make(chan Event, 10),
		UserID:           1,
		SubscribedTopics: make(map[string]bool),
	}
	client2 := &Client{
		Channel:          make(chan Event, 10),
		UserID:           2,
		SubscribedTopics: make(map[string]bool),
	}
	client3 := &Client{
		Channel:          make(chan Event, 10),
		UserID:           3,
		SubscribedTopics: make(map[string]bool),
	}

	hub.Register(client1, []string{"group_1"})
	hub.Register(client2, []string{"group_1"})
	hub.Register(client3, []string{"group_1", "group_2"})

	// Verify counts
	if got := hub.GetClientCount(); got != 3 {
		t.Errorf("GetClientCount() = %v, want 3", got)
	}

	if got := hub.GetGroupSubscriberCount("group_1"); got != 3 {
		t.Errorf("GetGroupSubscriberCount(group_1) = %v, want 3", got)
	}

	if got := hub.GetGroupSubscriberCount("group_2"); got != 1 {
		t.Errorf("GetGroupSubscriberCount(group_2) = %v, want 1", got)
	}
}

// TestHubUnregister verifies client unregistration and cleanup
func TestHubUnregister(t *testing.T) {
	tests := []struct {
		name                string
		setupClients        int
		unregisterClient    int // 0-indexed
		expectedClientCount int
		groupID             string
		expectedGroupCount  int
	}{
		{
			name:                "Unregister single client",
			setupClients:        1,
			unregisterClient:    0,
			expectedClientCount: 0,
			groupID:             "group_1",
			expectedGroupCount:  0,
		},
		{
			name:                "Unregister one of multiple clients",
			setupClients:        3,
			unregisterClient:    1,
			expectedClientCount: 2,
			groupID:             "group_1",
			expectedGroupCount:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hub := NewHub()
			clients := make([]*Client, tt.setupClients)

			// Register clients
			for i := 0; i < tt.setupClients; i++ {
				clients[i] = &Client{
					Channel:          make(chan Event, 10),
					UserID:           int64(i + 1),
					SubscribedTopics: make(map[string]bool),
				}
				hub.Register(clients[i], []string{tt.groupID})
			}

			// Unregister specified client
			hub.Unregister(clients[tt.unregisterClient])

			// Verify client count
			if got := hub.GetClientCount(); got != tt.expectedClientCount {
				t.Errorf("GetClientCount() = %v, want %v", got, tt.expectedClientCount)
			}

			// Verify group subscriber count
			if got := hub.GetGroupSubscriberCount(tt.groupID); got != tt.expectedGroupCount {
				t.Errorf("GetGroupSubscriberCount(%s) = %v, want %v", tt.groupID, got, tt.expectedGroupCount)
			}

			// Verify channel is closed
			_, ok := <-clients[tt.unregisterClient].Channel
			if ok {
				t.Error("Client channel should be closed after unregister")
			}
		})
	}
}

// TestHubUnregisterNonExistent verifies idempotent unregister
func TestHubUnregisterNonExistent(t *testing.T) {
	hub := NewHub()
	client := &Client{
		Channel:          make(chan Event, 10),
		UserID:           1,
		SubscribedTopics: make(map[string]bool),
	}

	// Unregister client that was never registered (should not panic)
	hub.Unregister(client)

	// Verify no clients registered
	if got := hub.GetClientCount(); got != 0 {
		t.Errorf("GetClientCount() = %v, want 0", got)
	}
}

// TestHubUnregisterCleanup verifies topicClients map cleanup
func TestHubUnregisterCleanup(t *testing.T) {
	hub := NewHub()
	client := &Client{
		Channel:          make(chan Event, 10),
		UserID:           1,
		SubscribedTopics: make(map[string]bool),
	}

	hub.Register(client, []string{"group_1", "group_2"})

	// Verify groups have subscribers
	if got := hub.GetGroupSubscriberCount("group_1"); got != 1 {
		t.Errorf("GetGroupSubscriberCount(group_1) = %v, want 1", got)
	}

	hub.Unregister(client)

	// Verify groups are cleaned up (no subscribers)
	if got := hub.GetGroupSubscriberCount("group_1"); got != 0 {
		t.Errorf("GetGroupSubscriberCount(group_1) after cleanup = %v, want 0", got)
	}

	if got := hub.GetGroupSubscriberCount("group_2"); got != 0 {
		t.Errorf("GetGroupSubscriberCount(group_2) after cleanup = %v, want 0", got)
	}

	// Verify internal map is cleaned up
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	if len(hub.topicClients) != 0 {
		t.Errorf("topicClients map should be empty, got %v entries", len(hub.topicClients))
	}
}

// TestHubBroadcastToSingleSubscriber verifies event delivery to one client
func TestHubBroadcastToSingleSubscriber(t *testing.T) {
	hub := NewHub()
	client := &Client{
		Channel:          make(chan Event, 10),
		UserID:           1,
		SubscribedTopics: make(map[string]bool),
	}

	hub.Register(client, []string{"group_1"})

	event := NewEvent(EventStudentCheckIn, "group_1", EventData{
		StudentID:   strPtr("123"),
		StudentName: strPtr("Test Student"),
	})

	// Broadcast event
	err := hub.BroadcastToGroup("group_1", event)
	if err != nil {
		t.Errorf("BroadcastToGroup() error = %v, want nil", err)
	}

	// Verify event received
	select {
	case received := <-client.Channel:
		if received.Type != event.Type {
			t.Errorf("Received event type = %v, want %v", received.Type, event.Type)
		}
		if received.ActiveGroupID != event.ActiveGroupID {
			t.Errorf("Received event ActiveGroupID = %v, want %v", received.ActiveGroupID, event.ActiveGroupID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for event")
	}
}

// TestHubBroadcastToMultipleSubscribers verifies event delivery to multiple clients
func TestHubBroadcastToMultipleSubscribers(t *testing.T) {
	hub := NewHub()

	// Register three clients to the same group
	clients := make([]*Client, 3)
	for i := 0; i < 3; i++ {
		clients[i] = &Client{
			Channel:          make(chan Event, 10),
			UserID:           int64(i + 1),
			SubscribedTopics: make(map[string]bool),
		}
		hub.Register(clients[i], []string{"group_1"})
	}

	event := NewEvent(EventActivityStart, "group_1", EventData{
		ActivityName: strPtr("Test Activity"),
	})

	// Broadcast event
	err := hub.BroadcastToGroup("group_1", event)
	if err != nil {
		t.Errorf("BroadcastToGroup() error = %v, want nil", err)
	}

	// Verify all clients received the event
	for i, client := range clients {
		select {
		case received := <-client.Channel:
			if received.Type != event.Type {
				t.Errorf("Client %d: received event type = %v, want %v", i, received.Type, event.Type)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("Client %d: timeout waiting for event", i)
		}
	}
}

// TestHubBroadcastGroupIsolation verifies events only go to subscribed groups
func TestHubBroadcastGroupIsolation(t *testing.T) {
	hub := NewHub()

	client1 := &Client{
		Channel:          make(chan Event, 10),
		UserID:           1,
		SubscribedTopics: make(map[string]bool),
	}
	client2 := &Client{
		Channel:          make(chan Event, 10),
		UserID:           2,
		SubscribedTopics: make(map[string]bool),
	}

	hub.Register(client1, []string{"group_1"})
	hub.Register(client2, []string{"group_2"})

	event := NewEvent(EventStudentCheckIn, "group_1", EventData{
		StudentID: strPtr("123"),
	})

	// Broadcast to group_1
	err := hub.BroadcastToGroup("group_1", event)
	if err != nil {
		t.Errorf("BroadcastToGroup() error = %v, want nil", err)
	}

	// Verify client1 received event
	select {
	case <-client1.Channel:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Error("Client1 should have received event")
	}

	// Verify client2 did NOT receive event
	select {
	case <-client2.Channel:
		t.Error("Client2 should not have received event for group_1")
	case <-time.After(50 * time.Millisecond):
		// Success - timeout expected
	}
}

// TestHubBroadcastNoSubscribers verifies silent broadcast when no clients
func TestHubBroadcastNoSubscribers(t *testing.T) {
	hub := NewHub()

	event := NewEvent(EventStudentCheckIn, "group_nonexistent", EventData{
		StudentID: strPtr("123"),
	})

	// Broadcast to group with no subscribers (should not error)
	err := hub.BroadcastToGroup("group_nonexistent", event)
	if err != nil {
		t.Errorf("BroadcastToGroup() with no subscribers should return nil, got error: %v", err)
	}

	// Verify hub still functional
	if got := hub.GetClientCount(); got != 0 {
		t.Errorf("GetClientCount() = %v, want 0", got)
	}
}

// TestHubBroadcastChannelFull verifies skip behavior when channel is full
func TestHubBroadcastChannelFull(t *testing.T) {
	hub := NewHub()

	// Create client with very small buffer
	client := &Client{
		Channel:          make(chan Event, 1),
		UserID:           1,
		SubscribedTopics: make(map[string]bool),
	}

	hub.Register(client, []string{"group_1"})

	// Fill the channel
	event1 := NewEvent(EventStudentCheckIn, "group_1", EventData{StudentID: strPtr("1")})
	client.Channel <- event1

	// Try to broadcast when channel is full (should not block or error)
	event2 := NewEvent(EventStudentCheckIn, "group_1", EventData{StudentID: strPtr("2")})
	err := hub.BroadcastToGroup("group_1", event2)
	if err != nil {
		t.Errorf("BroadcastToGroup() with full channel should return nil, got error: %v", err)
	}

	// Verify only first event in channel
	select {
	case received := <-client.Channel:
		if received.Data.StudentID == nil || *received.Data.StudentID != "1" {
			t.Error("Expected first event, got something else")
		}
	default:
		t.Error("Expected event in channel")
	}

	// Channel should now be empty (event2 was skipped)
	select {
	case <-client.Channel:
		t.Error("Channel should be empty after consuming first event")
	default:
		// Success - channel empty as expected
	}
}

// TestHubGetClientCount verifies client counting
func TestHubGetClientCount(t *testing.T) {
	hub := NewHub()

	// Initially zero
	if got := hub.GetClientCount(); got != 0 {
		t.Errorf("Initial GetClientCount() = %v, want 0", got)
	}

	// Add clients
	for i := 0; i < 5; i++ {
		client := &Client{
			Channel:          make(chan Event, 10),
			UserID:           int64(i + 1),
			SubscribedTopics: make(map[string]bool),
		}
		hub.Register(client, []string{"group_1"})
	}

	if got := hub.GetClientCount(); got != 5 {
		t.Errorf("After 5 registrations, GetClientCount() = %v, want 5", got)
	}
}

// TestHubGetGroupSubscriberCount verifies group subscriber counting
func TestHubGetGroupSubscriberCount(t *testing.T) {
	hub := NewHub()

	// Non-existent group
	if got := hub.GetGroupSubscriberCount("nonexistent"); got != 0 {
		t.Errorf("GetGroupSubscriberCount(nonexistent) = %v, want 0", got)
	}

	// Add subscribers
	for i := 0; i < 3; i++ {
		client := &Client{
			Channel:          make(chan Event, 10),
			UserID:           int64(i + 1),
			SubscribedTopics: make(map[string]bool),
		}
		hub.Register(client, []string{"group_1"})
	}

	if got := hub.GetGroupSubscriberCount("group_1"); got != 3 {
		t.Errorf("GetGroupSubscriberCount(group_1) = %v, want 3", got)
	}
}

// Helper function to create string pointers
func strPtr(s string) *string {
	return &s
}
