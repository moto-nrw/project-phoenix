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
// write \x00Name\x00 where a CSV has "Name". With the file's delimiter it
// follows the fields: a NUL opens a field only as its first character and
// closes it only before the delimiter, a line break or the end of the file;
// a quote inside such a field is doubled. A NUL anywhere else, also inside
// an ordinary quoted field, stays, so DecodeCSVText still refuses the file
// as no text. A file without NUL, or with a NUL field that never closes, is
// answered unchanged.
func QuoteNULDelimiters(content []byte, comma rune) []byte {
	if !bytes.Contains(content, []byte{0}) || comma <= 0 || comma >= utf8.RuneSelf {
		return content
	}
	content = bytes.TrimPrefix(content, []byte("\xEF\xBB\xBF"))
	delimiter := byte(comma)
	ends := func(i int) bool {
		return i == len(content) || content[i] == delimiter || content[i] == '\n' || content[i] == '\r'
	}
	const (
		plain = iota
		nulQuoted
		quoted
	)
	out := make([]byte, 0, len(content)+16)
	state, fieldStart := plain, true
	for i := 0; i < len(content); i++ {
		b := content[i]
		switch state {
		case nulQuoted:
			switch {
			case b == 0 && ends(i+1):
				state = plain
				out = append(out, '"')
			case b == '"':
				out = append(out, '"', '"')
			default:
				out = append(out, b)
			}
			continue
		case quoted:
			out = append(out, b)
			if b == '"' {
				if i+1 < len(content) && content[i+1] == '"' {
					out = append(out, '"')
					i++
				} else {
					state = plain
				}
			}
			continue
		}
		switch {
		case fieldStart && b == 0:
			state = nulQuoted
			out = append(out, '"')
		case fieldStart && b == '"':
			state = quoted
			out = append(out, b)
		default:
			out = append(out, b)
		}
		fieldStart = b == delimiter || b == '\n' || b == '\r'
	}
	if state == nulQuoted {
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
