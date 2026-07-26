package realtime

// Tests for Hub.BroadcastToTenant — pinning the tenant-scoped fan-out
// contract used by the post-commit settings hook (e.g. the photo-purge
// SSE notification when a school disables operations.student_photos_enabled).
//
// The hub doesn't keep a tenant→clients index; BroadcastToTenant walks
// the full client map and filters on Client.TenantID. These tests guard:
//   - Tenant isolation: only matching clients receive the event.
//   - Channel-full: a slow consumer is skipped, others still receive.
//   - Empty hub: returns nil, no panic, no recipient.

import (
	"log/slog"
	"testing"
	"time"
)

// TestHubBroadcastToTenant_OnlyMatchingTenantReceives — two clients in
// different tenants; a tenant-scoped broadcast must reach exactly the
// matching client and not leak to the other.
func TestHubBroadcastToTenant_OnlyMatchingTenantReceives(t *testing.T) {
	hub := NewHub(slog.Default())

	clientA := &Client{
		Channel:          make(chan Event, 10),
		UserID:           1,
		SubscribedGroups: make(map[string]bool),
	}
	clientB := &Client{
		Channel:          make(chan Event, 10),
		UserID:           2,
		SubscribedGroups: make(map[string]bool),
	}
	hub.Register(clientA, int64(100), []string{})
	hub.Register(clientB, int64(200), []string{})

	event := NewEvent(EventDashboardCountsChanged, "", EventData{})

	if err := hub.BroadcastToTenant(int64(100), event); err != nil {
		t.Fatalf("BroadcastToTenant returned error: %v", err)
	}

	// clientA (tenant 100) must receive the event.
	select {
	case received := <-clientA.Channel:
		if received.Type != EventDashboardCountsChanged {
			t.Errorf("clientA: got type %v, want %v", received.Type, EventDashboardCountsChanged)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("clientA should have received the tenant-100 broadcast")
	}

	// clientB (tenant 200) must NOT receive it.
	select {
	case <-clientB.Channel:
		t.Error("clientB should NOT receive a tenant-100 broadcast")
	case <-time.After(50 * time.Millisecond):
		// expected timeout — no event
	}
}

func TestHubBroadcastToTenantAdmins_ExcludesScopedStaffAndOtherTenants(t *testing.T) {
	hub := NewHub(slog.Default())

	admin := &Client{
		Channel:          make(chan Event, 1),
		UserID:           1,
		IsAdmin:          true,
		SubscribedGroups: make(map[string]bool),
	}
	caregiver := &Client{
		Channel:          make(chan Event, 1),
		UserID:           2,
		SubscribedGroups: make(map[string]bool),
	}
	otherTenantAdmin := &Client{
		Channel:          make(chan Event, 1),
		UserID:           3,
		IsAdmin:          true,
		SubscribedGroups: make(map[string]bool),
	}
	hub.Register(admin, int64(100), nil)
	hub.Register(caregiver, int64(100), nil)
	hub.Register(otherTenantAdmin, int64(200), nil)

	event := NewEvent(EventNotification, "", EventData{})
	if err := hub.BroadcastToTenantAdmins(int64(100), event); err != nil {
		t.Fatalf("BroadcastToTenantAdmins returned error: %v", err)
	}

	select {
	case got := <-admin.Channel:
		if got.Type != EventNotification {
			t.Errorf("admin got type %v, want %v", got.Type, EventNotification)
		}
	default:
		t.Fatal("admin should have received the admin-scoped broadcast")
	}
	for name, client := range map[string]*Client{
		"caregiver":          caregiver,
		"other tenant admin": otherTenantAdmin,
	} {
		select {
		case <-client.Channel:
			t.Errorf("%s should not receive the admin-scoped broadcast", name)
		default:
		}
	}
}

// TestHubBroadcastToTenant_NoClients — broadcasting on an empty hub
// or to a tenant with no connected clients must return nil and not panic.
func TestHubBroadcastToTenant_NoClients(t *testing.T) {
	hub := NewHub(slog.Default())

	if err := hub.BroadcastToTenant(int64(42), NewEvent(EventDashboardCountsChanged, "", EventData{})); err != nil {
		t.Errorf("BroadcastToTenant on empty hub should return nil, got: %v", err)
	}

	// Also: tenant has clients in OTHER tenants but none in this one.
	other := &Client{
		Channel:          make(chan Event, 10),
		UserID:           99,
		SubscribedGroups: make(map[string]bool),
	}
	hub.Register(other, int64(7), []string{})
	if err := hub.BroadcastToTenant(int64(42), NewEvent(EventDashboardCountsChanged, "", EventData{})); err != nil {
		t.Errorf("BroadcastToTenant with no matching tenant should return nil, got: %v", err)
	}
	// the other-tenant client must remain untouched
	select {
	case <-other.Channel:
		t.Error("non-matching tenant client must not receive the broadcast")
	case <-time.After(50 * time.Millisecond):
	}
}

// TestHubBroadcastToTenant_SkipsFullChannel — a tenant client whose
// channel buffer is full must be skipped without blocking the broadcast,
// and other clients in the same tenant must still receive the event.
// Without this, a single slow SSE consumer could hold up the whole
// fan-out path that runs inside the post-commit settings hook.
func TestHubBroadcastToTenant_SkipsFullChannel(t *testing.T) {
	hub := NewHub(slog.Default())

	full := &Client{
		Channel:          make(chan Event, 1),
		UserID:           1,
		SubscribedGroups: make(map[string]bool),
	}
	open := &Client{
		Channel:          make(chan Event, 10),
		UserID:           2,
		SubscribedGroups: make(map[string]bool),
	}
	hub.Register(full, int64(42), []string{})
	hub.Register(open, int64(42), []string{})

	// Fill the slow client's buffer.
	full.Channel <- NewEvent(EventStudentCheckIn, "g", EventData{})

	if err := hub.BroadcastToTenant(int64(42), NewEvent(EventDashboardCountsChanged, "", EventData{})); err != nil {
		t.Errorf("BroadcastToTenant must not error on full channel, got: %v", err)
	}

	// Open client must have received the event.
	select {
	case received := <-open.Channel:
		if received.Type != EventDashboardCountsChanged {
			t.Errorf("open client got type %v, want %v", received.Type, EventDashboardCountsChanged)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("open client should have received the broadcast")
	}

	// Full client's first event still in buffer (the new one was skipped).
	first := <-full.Channel
	if first.Type != EventStudentCheckIn {
		t.Errorf("full client first event = %v, want %v", first.Type, EventStudentCheckIn)
	}
	select {
	case <-full.Channel:
		t.Error("full client's channel should be empty after consuming the pre-existing event (the broadcast was skipped)")
	default:
	}
}

// TestHubBroadcastToTenant_MultipleSameTenant — every client whose
// TenantID matches must receive the event regardless of group
// subscription. Mirrors the photo-purge use case where every staff
// SSE session for a school must learn about the tenant-wide change.
func TestHubBroadcastToTenant_MultipleSameTenant(t *testing.T) {
	hub := NewHub(slog.Default())

	clients := make([]*Client, 3)
	for i := range clients {
		clients[i] = &Client{
			Channel:          make(chan Event, 10),
			UserID:           int64(i + 1),
			SubscribedGroups: make(map[string]bool),
		}
	}
	// Mix subscriptions: groups vs none. BroadcastToTenant must reach all.
	hub.Register(clients[0], int64(42), []string{"g1"})
	hub.Register(clients[1], int64(42), []string{"g2"})
	hub.Register(clients[2], int64(42), []string{}) // zero-topic client

	event := NewEvent(EventDashboardCountsChanged, "", EventData{})
	if err := hub.BroadcastToTenant(int64(42), event); err != nil {
		t.Fatalf("BroadcastToTenant error: %v", err)
	}

	for i, c := range clients {
		select {
		case got := <-c.Channel:
			if got.Type != EventDashboardCountsChanged {
				t.Errorf("client %d: got type %v, want %v", i, got.Type, EventDashboardCountsChanged)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("client %d: timeout waiting for tenant broadcast", i)
		}
	}
}
