package realtime

// Tests for Hub.BroadcastParentMessage — the recipient-scoped routing for the
// parent-OGS messaging trigger. The addressed guardian's portal client (matched
// on account id) is woken; staff clients are woken for their whole tenant (their
// inbox is access-filtered on refetch). These tests guard that one guardian
// never receives another guardian's trigger and that staff isolation holds.

import (
	"log/slog"
	"testing"
	"time"
)

func newParentClient(hub *Hub, accountID int64) *Client {
	c := &Client{
		Channel:          make(chan Event, 10),
		UserID:           accountID,
		SubscribedGroups: make(map[string]bool),
	}
	hub.RegisterParent(c)
	return c
}

func expectEvent(t *testing.T, c *Client, who string) {
	t.Helper()
	select {
	case got := <-c.Channel:
		if got.Type != EventParentMessage {
			t.Errorf("%s: got type %v, want %v", who, got.Type, EventParentMessage)
		}
	case <-time.After(100 * time.Millisecond):
		t.Errorf("%s should have received the parent_message trigger", who)
	}
}

func expectNoEvent(t *testing.T, c *Client, who string) {
	t.Helper()
	select {
	case <-c.Channel:
		t.Errorf("%s should NOT have received the parent_message trigger", who)
	case <-time.After(50 * time.Millisecond):
	}
}

// TestHubBroadcastParentMessage_RoutesByGuardianAndTenant — a trigger addressed
// to one guardian reaches that guardian and the tenant's staff, but never
// another guardian or another tenant's staff.
func TestHubBroadcastParentMessage_RoutesByGuardianAndTenant(t *testing.T) {
	hub := NewHub(slog.Default())

	guardianA := newParentClient(hub, 501) // account 501
	guardianB := newParentClient(hub, 502) // account 502

	staffTenant100 := &Client{Channel: make(chan Event, 10), UserID: 3, SubscribedGroups: make(map[string]bool)}
	staffTenant200 := &Client{Channel: make(chan Event, 10), UserID: 4, SubscribedGroups: make(map[string]bool)}
	hub.Register(staffTenant100, int64(100), []string{})
	hub.Register(staffTenant200, int64(200), []string{})

	gid := "501"
	event := NewEvent(EventParentMessage, "", EventData{Source: &gid})
	if err := hub.BroadcastParentMessage(int64(100), int64(501), event); err != nil {
		t.Fatalf("BroadcastParentMessage returned error: %v", err)
	}

	expectEvent(t, guardianA, "guardian 501")
	expectNoEvent(t, guardianB, "guardian 502")
	expectEvent(t, staffTenant100, "staff in tenant 100")
	expectNoEvent(t, staffTenant200, "staff in tenant 200")
}

// TestHubBroadcastParentMessage_SanitizesStaffCopy — the addressed guardian
// receives the full event (their own data), while every staff client gets a
// copy with thread_id, student_id, and source all cleared. ThreadID is opaque,
// but a staffer who knows a thread URL and later loses access to that child must
// not be able to correlate future triggers to that specific conversation.
func TestHubBroadcastParentMessage_SanitizesStaffCopy(t *testing.T) {
	hub := NewHub(slog.Default())

	guardian := newParentClient(hub, 501)
	staff := &Client{Channel: make(chan Event, 10), UserID: 3, SubscribedGroups: make(map[string]bool)}
	hub.Register(staff, int64(100), []string{})

	event := NewParentMessageEvent(int64(501), int64(77), int64(42))
	if err := hub.BroadcastParentMessage(int64(100), int64(501), event); err != nil {
		t.Fatalf("BroadcastParentMessage returned error: %v", err)
	}

	guardianGot := <-guardian.Channel
	if guardianGot.Data.ThreadID == nil || guardianGot.Data.StudentID == nil || guardianGot.Data.Source == nil {
		t.Errorf("guardian should receive the full event, got thread=%v student=%v source=%v",
			guardianGot.Data.ThreadID, guardianGot.Data.StudentID, guardianGot.Data.Source)
	}

	staffGot := <-staff.Channel
	if staffGot.Data.ThreadID != nil {
		t.Errorf("staff copy must strip thread_id, got %q", *staffGot.Data.ThreadID)
	}
	if staffGot.Data.StudentID != nil {
		t.Errorf("staff copy must strip student_id, got %q", *staffGot.Data.StudentID)
	}
	if staffGot.Data.Source != nil {
		t.Errorf("staff copy must strip source, got %q", *staffGot.Data.Source)
	}
}

// TestHubBroadcastParentMessage_EmptyHub — no clients connected must not panic.
func TestHubBroadcastParentMessage_EmptyHub(t *testing.T) {
	hub := NewHub(slog.Default())
	if err := hub.BroadcastParentMessage(int64(900), int64(901), NewEvent(EventParentMessage, "", EventData{})); err != nil {
		t.Errorf("BroadcastParentMessage on empty hub should return nil, got: %v", err)
	}
}
