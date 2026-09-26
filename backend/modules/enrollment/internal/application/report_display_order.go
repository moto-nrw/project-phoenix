package application

import (
	"strings"
	"sync"
	"unicode"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

// The reports sort names in German dictionary order (DIN 5007-1: umlauts
// group with their base letter, case is ignored, diacritics stay
// significant) and classes grade-aware, like the frontend's
// localeCompare(..., "de") and every other class list. The settings are
// those of internal/collation, which the architecture policy keeps out of an
// owner's application.

// germanCollators pools collators: a collate.Collator keeps per-comparison
// state and is not safe for concurrent use.
var germanCollators = sync.Pool{
	New: func() any {
		return collate.New(language.German, collate.IgnoreCase)
	},
}

// compareGerman compares two strings case-insensitively in German
// dictionary order. It returns -1, 0, or +1.
func compareGerman(a, b string) int {
	c := germanCollators.Get().(*collate.Collator)
	defer germanCollators.Put(c)
	return c.CompareString(a, b)
}

// compareGermanNames compares two person names by last name, then first
// name.
func compareGermanNames(aLast, aFirst, bLast, bFirst string) int {
	if r := compareGerman(aLast, bLast); r != 0 {
		return r
	}
	return compareGerman(aFirst, bFirst)
}

// compareSchoolClasses compares two school class names for display order.
// Labels with a leading grade number — with or without the display prefix
// "Klasse" — sort before labels without one and compare numerically among
// themselves ("2a" before "10a"), tie-breaking on the remainder; labels
// without a grade compare in German order. Empty names sort last. A result of
// 0 means the same logical class ("1a"/"1A", "1 a"/"1a", "Klasse 2a"/"2a").
func compareSchoolClasses(a, b string) int {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	switch {
	case a == "" && b == "":
		return 0
	case a == "":
		return 1
	case b == "":
		return -1
	}
	aNum, aRest, aOK := splitClassGrade(a)
	bNum, bRest, bOK := splitClassGrade(b)
	switch {
	case aOK && bOK:
		if aNum != bNum {
			if aNum < bNum {
				return -1
			}
			return 1
		}
		return compareGerman(aRest, bRest)
	case aOK:
		return -1
	case bOK:
		return 1
	}
	return compareGerman(a, b)
}

// splitClassGrade extracts the leading grade number, also accepting the
// display prefix "Klasse" ("Klasse 2a" → 2, "a"). A label where no digit
// follows the prefix ("Klassenfahrt") reports !ok.
func splitClassGrade(s string) (num int, rest string, ok bool) {
	if num, rest, ok = splitLeadingNumber(s); ok {
		return num, rest, true
	}
	const prefix = "klasse"
	if len(s) > len(prefix) && strings.EqualFold(s[:len(prefix)], prefix) {
		if num, rest, ok = splitLeadingNumber(strings.TrimLeftFunc(s[len(prefix):], unicode.IsSpace)); ok {
			return num, rest, true
		}
	}
	return 0, s, false
}

// splitLeadingNumber extracts a leading run of ASCII digits as an int. ok is
// false when the string does not start with a digit or the number would
// overflow a reasonable grade value.
func splitLeadingNumber(s string) (num int, rest string, ok bool) {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 || i > 9 {
		return 0, s, false
	}
	for _, c := range s[:i] {
		num = num*10 + int(c-'0')
	}
	return num, strings.TrimLeftFunc(s[i:], unicode.IsSpace), true
}
