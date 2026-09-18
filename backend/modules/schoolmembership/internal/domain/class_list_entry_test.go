package domain

import "testing"

// The display value is the audit trail's record of who the entry named, so
// it must read the same whatever whitespace the form carried.
func TestClassListEntryDisplayValueTrimsEveryPart(t *testing.T) {
	t.Parallel()

	got := ClassListEntryDisplayValue(ClassListEntryFields{FirstName: " Zoe ", LastName: " Aalders ", SchoolClass: " 1a "})
	if got != "Zoe Aalders (1a)" {
		t.Fatalf("display value = %q, want %q", got, "Zoe Aalders (1a)")
	}
}
