// Package collation provides German-locale string comparison for
// user-visible name sorting (class lists, exports). It uses CLDR
// dictionary order (DIN 5007-1): umlauts group with their base letter
// (Ärmel sorts between Anders and Becker) instead of after 'z' as with
// byte-order comparison. This matches the frontend's
// localeCompare(..., "de"), so UI lists and exports agree.
package collation

import (
	"sync"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

// collate.Collator is not safe for concurrent use (it keeps per-comparison
// iterator state), so collators are pooled instead of shared.
var germanCollators = sync.Pool{
	New: func() any {
		// IgnoreCase keeps diacritics significant (Müller != Muller)
		// while ignoring letter case; Loose would drop diacritics too.
		return collate.New(language.German, collate.IgnoreCase)
	},
}

// CompareGerman compares two strings case-insensitively in German
// dictionary order. It returns -1, 0, or +1.
func CompareGerman(a, b string) int {
	c := germanCollators.Get().(*collate.Collator)
	defer germanCollators.Put(c)
	return c.CompareString(a, b)
}

// CompareGermanNames compares two person names by last name, then first
// name, using CompareGerman for both legs.
func CompareGermanNames(aLast, aFirst, bLast, bFirst string) int {
	if r := CompareGerman(aLast, bLast); r != 0 {
		return r
	}
	return CompareGerman(aFirst, bFirst)
}
