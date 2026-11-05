package listexport

import "testing"

func TestSafeFilenameTransliteratesAccents(t *testing.T) {
	t.Parallel()

	for input, want := range map[string]string{
		"Gruppe Ćirić":           "gruppe-ciric",
		"Anmeldungen Müller":     "anmeldungen-mueller",
		"École / Łódź: Straße":   "ecole-lodz-strasse",
		"  OGS Export Liste  ":   "ogs-export-liste",
		"Gruppe Rodri\u0301guez": "gruppe-rodriguez",
	} {
		if got := safeFilename(input); got != want {
			t.Errorf("safeFilename(%q) = %q, want %q", input, got, want)
		}
	}
}
