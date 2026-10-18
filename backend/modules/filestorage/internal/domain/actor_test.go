package domain

import "testing"

func TestActorDisplayNameFallsBackToUnbekannt(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]string{"  Mia Meier ": "Mia Meier", "": "Unbekannt", "   ": "Unbekannt"} {
		if got := (Actor{Name: name}).DisplayName(); got != want {
			t.Errorf("DisplayName(%q) = %q, want %q", name, got, want)
		}
	}
}
