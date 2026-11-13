package peopledirectory_test

import (
	"testing"

	peopleModule "github.com/moto-ogs/project-phoenix/modules/peopledirectory"
)

// A name pasted in decomposed form (macOS) is stored composed, so search and
// sorting see one spelling (#4949).
func TestPersonNamesAreStoredTrimmedAndComposed(t *testing.T) {
	t.Parallel()

	first, last := peopleModule.NormalizePersonNames(" Jose\u0301 ", "Mu\u0308ller\t")
	if first != "José" || last != "Müller" {
		t.Fatalf("NormalizePersonNames = %q, %q, want José, Müller", first, last)
	}

	created, err := peopleModule.NormalizePersonCreate(peopleModule.CreatePerson{FirstName: "Zoe\u0308", LastName: "C\u0301iric\u0301"})
	if err != nil {
		t.Fatalf("NormalizePersonCreate: %v", err)
	}
	if created.FirstName != "Zoë" || created.LastName != "Ćirić" {
		t.Fatalf("NormalizePersonCreate names = %q, %q", created.FirstName, created.LastName)
	}

	person := peopleModule.DirectoryPersonSnapshot{FirstName: "Luci\u0301a", LastName: " Go\u0301mez "}
	if err := person.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if person.FirstName != "Lucía" || person.LastName != "Gómez" {
		t.Fatalf("DirectoryPersonSnapshot.Validate names = %q, %q", person.FirstName, person.LastName)
	}

	guardian := peopleModule.GuardianProfileSnapshot{FirstName: " Ayşe ", LastName: "C\u0327elik"}
	if err := guardian.Validate(); err != nil {
		t.Fatalf("GuardianProfileSnapshot.Validate: %v", err)
	}
	if guardian.FirstName != "Ayşe" || guardian.LastName != "Çelik" {
		t.Fatalf("GuardianProfileSnapshot.Validate names = %q, %q", guardian.FirstName, guardian.LastName)
	}
}
