package collation

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// searchLetters maps the letters that carry their accent without a Unicode
// decomposition, so stripping combining marks leaves them unchanged. ß has
// no single-letter base and folds to "ss", the ligatures æ and œ to "ae" and
// "oe" (#5791), as PostgreSQL's unaccent does; ǽ decomposes to æ first.
var searchLetters = strings.NewReplacer(
	"ł", "l", "ø", "o", "đ", "d", "ı", "i", "ß", "ss", "æ", "ae", "œ", "oe",
)

// FoldForSearch reduces s to the form a name search compares: lowercase and
// without accents, so "gomez" finds "Gómez" and "ozdem" finds "Özdemir"
// (#4949). It matches public.f_unaccent with ILIKE in the database and
// foldForSearch in the frontend. Umlauts fold to their base letter: "Müller"
// becomes "muller", not "mueller". Fold both sides before comparing.
func FoldForSearch(s string) string {
	decomposed := norm.NFD.String(strings.ToLower(s))
	var b strings.Builder
	b.Grow(len(decomposed))
	for _, r := range decomposed {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(r)
		}
	}
	return searchLetters.Replace(b.String())
}
