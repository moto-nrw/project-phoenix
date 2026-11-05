package staff

import "testing"

// The staff list's name filters find "Gómez" for "gomez" (#4949).
func TestMatchesNameFilterIgnoresAccents(t *testing.T) {
	t.Parallel()
	person := &Person{FirstName: "Zoë", LastName: "Gómez"}
	if !matchesNameFilter(person, "zoe", "gomez") {
		t.Error("matchesNameFilter must ignore accents")
	}
	if matchesNameFilter(person, "", "becker") {
		t.Error("matchesNameFilter must still reject other names")
	}
}
