package collation

import (
	"strings"
	"unicode"
)

// CompareSchoolClasses compares two school class names (e.g. "1a", "2b",
// "10a") for display order. Leading grade numbers compare numerically so
// "2a" sorts before "10a" (plain collation would put "10a" first); the
// remainder falls back to CompareGerman. Empty names sort last so students
// without a class end up at the bottom of grouped lists.
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
	aNum, aRest, aOK := splitLeadingNumber(a)
	bNum, bRest, bOK := splitLeadingNumber(b)
	if aOK && bOK {
		if aNum != bNum {
			if aNum < bNum {
				return -1
			}
			return 1
		}
		return CompareGerman(aRest, bRest)
	}
	return CompareGerman(a, b)
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
