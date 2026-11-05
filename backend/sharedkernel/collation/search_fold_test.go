package collation

import "testing"

func TestFoldForSearchIgnoresAccentsAndCase(t *testing.T) {
	t.Parallel()

	for input, want := range map[string]string{
		"Gómez":         "gomez",
		"Özdemir":       "ozdemir",
		"Łukasz Ćirić":  "lukasz ciric",
		"Søren Đoković": "soren dokovic",
		"Straße":        "strasse",
		"Iİı":           "iii",
		"Zoë":          "zoe", // NFD input, as macOS pastes it
		"Müller":        "muller",
		"Raum 2b":       "raum 2b",
		"  Kim  ":       "  kim  ",
	} {
		if got := FoldForSearch(input); got != want {
			t.Errorf("FoldForSearch(%q) = %q, want %q", input, got, want)
		}
	}
}
