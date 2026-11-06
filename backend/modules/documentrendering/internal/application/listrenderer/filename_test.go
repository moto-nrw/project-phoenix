package listexport

import "testing"

// The file name keeps its accents (#5791); the ASCII spelling for clients
// without filename* comes from textnorm.ContentDisposition at the HTTP edge.
func TestSafeFilenameKeepsAccents(t *testing.T) {
	t.Parallel()

	for input, want := range map[string]string{
		"Gruppe Ćirić":         "gruppe-ćirić",
		"Anmeldungen Müller":   "anmeldungen-müller",
		"École / Łódź: Straße": "école-łódź-straße",
		"  OGS Export Liste  ": "ogs-export-liste",
		"Gruppe Rodríguez":    "gruppe-rodríguez",
		"Liste »Nord« 2":       "liste-nord-2",
		"__":                   "",
	} {
		if got := safeFilename(input); got != want {
			t.Errorf("safeFilename(%q) = %q, want %q", input, got, want)
		}
	}
}
