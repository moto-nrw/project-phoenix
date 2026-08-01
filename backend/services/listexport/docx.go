package listexport

import (
	"archive/zip"
	"bytes"
	"strconv"
	"strings"
)

func renderDOCX(doc Document) ([]byte, error) {
	return writeDOCXArchive(documentXML(doc))
}

func renderRecordsDOCX(doc RecordDocument) ([]byte, error) {
	return writeDOCXArchive(recordDocumentXML(doc))
}

func writeDOCXArchive(document string) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	files := map[string]string{
		"[Content_Types].xml": contentTypesXML(),
		"_rels/.rels":         relsXML(),
		"word/document.xml":   document,
	}
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write([]byte(content)); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func contentTypesXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`
}

func relsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`
}

func documentXML(doc Document) string {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`)
	writeParagraph(&b, doc.Title, true)
	if doc.Subtitle != "" {
		writeParagraph(&b, doc.Subtitle, false)
	}
	writeParagraph(&b, "Erstellt: "+GeneratedAtLabel(doc.GeneratedAt), false)
	for _, filter := range doc.Filters {
		writeParagraph(&b, filter, false)
	}
	for segmentIdx, segment := range groupedRowSegments(doc.Rows) {
		if segment.title != "" {
			if segmentIdx > 0 {
				writeBlankParagraph(&b)
			}
			writeGroupHeading(&b, segment.title)
		}
		writeTable(&b, doc.Columns, segment.rows)
	}
	writeParagraph(&b, "moto", false)
	b.WriteString(`<w:sectPr><w:pgSz w:w="16838" w:h="11906" w:orient="landscape"/><w:pgMar w:top="720" w:right="720" w:bottom="720" w:left="720" w:header="450" w:footer="450" w:gutter="0"/></w:sectPr>`)
	b.WriteString(`</w:body></w:document>`)
	return b.String()
}

func recordDocumentXML(doc RecordDocument) string {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`)
	writeParagraph(&b, doc.Title, true)
	if doc.Subtitle != "" {
		writeParagraph(&b, doc.Subtitle, false)
	}
	writeParagraph(&b, "Erstellt: "+GeneratedAtLabel(doc.GeneratedAt), false)
	for _, filter := range doc.Filters {
		writeParagraph(&b, filter, false)
	}
	writeBlankParagraph(&b)
	groups := recordDocumentGroups(doc)
	if recordGroupCount(groups) == 0 {
		writeParagraph(&b, "Keine Anmeldungen vorhanden.", false)
	}
	for groupIdx, group := range groups {
		if len(group.Records) == 0 {
			continue
		}
		if groupIdx > 0 {
			writeBlankParagraph(&b)
		}
		writeGroupHeading(&b, group.Title)
		for i, record := range group.Records {
			writeRecordBlock(&b, record)
			if i < len(group.Records)-1 {
				writeBlankParagraph(&b)
			}
		}
	}
	if doc.Footer != "" {
		writeBlankParagraph(&b)
		writeParagraph(&b, doc.Footer, false)
	}
	writeParagraph(&b, "moto", false)
	b.WriteString(`<w:sectPr><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="720" w:right="720" w:bottom="720" w:left="720" w:header="450" w:footer="450" w:gutter="0"/></w:sectPr>`)
	b.WriteString(`</w:body></w:document>`)
	return b.String()
}

func writeRecordBlock(b *bytes.Buffer, record Record) {
	writeBlockTitle(b, record.Title, 0)
	for _, field := range record.Fields {
		writeFieldParagraph(b, field, 180)
	}
	for _, sub := range record.Subs {
		writeBlockTitle(b, sub.Title, 360)
		for _, field := range sub.Fields {
			writeFieldParagraph(b, field, 540)
		}
	}
}

func writeBlockTitle(b *bytes.Buffer, title string, indent int) {
	b.WriteString(`<w:p><w:pPr>`)
	if indent > 0 {
		b.WriteString(`<w:ind w:left="`)
		b.WriteString(strconv.Itoa(indent))
		b.WriteString(`"/>`)
	}
	b.WriteString(`<w:spacing w:before="180" w:after="80"/>`)
	b.WriteString(`</w:pPr><w:r><w:rPr><w:b/><w:sz w:val="24"/></w:rPr><w:t>`)
	b.WriteString(xmlText(title))
	b.WriteString(`</w:t></w:r></w:p>`)
}

