package importpkg

import (
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// matchKey folds a matched input so that two inputs the owners' SQL matching
// treats as equal always share a key (ADR 0067 addendum of 2026-11-08, point
// 2: a segment holds independent rows only). The owners compare
// LOWER(NORMALIZE(BTRIM(x), NFC)) for names and plain LOWER or equality for
// e-mail and card; matchKey takes the same steps (trim spaces, NFC, lower
// case) and then keeps only the ASCII letters of the canonical decomposition.
// Two inputs equal after the owners' steps are therefore equal here; the
// converse need not hold, so "Müller" and "Muller" share a key and only split
// a segment more often than needed. Go's simple lower-case mapping differs
// from PostgreSQL's full one only in letters that leave no ASCII letter
// behind (U+0130 lowers to "i" here and to "i" plus a combining dot there).
func matchKey(value string) string {
	lowered := norm.NFD.String(strings.ToLower(norm.NFC.String(strings.Trim(value, " "))))
	var key strings.Builder
	key.Grow(len(lowered))
	for _, r := range lowered {
		if r < utf8.RuneSelf {
			key.WriteRune(r)
		}
	}
	return key.String()
}
