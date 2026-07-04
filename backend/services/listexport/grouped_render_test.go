package listexport

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

func groupedSampleDocument() Document {
	cols := []Column{
		{ID: ColumnName, Label: "Name"},
		{ID: ColumnSchoolClass, Label: "Klasse"},
	}
	rows := []Row{
		{GroupTitle: "Klasse 1a"},
		{Values: map[ColumnID]string{ColumnName: "Anders, Emma", ColumnSchoolClass: "1a"}},
		{Values: map[ColumnID]string{ColumnName: "Becker, Finn", ColumnSchoolClass: "1a"}},
		{GroupTitle: "Klasse 2b"},
		{Values: map[ColumnID]string{ColumnName: "Conrad, Ida", ColumnSchoolClass: "2b"}},
	}
	return Document{
		Title:       "Klassenlisten",
		GeneratedAt: time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC),
		Columns:     cols,
		Rows:        rows,
	}
}

func TestClassGroupTitle(t *testing.T) {
	cases := map[string]string{
		"1a":         "Klasse 1a",
		" 2b ":       "Klasse 2b",
		"Klasse 1a":  "Klasse 1a",
		"klasse 10a": "klasse 10a",
		"":           "Ohne Klasse",
	}
	for in, want := range cases {
		if got := ClassGroupTitle(in); got != want {
			t.Errorf("ClassGroupTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

func pdfPageCount(data []byte) int {
	return bytes.Count(data, []byte("/Type /Page "))
}

func TestRenderPDFGroupsStartNewPages(t *testing.T) {
	file, err := NewService().Render(groupedSampleDocument(), FormatPDF, "klassenlisten")
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if got := pdfPageCount(file.Data); got != 2 {
		t.Fatalf("page count = %d, want 2", got)
	}
	if !bytes.Contains(file.Data, []byte("(Klasse 1a)")) || !bytes.Contains(file.Data, []byte("(Klasse 2b)")) {
		t.Fatal("expected both group headings in PDF content")
	}
	// A GroupTitle row must not be rendered as a table row: pages 1 and 2
	// hold exactly header + body rows, so each heading appears once.
	if got := bytes.Count(file.Data, []byte("(Klasse 1a)")); got != 1 {
		t.Fatalf("heading 'Klasse 1a' rendered %d times, want 1", got)
	}
}

func TestRenderPDFGroupHeadingRepeatsOnOverflowPages(t *testing.T) {
	doc := groupedSampleDocument()
	rows := []Row{{GroupTitle: "Klasse 1a"}}
	for i := 0; i < 60; i++ {
		rows = append(rows, Row{Values: map[ColumnID]string{ColumnName: fmt.Sprintf("Kind %02d", i), ColumnSchoolClass: "1a"}})
	}
	doc.Rows = rows
	file, err := NewService().Render(doc, FormatPDF, "klassenlisten")
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	pages := pdfPageCount(file.Data)
	if pages < 2 {
		t.Fatalf("expected overflow onto a second page, got %d page(s)", pages)
	}
	if got := bytes.Count(file.Data, []byte("(Klasse 1a)")); got != pages {
		t.Fatalf("heading appears %d times, want once per page (%d)", got, pages)
	}
}

func TestRenderPDFUngroupedStaysSinglePage(t *testing.T) {
	doc := groupedSampleDocument()
	doc.Rows = []Row{
		{Values: map[ColumnID]string{ColumnName: "Anders, Emma", ColumnSchoolClass: "1a"}},
	}
	file, err := NewService().Render(doc, FormatPDF, "klassenlisten")
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if got := pdfPageCount(file.Data); got != 1 {
		t.Fatalf("page count = %d, want 1", got)
	}
}

func docxDocumentXML(t *testing.T, data []byte) string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip.NewReader() error = %v", err)
	}
	for _, f := range zr.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open document.xml: %v", err)
		}
		content, err := io.ReadAll(rc)
		closeErr := rc.Close()
		if err != nil {
			t.Fatalf("read document.xml: %v", err)
		}
		if closeErr != nil {
			t.Fatalf("close document.xml: %v", closeErr)
		}
		return string(content)
	}
	t.Fatal("word/document.xml not found in DOCX archive")
	return ""
}

func TestRenderDOCXGroupsIntoSeparateTables(t *testing.T) {
	file, err := NewService().Render(groupedSampleDocument(), FormatDOCX, "klassenlisten")
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	xml := docxDocumentXML(t, file.Data)
	if got := strings.Count(xml, "<w:tbl>"); got != 2 {
		t.Fatalf("table count = %d, want 2", got)
	}
	if !strings.Contains(xml, "Klasse 1a") || !strings.Contains(xml, "Klasse 2b") {
		t.Fatal("expected both group headings in DOCX content")
	}
}

func TestRenderDOCXUngroupedKeepsSingleTable(t *testing.T) {
	doc := groupedSampleDocument()
	doc.Rows = []Row{
		{Values: map[ColumnID]string{ColumnName: "Anders, Emma", ColumnSchoolClass: "1a"}},
	}
	file, err := NewService().Render(doc, FormatDOCX, "klassenlisten")
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	xml := docxDocumentXML(t, file.Data)
	if got := strings.Count(xml, "<w:tbl>"); got != 1 {
		t.Fatalf("table count = %d, want 1", got)
	}
}
