package domain

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestTraegerDocumentFileNameKeepsAnyNameWithinBounds pins the stored name of
// an upload: at most 255 bytes of valid UTF-8, even when the extension alone
// is longer than that (#4692 review).
func TestTraegerDocumentFileNameKeepsAnyNameWithinBounds(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"short":            "brief.pdf",
		"long stem":        strings.Repeat("ä", 200) + ".pdf",
		"overlong ext":     "file." + strings.Repeat("a", 256),
		"only a long ext":  "." + strings.Repeat("ö", 300),
		"path and control": "C:\\Users\\x\\a\x00b.pdf",
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := TraegerDocumentFileName(input)
			if len(got) > 255 || !utf8.ValidString(got) || got == "" {
				t.Fatalf("TraegerDocumentFileName(%q) = %q (%d bytes)", input, got, len(got))
			}
		})
	}
	if got := TraegerDocumentFileName(strings.Repeat("ä", 200) + ".pdf"); !strings.HasSuffix(got, ".pdf") {
		t.Fatalf("a long stem keeps its extension, got %q", got)
	}
}
