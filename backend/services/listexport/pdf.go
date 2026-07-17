package listexport

import (
	"bytes"
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

type pdfObject struct {
	id   int
	data string
}

func pdfColumnWidths(cols []Column, total float64) []float64 {
	weights := make([]float64, len(cols))
	sum := 0.0
	for i, col := range cols {
		weight := 1.0
		switch col.ID {
		case ColumnName:
			weight = 1.4
		case ColumnSchoolClass:
			weight = 0.65
		case ColumnGroup:
			weight = 1.2
		case ColumnWeeklyMonday, ColumnWeeklyTuesday, ColumnWeeklyWednesday, ColumnWeeklyThursday, ColumnWeeklyFriday:
			weight = 0.9
		case ColumnDeparture:
			weight = 1.5
		case ColumnGuardianContacts:
			weight = 2.7
		}
		weights[i] = weight
		sum += weight
	}
	widths := make([]float64, len(cols))
	for i, weight := range weights {
		widths[i] = total * weight / sum
	}
	return widths
}

func writePDFText(b *strings.Builder, x, y, size float64, text string) {
	fmt.Fprintf(b, "BT /F1 %.1f Tf %.1f %.1f Td %s Tj ET\n", size, x, y, pdfLiteralString(text))
}

func pdfLiteralString(text string) string {
	var b strings.Builder
	b.WriteByte('(')
	for _, r := range text {
		writePDFEncodedRune(&b, r)
	}
	b.WriteByte(')')
	return b.String()
}

func writePDFEncodedRune(b *strings.Builder, r rune) {
	switch r {
	case '\\', '(', ')':
		b.WriteByte('\\')
		b.WriteRune(r)
	case '\n':
		b.WriteString(`\n`)
	case '\r':
		b.WriteString(`\r`)
	case '\t':
		b.WriteString(`\t`)
	case 'Ä':
		b.WriteByte(0xC4)
	case 'Ö':
		b.WriteByte(0xD6)
	case 'Ü':
		b.WriteByte(0xDC)
	case 'ä':
		b.WriteByte(0xE4)
	case 'ö':
		b.WriteByte(0xF6)
	case 'ü':
		b.WriteByte(0xFC)
	case 'ß':
		b.WriteByte(0xDF)
	case '–': // en dash (U+2013) → WinAnsi 0x96
		b.WriteByte(0x96)
	case '—': // em dash (U+2014) → WinAnsi 0x97
		b.WriteByte(0x97)
	case 'é':
		b.WriteByte(0xE9)
	case 'è':
		b.WriteByte(0xE8)
	case 'á':
		b.WriteByte(0xE1)
	case 'à':
		b.WriteByte(0xE0)
	case 'ó':
		b.WriteByte(0xF3)
	case 'ò':
		b.WriteByte(0xF2)
	case 'í':
		b.WriteByte(0xED)
	case 'ì':
		b.WriteByte(0xEC)
	default:
		if r >= 32 && r <= 126 {
			b.WriteRune(r)
			return
		}
		// The base-14 Helvetica font is WinAnsi-encoded, so it cannot
		// render arbitrary Unicode. Rather than drop a non-German name to
		// '?', transliterate it to its nearest ASCII form (ş→s, ğ→g, ą→a,
		// ł→l, ñ→n, …) so the print fallback stays legible for the diverse
		// families an OGS enrolls. Only genuinely unrepresentable runes
		// (CJK, Cyrillic, emoji) fall through to '?'.
		if repl, ok := transliterateRune(r); ok {
			for _, rr := range repl {
				writePDFEncodedRune(b, rr)
			}
			return
		}
		b.WriteByte('?')
	}
}

// nonDecomposingTranslit covers Latin letters that NFD does NOT split
// into base+accent (so accent-stripping alone can't reach ASCII).
var nonDecomposingTranslit = map[rune]string{
	'ł': "l", 'Ł': "L",
	'đ': "d", 'Đ': "D",
	'ø': "o", 'Ø': "O",
	'æ': "ae", 'Æ': "Ae",
	'œ': "oe", 'Œ': "Oe",
	'þ': "th", 'Þ': "Th",
	'ð': "d", 'Ð': "D",
	'ı': "i", 'İ': "I", // Turkish dotless/dotted i (do not NFD-decompose to ASCII)
}

// transliterateRune maps a non-WinAnsi rune to its nearest ASCII form,
// reporting ok=false when no reasonable mapping exists (caller emits
// '?'). Strategy: explicit table first, then NFD decomposition with
// combining marks stripped (ş→s, ç→c, ą→a, …). The returned string is
// always ASCII, so the caller's recursion terminates in one step.
func transliterateRune(r rune) (string, bool) {
	if s, ok := nonDecomposingTranslit[r]; ok {
		return s, true
	}
	var b strings.Builder
	for _, c := range norm.NFD.String(string(r)) {
		if unicode.Is(unicode.Mn, c) { // drop combining accents
			continue
		}
		b.WriteRune(c)
	}
	out := b.String()
	if out == "" || out == string(r) {
		return "", false
	}
	return out, true
}

func wrapPDFText(text string, width float64) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return []string{""}
	}
	maxChars := int((width - 8) / (7 * 0.55))
	if maxChars < 1 {
		maxChars = 1
	}
	words := strings.Fields(text)
	lines := make([]string, 0, 1)
	current := ""
	for _, word := range words {
		for len([]rune(word)) > maxChars {
			if current != "" {
				lines = append(lines, current)
				current = ""
			}
			runes := []rune(word)
			lines = append(lines, string(runes[:maxChars]))
			word = string(runes[maxChars:])
		}
		if current == "" {
			current = word
			continue
		}
		next := current + " " + word
		if len([]rune(next)) <= maxChars {
			current = next
			continue
		}
		lines = append(lines, current)
		current = word
	}
	if current != "" {
		lines = append(lines, current)
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

func buildPDF(objects []pdfObject) []byte {
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects)+1)
	for _, obj := range objects {
		offsets[obj.id] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", obj.id, obj.data)
	}
	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n", len(offsets))
	buf.WriteString("0000000000 65535 f \n")
	for id := 1; id < len(offsets); id++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offsets[id])
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	return buf.Bytes()
}
