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
	return decodeCSVText(content, false)
}

// DecodeCSVTextWithNULQuotes is DecodeCSVText for a file that may write a
// NUL where a CSV has a quote, as OGS Connect does (#7009): it keeps the NUL
// in the text, and the reader of each cell takes it off with UnquoteNULCell.
func DecodeCSVTextWithNULQuotes(content []byte) (string, string, error) {
	return decodeCSVText(content, true)
}

func decodeCSVText(content []byte, nulQuotes bool) (string, string, error) {
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
		if nulQuotes && r == 0 {
			continue
		}
		if r == unicode.ReplacementChar || (unicode.IsControl(r) && r != '\t' && r != '\n' && r != '\r') {
			return "", "", errors.New("invalid CSV text")
		}
	}
	if strings.TrimSpace(strings.ReplaceAll(text, "\x00", "")) == "" {
		return "", "", errors.New("invalid CSV text")
	}
	return text, encoding, nil
}

// UnquoteNULCell takes the NUL quotes off a cell a csv.Reader read from a
// DecodeCSVTextWithNULQuotes text (#7009): a cell without NUL stays, a cell
// that starts and ends with a NUL and has none in between loses both. ok is
// false for any other NUL, also one a delimiter or a line break inside the
// quotes left at one end only: the file is then no text the import can read.
func UnquoteNULCell(cell string) (string, bool) {
	if !strings.Contains(cell, "\x00") {
		return cell, true
	}
	if len(cell) >= 2 && cell[0] == 0 && cell[len(cell)-1] == 0 && !strings.Contains(cell[1:len(cell)-1], "\x00") {
		return cell[1 : len(cell)-1], true
	}
	return "", false
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
