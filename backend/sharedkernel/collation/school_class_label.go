package collation

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// SchoolClassLabel names a school class for display: "1b" → "Klasse 1b".
// Class names are free text; some schools store "1b", others "Klasse 1b"
// or "klasse 1b", and none of them may read "Klasse Klasse 1b" (#5101).
// A leading "Klasse" (any case, followed by a non-letter or nothing)
// is normalized to "Klasse "; a grade-only name that ends with the word
// ("1. Klasse") stays as written. An empty name labels nothing.
//
// This is the only backend place that puts "Klasse" in front of a class
// name. The frontend twin is schoolClassLabel in
// frontend/src/lib/school-class-label.ts; both follow the same rule.
func SchoolClassLabel(schoolClass string) string {
	name := strings.TrimSpace(schoolClass)
	if name == "" {
		return ""
	}
	if hasTrailingClassWord(name) {
		return name
	}
	if rest, ok := cutClassPrefix(name); ok {
		name = rest
	}
	if name == "" {
		return "Klasse"
	}
	return "Klasse " + name
}

// cutClassPrefix strips a leading "Klasse" that is followed by a non-letter
// or the end of the name. "Klassenfahrt" keeps its prefix.
func cutClassPrefix(name string) (string, bool) {
	const prefix = "klasse"
	if len(name) < len(prefix) || !strings.EqualFold(name[:len(prefix)], prefix) {
		return name, false
	}
	rest := name[len(prefix):]
	if rest != "" {
		next, _ := utf8.DecodeRuneInString(rest)
		if unicode.IsLetter(next) {
			return name, false
		}
	}
	return strings.TrimLeftFunc(rest, unicode.IsSpace), true
}

// hasTrailingClassWord reports a grade-only name such as "1. Klasse".
func hasTrailingClassWord(name string) bool {
	const word = "klasse"
	if len(name) <= len(word) || !strings.EqualFold(name[len(name)-len(word):], word) {
		return false
	}
	before, _ := utf8.DecodeLastRuneInString(name[:len(name)-len(word)])
	return unicode.IsSpace(before)
}
