package users

import "testing"

func TestParentAnnouncementResponseLockKey(t *testing.T) {
	key := parentAnnouncementResponseLockKey(123, 456)
	if key != "parent-announcement-response:123:456" {
		t.Fatalf("unexpected lock key %q", key)
	}
	if key == parentAnnouncementResponseLockKey(124, 456) {
		t.Fatal("different announcement IDs must use different lock keys")
	}
	if key == parentAnnouncementResponseLockKey(123, 457) {
		t.Fatal("different student IDs must use different lock keys")
	}
}