func writeGroupHeading(b *bytes.Buffer, title string) {
	title = strings.TrimSpace(title)
	if title == "" {
		return
	}
	b.WriteString(`<w:p><w:pPr><w:spacing w:before="240" w:after="120"/></w:pPr><w:r><w:rPr><w:b/><w:sz w:val="28"/></w:rPr><w:t>`)
	b.WriteString(xmlText(title))
	b.WriteString(`</w:t></w:r></w:p>`)
}

func writeFieldParagraph(b *bytes.Buffer, field Field, indent int) {
	value := strings.TrimSpace(field.Value)
	if value == "" {
		return
	}
	label := strings.TrimSpace(field.Label)
	b.WriteString(`<w:p><w:pPr><w:ind w:left="`)
	b.WriteString(strconv.Itoa(indent))
	b.WriteString(`"/><w:spacing w:after="40"/></w:pPr>`)
	if label != "" {
		b.WriteString(`<w:r><w:rPr><w:b/></w:rPr><w:t xml:space="preserve">`)
		b.WriteString(xmlText(label + ": "))
		b.WriteString(`</w:t></w:r>`)
	}
	b.WriteString(`<w:r><w:t>`)
	b.WriteString(xmlText(value))
	b.WriteString(`</w:t></w:r></w:p>`)
}

func writeBlankParagraph(b *bytes.Buffer) {
	b.WriteString(`<w:p><w:r><w:t xml:space="preserve"> </w:t></w:r></w:p>`)
}

func writeParagraph(b *bytes.Buffer, text string, bold bool) {
	b.WriteString(`<w:p><w:r>`)
	if bold {
		b.WriteString(`<w:rPr><w:b/></w:rPr>`)
	}
	b.WriteString(`<w:t>`)
	b.WriteString(xmlText(text))
	b.WriteString(`</w:t></w:r></w:p>`)
}

// rowSegment is a run of consecutive rows sharing one group heading; an
// ungrouped document yields a single segment with an empty title.
type rowSegment struct {
	title string
	rows  []Row
}

func groupedRowSegments(rows []Row) []rowSegment {
	segments := []rowSegment{{}}
	for _, row := range rows {
		if row.GroupTitle != "" {
			if len(segments) == 1 && segments[0].title == "" && len(segments[0].rows) == 0 {
				segments[0].title = row.GroupTitle
				continue
			}
			segments = append(segments, rowSegment{title: row.GroupTitle})
			continue
		}
		segments[len(segments)-1].rows = append(segments[len(segments)-1].rows, row)
	}
	return segments
}

func writeTable(b *bytes.Buffer, columns []Column, rows []Row) {
	b.WriteString(`<w:tbl><w:tblPr><w:tblW w:w="0" w:type="auto"/><w:tblBorders><w:top w:val="single" w:sz="4" w:color="D1D5DB"/><w:left w:val="single" w:sz="4" w:color="D1D5DB"/><w:bottom w:val="single" w:sz="4" w:color="D1D5DB"/><w:right w:val="single" w:sz="4" w:color="D1D5DB"/><w:insideH w:val="single" w:sz="4" w:color="D1D5DB"/><w:insideV w:val="single" w:sz="4" w:color="D1D5DB"/></w:tblBorders></w:tblPr>`)
	b.WriteString(`<w:tr>`)
	for _, column := range columns {
		writeCell(b, column.Label, true)
	}
	b.WriteString(`</w:tr>`)
	for _, row := range rows {
		b.WriteString(`<w:tr>`)
		for _, column := range columns {
			writeCell(b, row.Values[column.ID], false)
		}
		b.WriteString(`</w:tr>`)
	}
	b.WriteString(`</w:tbl>`)
}

func writeCell(b *bytes.Buffer, text string, bold bool) {
	b.WriteString(`<w:tc><w:tcPr><w:tcW w:w="2400" w:type="dxa"/></w:tcPr><w:p><w:r>`)
	if bold {
		b.WriteString(`<w:rPr><w:b/></w:rPr>`)
	}
	if text == "" {
		text = " "
	}
	// "\n" is a line break in a cell across all three renderers (the PDF
	// wrapper and the XLSX wrap style honour it too); Word collapses raw
	// whitespace inside <w:t>, so it needs an explicit <w:br/>.
	for i, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if i > 0 {
			b.WriteString(`<w:br/>`)
		}
		_, line = DecodeLine(line)
		b.WriteString(`<w:t xml:space="preserve">`)
		b.WriteString(xmlText(line))
		b.WriteString(`</w:t>`)
	}
	b.WriteString(`</w:r></w:p></w:tc>`)
}

func xmlText(text string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(text)
}
