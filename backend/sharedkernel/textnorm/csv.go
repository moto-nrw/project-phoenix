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

// CSVDelimiter is the chosen delimiter or, when none is chosen, the one
// of semicolon, comma and tab the header line holds most often outside
// quotes; a tie takes the semicolon German spreadsheets write.
func CSVDelimiter(text, chosen string) (rune, error) {
	switch chosen {
	case ";", ",", "\t":
		return rune(chosen[0]), nil
	case "":
	default:
		return 0, errors.New("invalid CSV delimiter")
	}
	header, _, _ := strings.Cut(text, "\n")
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
