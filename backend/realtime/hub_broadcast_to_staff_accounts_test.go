package realtime

import (
	"testing"
)

func staffClient(accountID, tenantID int64, staffID int64) *Client {
	return &Client{
		Channel:          make(chan Event, 1),
		UserID:           staffID,
		AccountID:        accountID,
		SubscribedGroups: make(map[string]bool),
	}
}

func received(t *testing.T, client *Client) bool {
	t.Helper()
	select {
	case <-client.Channel:
		return true
	default:
		return false
	}
}

// TestBroadcastToStaffAccountsAddressesExactlyTheNamedAccounts pins the
// transport for personal notifications. Every case here is one where reaching
// the wrong tab would show someone a count derived from data they may not read.
func TestBroadcastToStaffAccountsAddressesExactlyTheNamedAccounts(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)

	addressed := staffClient(11, 100, 1)
	bystander := staffClient(22, 100, 2)
	hub.Register(addressed, 100, nil)
	hub.Register(bystander, 100, nil)

	require := func(cond bool, msg string) {
		t.Helper()
		if !cond {
			t.Fatal(msg)
		}
	}

	err := hub.BroadcastToStaffAccounts(100, []int64{11}, Event{Type: EventNotification})
	require(err == nil, "broadcast must not error")

	require(received(t, addressed), "the named account must receive the event")
	require(!received(t, bystander), "a personal notification must not reach a bystander")
}

// TestBroadcastToStaffAccountsIsTenantScoped is the cross-tenant guard: one
// person can hold staff sessions at several schools, and school A's counts must
// never land in school B's tab.
func TestBroadcastToStaffAccountsIsTenantScoped(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)

	atSchoolA := staffClient(11, 100, 1)
	atSchoolB := staffClient(11, 200, 2)
	hub.Register(atSchoolA, 100, nil)
	hub.Register(atSchoolB, 200, nil)

	if err := hub.BroadcastToStaffAccounts(100, []int64{11}, Event{Type: EventNotification}); err != nil {
		t.Fatalf("broadcast: %v", err)
	}

	if !received(t, atSchoolA) {
		t.Fatal("the session at the addressed school must receive the event")
	}
	if received(t, atSchoolB) {
		t.Fatal("the same account's session at another school must not receive it")
	}
}

// TestBroadcastToStaffAccountsSkipsUnregisteredClients guards the panic path:
// Unregister closes the channel, so an index entry left behind would make the
// next broadcast send on a closed channel.
func TestBroadcastToStaffAccountsSkipsUnregisteredClients(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)

	client := staffClient(11, 100, 1)
	hub.Register(client, 100, nil)
	hub.Unregister(client)

	if err := hub.BroadcastToStaffAccounts(100, []int64{11}, Event{Type: EventNotification}); err != nil {
		t.Fatalf("broadcast after unregister must be a no-op, got %v", err)
	}
}

// TestBroadcastToStaffAccountsEmptyRecipients pins that an empty recipient list
// stays empty. Widening it into a tenant-wide send is the worst possible
// interpretation of "nobody was addressed".
func TestBroadcastToStaffAccountsEmptyRecipients(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)

	client := staffClient(11, 100, 1)
	hub.Register(client, 100, nil)

	if err := hub.BroadcastToStaffAccounts(100, nil, Event{Type: EventNotification}); err != nil {
		t.Fatalf("broadcast: %v", err)
	}
	if received(t, client) {
		t.Fatal("an empty recipient list must reach nobody")
	}
}

// TestRegisterIgnoresZeroAccountID covers the effective admin without a staff
// record: their account id may be unknown, and bucketing them under 0 would
// merge every such admin across every school into one recipient.
func TestRegisterIgnoresZeroAccountID(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)

	anonymous := staffClient(0, 100, 0)
	hub.Register(anonymous, 100, nil)

	if err := hub.BroadcastToStaffAccounts(100, []int64{0}, Event{Type: EventNotification}); err != nil {
		t.Fatalf("broadcast: %v", err)
	}
	if received(t, anonymous) {
		t.Fatal("account id 0 must not be addressable")
	}
}
