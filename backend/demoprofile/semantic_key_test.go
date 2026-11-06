package demoprofile

import "testing"

func TestSemanticKeyIsASCII(t *testing.T) {
	t.Parallel()

	for input, want := range map[string]string{
		"Chiara Rodríguez":       "chiara-rodriguez",
		"Chiara Rodri\u0301guez": "chiara-rodriguez",
		"Anna Müller":            "anna-mueller",
		"Ayşe Çelik":             "ayse-celik",
		"  Raum 1.02 / OG ":      "raum-1-02-og",
		"Katarzyna Schmidt":      "katarzyna-schmidt",
	} {
		if got := SemanticKey(input); got != want {
			t.Errorf("SemanticKey(%q) = %q, want %q", input, got, want)
		}
	}
}
