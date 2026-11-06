package domain

import "testing"

func TestInvitationNamesAreComposed(t *testing.T) {
	t.Parallel()

	first, last, err := InvitationRegistration{FirstName: " Jose\u0301 "}.Names(SchoolInvitation{LastName: ptr("Mu\u0308ller")})
	if err != nil {
		t.Fatalf("Names: %v", err)
	}
	if first != "José" || last != "Müller" {
		t.Fatalf("Names = %q, %q, want José, Müller", first, last)
	}
}

func ptr(value string) *string { return &value }
