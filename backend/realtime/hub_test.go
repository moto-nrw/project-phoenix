package realtime

import (
	"fmt"
	"log/slog"
	"testing"
	"time"
)

// TestHubRegister verifies client registration with group subscriptions
func TestHubRegister(t *testing.T) {
	t.Parallel()

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
			hub := NewHub(slog.Default())
			client := &Client{
				Channel:          make(chan Event, 10),
				UserID:           123,
				SubscribedGroups: make(map[string]bool),
			}

			hub.Register(client, int64(42), tt.activeGroupIDs)

			// Verify client count
			if got := hub.GetClientCount(); got != tt.expectedClientCount {
				t.Errorf("GetClientCount() = %v, want %v", got, tt.expectedClientCount)
			}

			// Verify group subscriber counts
			for groupID, expectedCount := range tt.expectedGroupCount {
				if got := hub.GetGroupSubscriberCount(int64(42), groupID); got != expectedCount {
					t.Errorf("GetGroupSubscriberCount(%s) = %v, want %v", groupID, got, expectedCount)
				}
			}

			// Verify client's subscribed groups (stored as composite "tenantID:groupID" keys)
			for _, groupID := range tt.activeGroupIDs {
				key := fmt.Sprintf("%d:%s", int64(42), groupID)
				if !client.SubscribedGroups[key] {
					t.Errorf("Client not subscribed to group %s (key %s)", groupID, key)
				}
			}
		})
	}
}

// TestHubRegisterMultipleClients verifies multiple client registrations
func TestHubRegisterMultipleClients(t *testing.T) {
	t.Parallel()

	hub := NewHub(slog.Default())

	// Register three clients to the same group
	client1 := &Client{
		Channel:          make(chan Event, 10),
		UserID:           1,
		SubscribedGroups: make(map[string]bool),
	}
	client2 := &Client{
		Channel:          make(chan Event, 10),
		UserID:           2,
		SubscribedGroups: make(map[string]bool),
	}
	client3 := &Client{
		Channel:          make(chan Event, 10),
		UserID:           3,
		SubscribedGroups: make(map[string]bool),
	}

	hub.Register(client1, int64(42), []string{"group_1"})
	hub.Register(client2, int64(42), []string{"group_1"})
	hub.Register(client3, int64(42), []string{"group_1", "group_2"})

	// Verify counts
	if got := hub.GetClientCount(); got != 3 {
		t.Errorf("GetClientCount() = %v, want 3", got)
	}

	if got := hub.GetGroupSubscriberCount(int64(42), "group_1"); got != 3 {
		t.Errorf("GetGroupSubscriberCount(group_1) = %v, want 3", got)
	}

	if got := hub.GetGroupSubscriberCount(int64(42), "group_2"); got != 1 {
		t.Errorf("GetGroupSubscriberCount(group_2) = %v, want 1", got)
	}
}

func TestBroadcastToGroupsDeduplicatesOverlappingSubscriptions(t *testing.T) {
	t.Parallel()

	hub := NewHub(slog.Default())
	overlap := &Client{Channel: make(chan Event, 2), UserID: 1, SubscribedGroups: make(map[string]bool)}
	oneTopic := &Client{Channel: make(chan Event, 2), UserID: 2, SubscribedGroups: make(map[string]bool)}
	hub.Register(overlap, 42, []string{"active:7", "edu:9"})
	hub.Register(oneTopic, 42, []string{"edu:9"})

	event := Event{Type: EventStudentCheckIn}
	if err := hub.BroadcastToGroups(42, []string{"active:7", "edu:9", "edu:9"}, event); err != nil {
		t.Fatalf("BroadcastToGroups() error = %v", err)
	}

	for name, client := range map[string]*Client{"overlap": overlap, "one topic": oneTopic} {
		select {
		case got := <-client.Channel:
			if got.Type != event.Type {
				t.Errorf("%s event type = %q, want %q", name, got.Type, event.Type)
			}
		default:
			t.Errorf("%s received no event", name)
		}
		select {
		case duplicate := <-client.Channel:
			t.Errorf("%s received duplicate event: %#v", name, duplicate)
		default:
		}
	}
}

