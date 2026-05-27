package listexport

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func TestRenderFormats(t *testing.T) {
	doc := sampleDocument()
	service := NewService()

	tests := []struct {
		name        string
		format      Format
		contentType string
		extension   string
	}{
		{name: "pdf", format: FormatPDF, contentType: "application/pdf", extension: ".pdf"},
		{name: "docx", format: FormatDOCX, contentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document", extension: ".docx"},
		{name: "xlsx", format: FormatXLSX, contentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", extension: ".xlsx"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, err := service.Render(doc, tt.format, "OGS Export Liste")
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}
			if file.ContentType != tt.contentType {
				t.Fatalf("content type = %q, want %q", file.ContentType, tt.contentType)
			}
			if !strings.HasSuffix(file.Filename, tt.extension) {
				t.Fatalf("filename = %q, want suffix %q", file.Filename, tt.extension)
			}
			if len(file.Data) == 0 {
				t.Fatal("expected non-empty export data")
			}
		})
	}
}

func TestRenderPDFWritesDocumentHeader(t *testing.T) {
	file, err := NewService().Render(sampleDocument(), FormatPDF, "liste")
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if !bytes.HasPrefix(file.Data, []byte("%PDF-1.4")) {
		t.Fatalf("PDF header = %q", file.Data[:8])
	}
}

func TestRenderPDFWritesReadableWinAnsiText(t *testing.T) {
	file, err := NewService().Render(sampleDocument(), FormatPDF, "liste")
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if bytes.Contains(file.Data, []byte("FEFF")) {
		t.Fatal("PDF should not contain UTF-16 byte order markers for simple font text")
	}
	if !bytes.Contains(file.Data, []byte("(OGS Wochenliste)")) {
		t.Fatal("expected PDF stream to contain readable literal text")
	}
}

func TestRenderPDFWrapsWithoutTruncatingCellText(t *testing.T) {
	doc := Document{
		Title:       "Meine Liste",
		GeneratedAt: time.Date(2026, time.May, 27, 14, 30, 0, 0, time.UTC),
		Columns:     ResolveColumns([]ColumnID{ColumnName, ColumnSchoolClass, ColumnGroup, ColumnCareDays, ColumnWeeklyMonday}, PresetOGSWeekly),
		Rows: []Row{
			{Values: map[ColumnID]string{
				ColumnName:         "Mila Muster",
				ColumnSchoolClass:  "Klasse 1a",
				ColumnGroup:        "Regenbogengruppe",
				ColumnCareDays:     "Mo, Di, Mi, Do, Fr",
				ColumnWeeklyMonday: "08:00 bis 16:00",
			}},
		},
	}

	file, err := NewService().Render(doc, FormatPDF, "liste")
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if bytes.Contains(file.Data, []byte("...")) {
		t.Fatal("PDF should wrap cell text instead of truncating it")
	}
	for _, want := range []string{"Mo, Di, Mi,", "Do, Fr"} {
		if !bytes.Contains(file.Data, []byte(want)) {
			t.Fatalf("expected wrapped PDF stream to contain %q", want)
		}
	}
}

func TestRenderDOCXWritesWordDocument(t *testing.T) {
	file, err := NewService().Render(sampleDocument(), FormatDOCX, "liste")
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(file.Data), int64(len(file.Data)))
	if err != nil {
		t.Fatalf("zip reader error = %v", err)
	}
	for _, entry := range reader.File {
		if entry.Name != "word/document.xml" {
			continue
		}
		rc, err := entry.Open()
		if err != nil {
			t.Fatalf("open document.xml error = %v", err)
		}
		defer func() {
			if err := rc.Close(); err != nil {
				t.Errorf("close document.xml error = %v", err)
			}
		}()
		content, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("read document.xml error = %v", err)
		}
		if !strings.Contains(string(content), "OGS Wochenliste") {
			t.Fatal("expected DOCX document to contain title")
		}
		return
	}
	t.Fatal("word/document.xml not found")
}

func TestRenderXLSXWritesTable(t *testing.T) {
	file, err := NewService().Render(sampleDocument(), FormatXLSX, "liste")
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	workbook, err := excelize.OpenReader(bytes.NewReader(file.Data))
	if err != nil {
		t.Fatalf("open xlsx error = %v", err)
	}
	defer func() {
		if err := workbook.Close(); err != nil {
			t.Errorf("close xlsx error = %v", err)
		}
	}()

	title, err := workbook.GetCellValue("Export", "A1")
	if err != nil {
		t.Fatalf("read title error = %v", err)
	}
	if title != "OGS Wochenliste" {
		t.Fatalf("title = %q, want OGS Wochenliste", title)
	}
	cell, err := workbook.GetCellValue("Export", "A7")
	if err != nil {
		t.Fatalf("read row error = %v", err)
	}
	if cell != "Mila Muster" {
		t.Fatalf("first row name = %q, want Mila Muster", cell)
	}
}

func TestColumnCatalogExcludesRoomAndInternalIdentifier(t *testing.T) {
	catalog := ColumnCatalog()
	blocked := []ColumnID{"room", "homeroom", "identifier", "internal_identifier"}
	for _, columnID := range blocked {
		if _, ok := catalog[columnID]; ok {
			t.Fatalf("column catalog contains blocked column %q", columnID)
		}
	}
}

func sampleDocument() Document {
	columns := ResolveColumns([]ColumnID{ColumnName, ColumnSchoolClass, ColumnWeeklyMonday}, PresetOGSWeekly)
	return Document{
		Title:       "OGS Wochenliste",
		Subtitle:    "2 Kinder",
		GeneratedAt: time.Date(2026, time.May, 27, 14, 30, 0, 0, time.UTC),
		Filters:     []string{"Suche: Klasse 1a"},
		Columns:     columns,
		Rows: []Row{
			{Values: map[ColumnID]string{ColumnName: "Mila Muster", ColumnSchoolClass: "1a", ColumnWeeklyMonday: "08:00 bis 15:00"}},
			{Values: map[ColumnID]string{ColumnName: "Noah Beispiel", ColumnSchoolClass: "1a", ColumnWeeklyMonday: "08:15 bis 16:00"}},
		},
	}
}
