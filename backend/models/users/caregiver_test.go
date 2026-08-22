package users

import "testing"

func TestActiveCaregiverFullName(t *testing.T) {
	t.Parallel()

	caregiver := &ActiveCaregiver{
		FirstName: "Ada",
		LastName:  "Lovelace",
	}

	if got := caregiver.FullName(); got != "Ada Lovelace" {
		t.Fatalf("FullName() = %q, want %q", got, "Ada Lovelace")
	}
}