// TestHubUnregister verifies client unregistration and cleanup
func TestHubUnregister(t *testing.T) {
	t.Parallel()

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
			hub := NewHub(slog.Default())
			clients := make([]*Client, tt.setupClients)

			// Register clients
			for i := 0; i < tt.setupClients; i++ {
				clients[i] = &Client{
					Channel:          make(chan Event, 10),
					UserID:           int64(i + 1),
					SubscribedGroups: make(map[string]bool),
				}
				hub.Register(clients[i], int64(42), []string{tt.groupID})
			}

			// Unregister specified client
			hub.Unregister(clients[tt.unregisterClient])

			// Verify client count
			if got := hub.GetClientCount(); got != tt.expectedClientCount {
				t.Errorf("GetClientCount() = %v, want %v", got, tt.expectedClientCount)
			}

			// Verify group subscriber count
			if got := hub.GetGroupSubscriberCount(int64(42), tt.groupID); got != tt.expectedGroupCount {
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
	t.Parallel()

	hub := NewHub(slog.Default())
	client := &Client{
		Channel:          make(chan Event, 10),
		UserID:           1,
		SubscribedGroups: make(map[string]bool),
	}

	// Unregister client that was never registered (should not panic)
	hub.Unregister(client)

	// Verify no clients registered
	if got := hub.GetClientCount(); got != 0 {
		t.Errorf("GetClientCount() = %v, want 0", got)
	}
}

// TestHubUnregisterCleanup verifies groupClients map cleanup
func TestHubUnregisterCleanup(t *testing.T) {
	t.Parallel()

	hub := NewHub(slog.Default())
	client := &Client{
		Channel:          make(chan Event, 10),
		UserID:           1,
		SubscribedGroups: make(map[string]bool),
	}

	hub.Register(client, int64(42), []string{"group_1", "group_2"})

	// Verify groups have subscribers
	if got := hub.GetGroupSubscriberCount(int64(42), "group_1"); got != 1 {
		t.Errorf("GetGroupSubscriberCount(group_1) = %v, want 1", got)
	}

	hub.Unregister(client)

	// Verify groups are cleaned up (no subscribers)
	if got := hub.GetGroupSubscriberCount(int64(42), "group_1"); got != 0 {
		t.Errorf("GetGroupSubscriberCount(group_1) after cleanup = %v, want 0", got)
	}

	if got := hub.GetGroupSubscriberCount(int64(42), "group_2"); got != 0 {
		t.Errorf("GetGroupSubscriberCount(group_2) after cleanup = %v, want 0", got)
	}

	// Verify internal map is cleaned up
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	if len(hub.groupClients) != 0 {
		t.Errorf("groupClients map should be empty, got %v entries", len(hub.groupClients))
	}
}

// TestHubBroadcastToSingleSubscriber verifies event delivery to one client
func TestHubBroadcastToSingleSubscriber(t *testing.T) {
	t.Parallel()

	hub := NewHub(slog.Default())
	client := &Client{
		Channel:          make(chan Event, 10),
		UserID:           1,
		SubscribedGroups: make(map[string]bool),
	}

	hub.Register(client, int64(42), []string{"group_1"})

	event := NewEvent(EventStudentCheckIn, "group_1", EventData{
		StudentID: strPtr("123"),
	})

	// Broadcast event
	err := hub.BroadcastToGroup(int64(42), "group_1", event)
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
	t.Parallel()

	hub := NewHub(slog.Default())

	// Register three clients to the same group
	clients := make([]*Client, 3)
	for i := 0; i < 3; i++ {
		clients[i] = &Client{
			Channel:          make(chan Event, 10),
			UserID:           int64(i + 1),
			SubscribedGroups: make(map[string]bool),
		}
		hub.Register(clients[i], int64(42), []string{"group_1"})
	}

	event := NewEvent(EventActivityStart, "group_1", EventData{
		ActivityName: strPtr("Test Activity"),
	})

	// Broadcast event
	err := hub.BroadcastToGroup(int64(42), "group_1", event)
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
	t.Parallel()

	hub := NewHub(slog.Default())

	client1 := &Client{
		Channel:          make(chan Event, 10),
		UserID:           1,
		SubscribedGroups: make(map[string]bool),
	}
	client2 := &Client{
		Channel:          make(chan Event, 10),
		UserID:           2,
		SubscribedGroups: make(map[string]bool),
	}

	hub.Register(client1, int64(42), []string{"group_1"})
	hub.Register(client2, int64(42), []string{"group_2"})

	event := NewEvent(EventStudentCheckIn, "group_1", EventData{
		StudentID: strPtr("123"),
	})

	// Broadcast to group_1
	err := hub.BroadcastToGroup(int64(42), "group_1", event)
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
	t.Parallel()

	hub := NewHub(slog.Default())

	event := NewEvent(EventStudentCheckIn, "group_nonexistent", EventData{
		StudentID: strPtr("123"),
	})

	// Broadcast to group with no subscribers (should not error)
	err := hub.BroadcastToGroup(int64(42), "group_nonexistent", event)
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
	t.Parallel()

	hub := NewHub(slog.Default())

	// Create client with very small buffer
	client := &Client{
		Channel:          make(chan Event, 1),
		UserID:           1,
		SubscribedGroups: make(map[string]bool),
	}

	hub.Register(client, int64(42), []string{"group_1"})

	// Fill the channel
	event1 := NewEvent(EventStudentCheckIn, "group_1", EventData{StudentID: strPtr("1")})
	client.Channel <- event1

	// Try to broadcast when channel is full (should not block or error)
	event2 := NewEvent(EventStudentCheckIn, "group_1", EventData{StudentID: strPtr("2")})
	err := hub.BroadcastToGroup(int64(42), "group_1", event2)
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
	t.Parallel()

	hub := NewHub(slog.Default())

	// Initially zero
	if got := hub.GetClientCount(); got != 0 {
		t.Errorf("Initial GetClientCount() = %v, want 0", got)
	}

	// Add clients
	for i := 0; i < 5; i++ {
		client := &Client{
			Channel:          make(chan Event, 10),
			UserID:           int64(i + 1),
			SubscribedGroups: make(map[string]bool),
		}
		hub.Register(client, int64(42), []string{"group_1"})
	}

	if got := hub.GetClientCount(); got != 5 {
		t.Errorf("After 5 registrations, GetClientCount() = %v, want 5", got)
	}
}

// TestHubGetGroupSubscriberCount verifies group subscriber counting
func TestHubGetGroupSubscriberCount(t *testing.T) {
	t.Parallel()

	hub := NewHub(slog.Default())

	// Non-existent group
	if got := hub.GetGroupSubscriberCount(int64(42), "nonexistent"); got != 0 {
		t.Errorf("GetGroupSubscriberCount(nonexistent) = %v, want 0", got)
	}

	// Add subscribers
	for i := 0; i < 3; i++ {
		client := &Client{
			Channel:          make(chan Event, 10),
			UserID:           int64(i + 1),
			SubscribedGroups: make(map[string]bool),
		}
		hub.Register(client, int64(42), []string{"group_1"})
	}

	if got := hub.GetGroupSubscriberCount(int64(42), "group_1"); got != 3 {
		t.Errorf("GetGroupSubscriberCount(group_1) = %v, want 3", got)
	}
}

// TestHubBroadcastToAllReachesAllClients verifies BroadcastToAll delivers to every client
func TestHubBroadcastToAllReachesAllClients(t *testing.T) {
	t.Parallel()

	hub := NewHub(slog.Default())

	// Register clients to different groups (and one with no groups)
	clients := make([]*Client, 3)
	for i := 0; i < 3; i++ {
		clients[i] = &Client{
			Channel:          make(chan Event, 10),
			UserID:           int64(i + 1),
			SubscribedGroups: make(map[string]bool),
		}
	}
	hub.Register(clients[0], int64(42), []string{"group_1"})
	hub.Register(clients[1], int64(42), []string{"group_2"})
	hub.Register(clients[2], int64(42), []string{}) // zero-topic client

	event := NewEvent(EventDashboardCountsChanged, "group_1", EventData{})

	err := hub.BroadcastToAll(event)
	if err != nil {
		t.Errorf("BroadcastToAll() error = %v", err)
	}

	for i, client := range clients {
		select {
		case received := <-client.Channel:
			if received.Type != EventDashboardCountsChanged {
				t.Errorf("Client %d: got type %v, want %v", i, received.Type, EventDashboardCountsChanged)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("Client %d: timeout waiting for broadcast-to-all event", i)
		}
	}
}

func TestHubBroadcastToTenantOnlyReachesTenantClients(t *testing.T) {
	t.Parallel()

	hub := NewHub(slog.Default())

	tenantClient := &Client{
		Channel:          make(chan Event, 10),
		UserID:           1,
		SubscribedGroups: make(map[string]bool),
	}
	sameTenantNoGroupClient := &Client{
		Channel:          make(chan Event, 10),
		UserID:           2,
		SubscribedGroups: make(map[string]bool),
	}
	otherTenantClient := &Client{
		Channel:          make(chan Event, 10),
		UserID:           3,
		SubscribedGroups: make(map[string]bool),
	}

	hub.Register(tenantClient, int64(42), []string{"group_1"})
	hub.Register(sameTenantNoGroupClient, int64(42), []string{})
	hub.Register(otherTenantClient, int64(84), []string{"group_1"})

	event := NewEvent(EventInstanceStarted, "group_1", EventData{})
	if err := hub.BroadcastToTenant(int64(42), event); err != nil {
		t.Errorf("BroadcastToTenant() error = %v", err)
	}

	for name, client := range map[string]*Client{
		"tenant subscribed": tenantClient,
		"tenant no group":   sameTenantNoGroupClient,
	} {
		select {
		case received := <-client.Channel:
			if received.Type != EventInstanceStarted {
				t.Errorf("%s: got type %v, want %v", name, received.Type, EventInstanceStarted)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("%s: timeout waiting for tenant event", name)
		}
	}

	select {
	case received := <-otherTenantClient.Channel:
		t.Errorf("other tenant received event: %v", received.Type)
	default:
	}
}

// TestHubBroadcastToAllNoClients verifies BroadcastToAll with empty hub
func TestHubBroadcastToAllNoClients(t *testing.T) {
	t.Parallel()

	hub := NewHub(slog.Default())

	err := hub.BroadcastToAll(NewEvent(EventDashboardCountsChanged, "", EventData{}))
	if err != nil {
		t.Errorf("BroadcastToAll() with no clients should return nil, got: %v", err)
	}
}

// TestHubBroadcastToAllSkipsFullChannel verifies non-blocking behavior
func TestHubBroadcastToAllSkipsFullChannel(t *testing.T) {
	t.Parallel()

	hub := NewHub(slog.Default())

	fullClient := &Client{
		Channel:          make(chan Event, 1),
		UserID:           1,
		SubscribedGroups: make(map[string]bool),
	}
	openClient := &Client{
		Channel:          make(chan Event, 10),
		UserID:           2,
		SubscribedGroups: make(map[string]bool),
	}
	hub.Register(fullClient, int64(42), []string{})
	hub.Register(openClient, int64(42), []string{})

	// Fill fullClient's channel
	fullClient.Channel <- NewEvent(EventStudentCheckIn, "g", EventData{})

	// BroadcastToAll should not block
	err := hub.BroadcastToAll(NewEvent(EventDashboardCountsChanged, "", EventData{}))
	if err != nil {
		t.Errorf("BroadcastToAll() should not error on full channel, got: %v", err)
	}

	// openClient got it
	select {
	case received := <-openClient.Channel:
		if received.Type != EventDashboardCountsChanged {
			t.Errorf("got type %v, want dashboard_counts_changed", received.Type)
		}
	default:
		t.Error("openClient should have received the event")
	}
}

// Helper function to create string pointers
func strPtr(s string) *string {
	return &s
}
