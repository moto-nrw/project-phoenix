package listexport

import (
	"bytes"
	"fmt"
	"strings"
)

type pdfObject struct {
	id   int
	data string
}

func renderPDF(doc Document) ([]byte, error) {
	pageWidth := 842.0
	pageHeight := 595.0
	margin := 32.0

	cols := doc.Columns
	if len(cols) == 0 {
		cols = ResolveColumns(nil, PresetOGSWeekly)
	}
	usableWidth := pageWidth - 2*margin
	widths := pdfColumnWidths(cols, usableWidth)
	pages := pdfPages(doc, cols, widths, pageWidth, pageHeight, margin)

	objects := []pdfObject{
		{id: 1, data: "<< /Type /Catalog /Pages 2 0 R >>"},
		{id: 3, data: "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>"},
	}
	pageRefs := make([]string, 0, len(pages))
	nextID := 4
	for _, content := range pages {
		pageID := nextID
		contentID := nextID + 1
		nextID += 2
		pageRefs = append(pageRefs, fmt.Sprintf("%d 0 R", pageID))
		pageObj := fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.0f %.0f] /Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>", pageWidth, pageHeight, contentID)
		contentObj := fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content)
		objects = append(objects, pdfObject{id: pageID, data: pageObj}, pdfObject{id: contentID, data: contentObj})
	}
	objects = append(objects[:1], append([]pdfObject{{id: 2, data: fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(pageRefs, " "), len(pageRefs))}}, objects[1:]...)...)
	return buildPDF(objects), nil
}

func pdfPages(doc Document, cols []Column, widths []float64, pageWidth, pageHeight, margin float64) []string {
	pages := make([]string, 0, 1)
	page, y := newPDFPage(doc, cols, widths, pageWidth, pageHeight, margin)
	for _, row := range doc.Rows {
		lines := pdfRowLines(row, cols, widths)
		rowHeight := pdfRowHeight(lines)
		if y-rowHeight < margin+12 {
			pages = append(pages, page.String())
			page, y = newPDFPage(doc, cols, widths, pageWidth, pageHeight, margin)
		}
		writePDFRow(page, margin, y, widths, lines, rowHeight)
		y -= rowHeight
	}
	pages = append(pages, page.String())
	return pages
}

func newPDFPage(doc Document, cols []Column, widths []float64, pageWidth, pageHeight, margin float64) (*strings.Builder, float64) {
	b := &strings.Builder{}
	y := pageHeight - margin
	writePDFText(b, margin, y, 16, doc.Title)
	y -= 20
	if doc.Subtitle != "" {
		writePDFText(b, margin, y, 9, doc.Subtitle)
		y -= 13
	}
	writePDFText(b, margin, y, 8, "Erstellt: "+GeneratedAtLabel(doc.GeneratedAt))
	y -= 12
	for _, filter := range doc.Filters {
		writePDFText(b, margin, y, 8, filter)
		y -= 10
	}
	y -= 6

	headerLines := pdfHeaderLines(cols, widths)
	headerHeight := pdfRowHeight(headerLines)
	writePDFRow(b, margin, y, widths, headerLines, headerHeight)
	y -= headerHeight

	writePDFText(b, pageWidth-margin-28, margin-8, 7, "moto")
	return b, y
}

func pdfColumnWidths(cols []Column, total float64) []float64 {
	weights := make([]float64, len(cols))
	sum := 0.0
	for i, col := range cols {
		weight := 1.0
		switch col.ID {
		case ColumnName:
			weight = 1.8
		case ColumnSchoolClass:
			weight = 0.8
		case ColumnGroup:
			weight = 1.2
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

func pdfHeaderLines(cols []Column, widths []float64) [][]string {
	lines := make([][]string, len(cols))
	for idx, column := range cols {
		lines[idx] = wrapPDFText(column.Label, widths[idx])
	}
	return lines
}

func pdfRowLines(row Row, cols []Column, widths []float64) [][]string {
	lines := make([][]string, len(cols))
	for idx, column := range cols {
		lines[idx] = wrapPDFText(row.Values[column.ID], widths[idx])
	}
	return lines
}

func pdfRowHeight(lines [][]string) float64 {
	maxLines := 1
	for _, cellLines := range lines {
		if len(cellLines) > maxLines {
			maxLines = len(cellLines)
		}
	}
	height := float64(maxLines)*9.5 + 9
	if height < 18 {
		return 18
	}
	return height
}

func writePDFRow(b *strings.Builder, startX, topY float64, widths []float64, lines [][]string, rowHeight float64) {
	x := startX
	for idx, cellLines := range lines {
		drawPDFRect(b, x, topY-rowHeight, widths[idx], rowHeight)
		textY := topY - 9
		for _, line := range cellLines {
			writePDFText(b, x+4, textY, 7, line)
			textY -= 9.5
		}
		x += widths[idx]
	}
}

func writePDFText(b *strings.Builder, x, y, size float64, text string) {
	fmt.Fprintf(b, "BT /F1 %.1f Tf %.1f %.1f Td %s Tj ET\n", size, x, y, pdfLiteralString(text))
}

func drawPDFRect(b *strings.Builder, x, y, w, h float64) {
	fmt.Fprintf(b, "%.1f %.1f %.1f %.1f re S\n", x, y, w, h)
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
		b.WriteByte('?')
	}
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
