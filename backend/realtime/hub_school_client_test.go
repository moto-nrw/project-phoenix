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
// connection contract (#2208): a Lehrkraft's tab is woken by the explicitly
// school-scoped personal fan-out for its own account at its own school, and by nothing that is
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
	if received(t, teacher) {
		t.Fatal("a staff-only fan-out must not reach a school client")
	}

	if err := hub.BroadcastToSchoolAccounts(200, []int64{11}, Event{Type: EventStaffMessage}); err != nil {
		t.Fatalf("school broadcast: %v", err)
	}
	if received(t, teacher) {
		t.Fatal("another school's school fan-out to the same account must not reach it")
	}

	if err := hub.BroadcastToSchoolAccounts(100, []int64{11}, Event{Type: EventStaffMessage}); err != nil {
		t.Fatalf("school broadcast: %v", err)
	}
	if !received(t, teacher) {
		t.Fatal("the school fan-out at the client's own school must reach it")
	}

	// Unregister removes the account index entry: a later fan-out must not
	// send on a closed channel.
	hub.Unregister(teacher)
	if err := hub.BroadcastToSchoolAccounts(100, []int64{11}, Event{Type: EventStaffMessage}); err != nil {
		t.Fatalf("school broadcast after unregister: %v", err)
	}
	if hub.GetClientCount() != 0 {
		t.Fatalf("expected no clients after unregister, got %d", hub.GetClientCount())
	}
}

// TestSchoolClientHearsThatAClassDayChanged pins the one tenant-wide signal
// moto schule receives (#4792): a student_updated at its own school reaches
// the school client as class_day_changed, without the source event's
// payload, so an open class view refetches after a new sick note.
func TestSchoolClientHearsThatAClassDayChanged(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)
	teacher := schoolClient(11)
	hub.RegisterSchool(teacher, 100)
	elsewhere := schoolClient(12)
	hub.RegisterSchool(elsewhere, 200)

	source := "parent"
	studentID := "7"
	if err := hub.BroadcastToTenant(100, NewEvent(EventStudentUpdated, "", EventData{Source: &source, StudentID: &studentID})); err != nil {
		t.Fatalf("tenant broadcast: %v", err)
	}
	select {
	case event := <-teacher.Channel:
		if event.Type != EventClassDayChanged {
			t.Fatalf("expected %s, got %s", EventClassDayChanged, event.Type)
		}
		if event.Data.StudentID != nil {
			t.Fatal("the school signal must not name the child")
		}
		if event.Data.Source == nil || *event.Data.Source != source {
			t.Fatal("the school signal keeps the source for log review")
		}
	default:
		t.Fatal("a student update at the client's school must reach it as class_day_changed")
	}
	if received(t, elsewhere) {
		t.Fatal("another school's student update must not reach a school client")
	}

	hub.Unregister(teacher)
	if err := hub.BroadcastToTenant(100, NewEvent(EventStudentUpdated, "", EventData{Source: &source})); err != nil {
		t.Fatalf("tenant broadcast after unregister: %v", err)
	}
}
