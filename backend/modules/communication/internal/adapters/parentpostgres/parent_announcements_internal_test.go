package parentpostgres

import "testing"

// TestResponseLockKey pins the advisory-lock key that serializes poll answer
// replacements: one key per (poll, child), so two guardians of the same
// child queue behind each other while different children never collide.
func TestResponseLockKey(t *testing.T) {
	t.Parallel()

	key := responseLockKey(123, 456)
	if key != "parent-announcement-response:123:456" {
		t.Fatalf("unexpected lock key %q", key)
	}
	if key == responseLockKey(124, 456) {
		t.Fatal("different announcement IDs must use different lock keys")
	}
	if key == responseLockKey(123, 457) {
		t.Fatal("different student IDs must use different lock keys")
	}
}
