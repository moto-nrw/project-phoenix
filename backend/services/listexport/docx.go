package listexport

import (
	"archive/zip"
	"bytes"
	"strings"
)

func renderDOCX(doc Document) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	files := map[string]string{
		"[Content_Types].xml": contentTypesXML(),
		"_rels/.rels":         relsXML(),
		"word/document.xml":   documentXML(doc),
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
	writeTable(&b, doc)
	writeParagraph(&b, "moto", false)
	b.WriteString(`<w:sectPr><w:pgSz w:w="16838" w:h="11906" w:orient="landscape"/><w:pgMar w:top="720" w:right="720" w:bottom="720" w:left="720" w:header="450" w:footer="450" w:gutter="0"/></w:sectPr>`)
	b.WriteString(`</w:body></w:document>`)
	return b.String()
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

func writeTable(b *bytes.Buffer, doc Document) {
	b.WriteString(`<w:tbl><w:tblPr><w:tblW w:w="0" w:type="auto"/><w:tblBorders><w:top w:val="single" w:sz="4" w:color="D1D5DB"/><w:left w:val="single" w:sz="4" w:color="D1D5DB"/><w:bottom w:val="single" w:sz="4" w:color="D1D5DB"/><w:right w:val="single" w:sz="4" w:color="D1D5DB"/><w:insideH w:val="single" w:sz="4" w:color="D1D5DB"/><w:insideV w:val="single" w:sz="4" w:color="D1D5DB"/></w:tblBorders></w:tblPr>`)
	b.WriteString(`<w:tr>`)
	for _, column := range doc.Columns {
		writeCell(b, column.Label, true)
	}
	b.WriteString(`</w:tr>`)
	for _, row := range doc.Rows {
		b.WriteString(`<w:tr>`)
		for _, column := range doc.Columns {
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
	b.WriteString(`<w:t>`)
	if text == "" {
		text = " "
	}
	b.WriteString(xmlText(text))
	b.WriteString(`</w:t></w:r></w:p></w:tc>`)
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
