package realtime

// Tests for Hub.BroadcastToGuardian — the guardian-ONLY routing for a
// message-independent care-state invalidation (parent_child_updated). Unlike
// BroadcastParentMessage it must NOT fan a copy out to staff: staff neither
// listen for nor need this event, and the child-update flow calls it once per
// guardian, so a staff copy per guardian would flood staff channels (#1845
// review).

import (
	"log/slog"
	"testing"
	"time"
)

func expectChildUpdate(t *testing.T, c *Client, who string) {
	t.Helper()
	select {
	case got := <-c.Channel:
		if got.Type != EventParentChildUpdated {
			t.Errorf("%s: got type %v, want %v", who, got.Type, EventParentChildUpdated)
		}
	case <-time.After(100 * time.Millisecond):
		t.Errorf("%s should have received the parent_child_updated trigger", who)
	}
}

func expectNoAnyEvent(t *testing.T, c *Client, who string) {
	t.Helper()
	select {
	case got := <-c.Channel:
		t.Errorf("%s should NOT have received any event, got %v", who, got.Type)
	case <-time.After(50 * time.Millisecond):
	}
}

// TestHubBroadcastToGuardian_OnlyAddressedGuardian — the event reaches the
// addressed guardian's tabs and NO ONE else: not another guardian, and crucially
// not any staff client in the tenant.
func TestHubBroadcastToGuardian_OnlyAddressedGuardian(t *testing.T) {
	hub := NewHub(slog.Default())

	guardianA := newParentClient(hub, 501)
	guardianB := newParentClient(hub, 502)

	staff := &Client{Channel: make(chan Event, 10), UserID: 3, SubscribedGroups: make(map[string]bool)}
	hub.Register(staff, int64(100), []string{})

	event := NewParentChildUpdatedEvent(int64(501), int64(42))
	if err := hub.BroadcastToGuardian(int64(100), int64(501), event); err != nil {
		t.Fatalf("BroadcastToGuardian returned error: %v", err)
	}

	expectChildUpdate(t, guardianA, "guardian 501")
	expectNoAnyEvent(t, guardianB, "guardian 502")
	// The key regression: staff must receive nothing at all from this path.
	expectNoAnyEvent(t, staff, "staff in tenant 100")
}

// TestHubBroadcastToGuardian_EmptyHub — no clients connected must not panic.
func TestHubBroadcastToGuardian_EmptyHub(t *testing.T) {
	hub := NewHub(slog.Default())
	if err := hub.BroadcastToGuardian(int64(900), int64(901), NewEvent(EventParentChildUpdated, "", EventData{})); err != nil {
		t.Errorf("BroadcastToGuardian on empty hub should return nil, got: %v", err)
	}
}
