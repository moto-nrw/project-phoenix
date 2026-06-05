package listexport

import "testing"

// TestTransliterateRune locks the WinAnsi fallback: non-German Latin
// names must degrade to their nearest ASCII form (so the print export
// stays legible for the diverse families an OGS enrolls) rather than to
// '?', while genuinely unrepresentable scripts still fall through.
func TestTransliterateRune(t *testing.T) {
	latin := map[rune]string{
		'ş': "s", 'ğ': "g", 'ı': "i", // Turkish
		'ą': "a", 'ć': "c", 'ł': "l", 'ż': "z", 'ś': "s", // Polish
		'ñ': "n", 'ç': "c", // Spanish/French
		'ø': "o", 'æ': "ae", 'œ': "oe", 'đ': "d", // non-decomposing
	}
	for r, want := range latin {
		got, ok := transliterateRune(r)
		if !ok || got != want {
			t.Errorf("transliterateRune(%q) = (%q, %v); want (%q, true)", r, got, ok, want)
		}
	}

	// Scripts with no ASCII equivalent must report ok=false (caller emits '?').
	for _, r := range []rune{'ж', '好', '😀', 'م'} {
		if got, ok := transliterateRune(r); ok {
			t.Errorf("transliterateRune(%q) = (%q, true); want ok=false", r, got)
		}
	}

	// German umlauts/ß are handled directly by writePDFEncodedRune (WinAnsi
	// bytes), never reaching the transliterator — full-string sanity check.
	if got := pdfLiteralString("Łukasz Müller-Wiśniewski"); got != "(Lukasz M\xfcller-Wisniewski)" {
		t.Errorf("pdfLiteralString = %q", got)
	}
}
