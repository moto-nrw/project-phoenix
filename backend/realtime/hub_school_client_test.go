package realtime

import "testing"

func schoolClient(accountID int64) *Client {
	return &Client{
		Channel:          make(chan Event, 1),
		UserID:           accountID,
		AccountID:        accountID,
		SubscribedGroups: make(map[string]bool),
	}
}

// TestSchoolClientReceivesOnlyAccountAddressedEvents pins the school-portal
// connection contract (#2208): a Lehrkraft's tab is woken by the personal
// fan-out for its own account at its own school, and by nothing that is
// staff-portal data (tenant-wide and global refreshes).
func TestSchoolClientReceivesOnlyAccountAddressedEvents(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)
	teacher := schoolClient(11)
	hub.RegisterSchool(teacher, 100)

	if err := hub.BroadcastToTenant(100, Event{Type: EventNotification}); err != nil {
		t.Fatalf("tenant broadcast: %v", err)
	}
	if received(t, teacher) {
		t.Fatal("a tenant-wide refresh must not reach a school client")
	}

	if err := hub.BroadcastToAll(Event{Type: EventNotification}); err != nil {
		t.Fatalf("broadcast to all: %v", err)
	}
	if received(t, teacher) {
		t.Fatal("a global refresh must not reach a school client")
	}

	if err := hub.BroadcastToStaffAccounts(200, []int64{11}, Event{Type: EventStaffMessage}); err != nil {
		t.Fatalf("staff broadcast: %v", err)
	}
	if received(t, teacher) {
		t.Fatal("another school's fan-out to the same account must not reach it")
	}

	if err := hub.BroadcastToStaffAccounts(100, []int64{11}, Event{Type: EventStaffMessage}); err != nil {
		t.Fatalf("staff broadcast: %v", err)
	}
	if !received(t, teacher) {
		t.Fatal("the personal fan-out at the client's own school must reach it")
	}

	// Unregister removes the account index entry: a later fan-out must not
	// send on a closed channel.
	hub.Unregister(teacher)
	if err := hub.BroadcastToStaffAccounts(100, []int64{11}, Event{Type: EventStaffMessage}); err != nil {
		t.Fatalf("staff broadcast after unregister: %v", err)
	}
	if hub.GetClientCount() != 0 {
		t.Fatalf("expected no clients after unregister, got %d", hub.GetClientCount())
	}
}
