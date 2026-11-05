package application

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Parents type names on their own devices; a decomposed name (macOS
// copy-paste) is stored composed (#4949).
func TestSubmittedNamesAreStoredComposed(t *testing.T) {
	t.Parallel()

	child := newSubmittedChild(7, 0, SubmitChild{FirstName: " Zoe\u0308 ", LastName: "Go\u0301mez", DateOfBirth: calendar.NewDate(2019, 3, 4)}, nil)
	if child.FirstName != "Zoë" || child.LastName != "Gómez" {
		t.Fatalf("child names = %q, %q", child.FirstName, child.LastName)
	}

	guardian, keep, err := normalizeAdditionalGuardian(0, SubmitGuardian{FirstName: "Rene\u0301e", LastName: " C\u0327elik "})
	if err != nil || !keep {
		t.Fatalf("normalizeAdditionalGuardian: keep=%v err=%v", keep, err)
	}
	if guardian.FirstName != "Renée" || guardian.LastName != "Çelik" {
		t.Fatalf("guardian names = %q, %q", guardian.FirstName, guardian.LastName)
	}
}
