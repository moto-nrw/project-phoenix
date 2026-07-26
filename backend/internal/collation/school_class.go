package collation

import (
	"strings"
	"unicode"
)

// CompareSchoolClasses compares two school class names (e.g. "1a", "2b",
// "10a", "Klasse 2a") for display order. Labels with a leading grade number —
// with or without the display prefix "Klasse" — sort before labels without
// one, compare numerically among themselves so "2a" sorts before "10a"
// (plain collation would put "10a" first), and tie-break on the remainder
// via CompareGerman. Labels without a grade compare via CompareGerman.
// Comparing across the two forms by original string would be intransitive
// ("2a" == "Klasse 2a" but "2a" < "Eulen" < "Klasse 2a"), so the grade
// bucket always wins instead. Empty names sort last so students without a
// class end up at the bottom of grouped lists.
//
// A result of 0 is the equivalence rule for "same logical class": case
// variants ("1a"/"1A"), spacing variants ("1 a"/"1a"), and prefix variants
// ("Klasse 2a"/"2a") all compare equal. Grouped exports rely on this to
// merge such variants into one section.
func CompareSchoolClasses(a, b string) int {
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
		return CompareGerman(aRest, bRest)
	case aOK: // grade labels sort before non-grade labels
		return -1
	case bOK:
		return 1
	}
	return CompareGerman(a, b)
}

// splitClassGrade extracts the leading grade number, also accepting the
// supported display prefix "Klasse" (e.g. "Klasse 2a" → 2, "a"). Labels
// where no digit follows the prefix (e.g. "Klassenfahrt") report !ok and
// stay on the string-collation path.
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

// splitLeadingNumber extracts a leading run of ASCII digits as an int.
// ok is false when the string does not start with a digit or the number
// would overflow a reasonable grade value.
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
