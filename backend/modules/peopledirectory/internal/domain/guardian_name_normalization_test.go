package domain

import "testing"

func TestNormalizeGuardianContactComposesNames(t *testing.T) {
	t.Parallel()

	contact, err := NormalizeGuardianContact(GuardianContact{FirstName: " Rene\u0301e ", LastName: "Ło\u0301dz\u0301"})
	if err != nil {
		t.Fatalf("NormalizeGuardianContact: %v", err)
	}
	if contact.FirstName != "Renée" || contact.LastName != "Łódź" {
		t.Fatalf("names = %q, %q, want Renée, Łódź", contact.FirstName, contact.LastName)
	}
}
