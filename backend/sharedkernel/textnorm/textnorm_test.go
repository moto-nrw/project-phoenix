package textnorm

import "testing"

func TestNameTrimsAndComposes(t *testing.T) {
	t.Parallel()

	for input, want := range map[string]string{
		"  José ":                "José",
		"Jose\u0301":             "José",
		"\tMu\u0308ller\n":       "Müller",
		"C\u0301iric\u0301":      "Ćirić",
		"Özdemir":                "Özdemir",
		"":                       "",
		"   ":                    "",
		"Anna-Lena von der Bach": "Anna-Lena von der Bach",
	} {
		if got := Name(input); got != want {
			t.Errorf("Name(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestASCIITransliteratesLatinLetters(t *testing.T) {
	t.Parallel()

	for input, want := range map[string]string{
		"École":            "Ecole",
		"Gruppe Ćirić":     "Gruppe Ciric",
		"Rodríguez":        "Rodriguez",
		"Müller Straße":    "Mueller Strasse",
		"ÄÖÜ äöü":          "AeOeUe aeoeue",
		"Mu\u0308ller":     "Mueller",
		"Łukasz Øster":     "Lukasz Oster",
		"Đorđe ı":          "Dorde i",
		"Æsir Œuvre":       "AEsir OEuvre",
		"Ayşe Çelik":       "Ayse Celik",
		"OGS »Nord« 2":     "OGS »Nord« 2",
		"学校":               "学校",
		"plain ascii-1.0/": "plain ascii-1.0/",
	} {
		if got := ASCII(input); got != want {
			t.Errorf("ASCII(%q) = %q, want %q", input, got, want)
		}
	}
}
