package listexport

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	listrecords "github.com/moto-nrw/project-phoenix/modules/documentrendering/records"
)

func groupedSampleDocument() listrecords.Document {
	cols := []listrecords.Column{
		{ID: listrecords.ColumnName, Label: "Name"},
		{ID: listrecords.ColumnSchoolClass, Label: "Klasse"},
	}
	rows := []listrecords.Row{
		{GroupTitle: "Klasse 1a"},
		{Values: map[listrecords.ColumnID]string{listrecords.ColumnName: "Anders, Emma", listrecords.ColumnSchoolClass: "1a"}},
		{Values: map[listrecords.ColumnID]string{listrecords.ColumnName: "Becker, Finn", listrecords.ColumnSchoolClass: "1a"}},
		{GroupTitle: "Klasse 2b"},
		{Values: map[listrecords.ColumnID]string{listrecords.ColumnName: "Conrad, Ida", listrecords.ColumnSchoolClass: "2b"}},
	}
	return listrecords.Document{
		Title:       "Klassenlisten",
		GeneratedAt: time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC),
		Columns:     cols,
		Rows:        rows,
	}
}

func TestClassGroupTitle(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"1a":         "Klasse 1a",
		" 2b ":       "Klasse 2b",
		"Klasse 1a":  "Klasse 1a",
		"klasse 10a": "klasse 10a",
		"":           "Ohne Klasse",
	}
	for in, want := range cases {
		if got := listrecords.ClassGroupTitle(in); got != want {
			t.Errorf("listrecords.ClassGroupTitle(%q) = %q, want %q", in, got, want)
		}
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
	t.Parallel()

	file, err := NewService().Render(groupedSampleDocument(), listrecords.FormatDOCX, "klassenlisten")
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
	t.Parallel()

	doc := groupedSampleDocument()
	doc.Rows = []listrecords.Row{
		{Values: map[listrecords.ColumnID]string{listrecords.ColumnName: "Anders, Emma", listrecords.ColumnSchoolClass: "1a"}},
	}
	file, err := NewService().Render(doc, listrecords.FormatDOCX, "klassenlisten")
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	xml := docxDocumentXML(t, file.Data)
	if got := strings.Count(xml, "<w:tbl>"); got != 1 {
		t.Fatalf("table count = %d, want 1", got)
	}
}
