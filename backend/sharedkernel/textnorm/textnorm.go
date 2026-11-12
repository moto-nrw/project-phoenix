// Package textnorm normalizes user-typed text: names are stored trimmed and
// in Unicode NFC, and identifiers that must be ASCII (seed keys, e-mail
// local parts, file names, slugs) share one transliteration (#4949).
package textnorm

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Name trims a person-like name and composes it to NFC, so a name pasted in
// decomposed form (macOS) compares, searches and sorts like the typed one.
func Name(value string) string {
	return norm.NFC.String(strings.TrimSpace(value))
}

// asciiLetters spells out letters that are not a base letter plus combining
// marks. German umlauts become ae/oe/ue, the spelling German addresses and
// file names have always used here; every other accent is dropped.
var asciiLetters = map[rune]string{
	'ä': "ae", 'ö': "oe", 'ü': "ue", 'Ä': "Ae", 'Ö': "Oe", 'Ü': "Ue",
	'ß': "ss", 'ẞ': "SS",
	'ł': "l", 'Ł': "L",
	'ø': "o", 'Ø': "O",
	'đ': "d", 'Đ': "D",
	'æ': "ae", 'Æ': "AE",
	'œ': "oe", 'Œ': "OE",
	'ı': "i",
}

// ASCII transliterates Latin letters to ASCII and keeps case. Characters
// without an ASCII spelling (punctuation, other scripts) stay as they are;
// each caller applies its own rules for those.
func ASCII(value string) string {
	value = norm.NFC.String(value)
	var out strings.Builder
	out.Grow(len(value))
	for _, r := range value {
		if r < utf8.RuneSelf {
			out.WriteRune(r)
			continue
		}
		if spelled, ok := asciiLetters[r]; ok {
			out.WriteString(spelled)
			continue
		}
		out.WriteString(stripMarks(r))
	}
	return out.String()
}

// stripMarks returns the ASCII base of an accented letter, or the rune
// itself when its decomposition is not ASCII.
func stripMarks(r rune) string {
	var base strings.Builder
	for _, d := range norm.NFD.String(string(r)) {
		if unicode.Is(unicode.Mn, d) {
			continue
		}
		if d >= utf8.RuneSelf {
			return string(r)
		}
		base.WriteRune(d)
	}
	if base.Len() == 0 {
		return string(r)
	}
	return base.String()
}

// Badge is the short text of a 48 px logo stand-in, such as the care term
// in a mail header (#6042, #6082): a term of up to four characters as it
// is, else at most three initials, or the first three characters of a
// single word. The full term stays in the header text, which wraps.
func Badge(term string) string {
	term = strings.TrimSpace(term)
	if utf8.RuneCountInString(term) <= 4 {
		return term
	}
	words := strings.Fields(term)
	if len(words) == 1 {
		return string([]rune(term)[:3])
	}
	initials := make([]rune, 0, 3)
	for _, word := range words {
		for _, r := range word {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				initials = append(initials, unicode.ToUpper(r))
				break
			}
		}
		if len(initials) == 3 {
			break
		}
	}
	return string(initials)
}
