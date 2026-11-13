package textnorm

import (
	"bytes"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
)

// DecodeCSVText answers the file as text: UTF-8 when it is valid UTF-8,
// else Windows-1252. Control characters other than tab and line breaks
// mark a file that is no text, whatever its name says.
func DecodeCSVText(content []byte) (string, string, error) {
	content = bytes.TrimPrefix(content, []byte("\xEF\xBB\xBF"))
	encoding := "utf-8"
	if !utf8.Valid(content) {
		decoded, err := charmap.Windows1252.NewDecoder().Bytes(content)
		if err != nil {
			return "", "", errors.New("invalid CSV text")
		}
		content, encoding = decoded, "windows-1252"
	}
	text := string(content)
	for _, r := range text {
		if r == unicode.ReplacementChar || (unicode.IsControl(r) && r != '\t' && r != '\n' && r != '\r') {
			return "", "", errors.New("invalid CSV text")
		}
	}
	if strings.TrimSpace(text) == "" {
		return "", "", errors.New("invalid CSV text")
	}
	return text, encoding, nil
}

// QuoteNULDelimiters reads a NUL character in the role of the quote
// character as a quote (#7009): some programs, OGS Connect among them,
// write \x00Name\x00 where a CSV has "Name". A NUL counts as a quote only
// at the start of a field (start of the file or line, after a delimiter)
// and at its end (before a delimiter, line break or the end of the file);
// a quote inside such a field is doubled. Any other NUL stays, so
// DecodeCSVText still refuses the file as no text. A file without NUL is
// answered unchanged.
func QuoteNULDelimiters(content []byte) []byte {
	if !bytes.Contains(content, []byte{0}) {
		return content
	}
	content = bytes.TrimPrefix(content, []byte("\xEF\xBB\xBF"))
	boundary := func(b byte) bool { return b == ';' || b == ',' || b == '\t' || b == '\n' || b == '\r' }
	out := make([]byte, 0, len(content)+16)
	quoted := false
	for i, b := range content {
		switch {
		case b == 0 && !quoted && (i == 0 || boundary(content[i-1])):
			quoted = true
			out = append(out, '"')
		case b == 0 && quoted && (i == len(content)-1 || boundary(content[i+1])):
			quoted = false
			out = append(out, '"')
		case b == '"' && quoted:
			out = append(out, '"', '"')
		default:
			out = append(out, b)
		}
	}
	if quoted {
		// An unclosed field: the NUL was no quote, the file stays refused.
		return content
	}
	return out
}

// CSVDelimiter is the chosen delimiter or, when none is chosen, the one
// of semicolon, comma and tab the first nonempty line holds most often outside
// quotes; a tie takes the semicolon German spreadsheets write.
func CSVDelimiter(text, chosen string) (rune, error) {
	switch chosen {
	case ";", ",", "\t":
		return rune(chosen[0]), nil
	case "":
	default:
		return 0, errors.New("invalid CSV delimiter")
	}
	header, rest, _ := strings.Cut(text, "\n")
	for (header == "" || header == "\r") && rest != "" {
		header, rest, _ = strings.Cut(rest, "\n")
	}
	counts := map[rune]int{}
	quoted := false
	for _, r := range header {
		switch {
		case r == '"':
			quoted = !quoted
		case !quoted && (r == ';' || r == ',' || r == '\t'):
			counts[r]++
		}
	}
	best := ';'
	for _, candidate := range []rune{',', '\t'} {
		if counts[candidate] > counts[best] {
			best = candidate
		}
	}
	return best, nil
}
