package listexport

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
	"time"
)

func sampleRecord(title string) Record {
	return Record{
		Title: title,
		Fields: []Field{
			{Label: "E-Mail", Value: "anna@example.test"},
			{Label: "Zustimmungen", Value: "AGB: Ja, Foto: Nein"},
		},
		Subs: []SubRecord{
			{Title: "Kind: Lina", Fields: []Field{
				{Label: "Geburtsdatum", Value: "12.05.2018"},
				{Label: "Betreuungsangebote", Value: "Kernzeit (Mo, Di, Mi)"},
			}},
		},
	}
}

func TestRenderRecords_ProducesValidPDF(t *testing.T) {
	svc := NewService()
	doc := RecordDocument{
		Title:       "Anmeldungen – Test",
		Subtitle:    "1 Anmeldungen, 1 Kinder",
		GeneratedAt: time.Date(2026, 6, 4, 9, 0, 0, 0, time.UTC),
		Footer:      "Vertraulich",
		Records:     []Record{sampleRecord("Familie Muster")},
	}

	file, err := svc.RenderRecords(doc, "Anmeldungen – Test")
	if err != nil {
		t.Fatalf("RenderRecords: %v", err)
	}
	if file.ContentType != "application/pdf" {
		t.Errorf("content type = %q, want application/pdf", file.ContentType)
	}
	if !strings.HasSuffix(file.Filename, ".pdf") {
		t.Errorf("filename = %q, want .pdf suffix", file.Filename)
	}
	if !bytes.HasPrefix(file.Data, []byte("%PDF-1.")) {
		t.Errorf("data does not start with PDF header")
	}
	for _, want := range []string{"E-Mail", "Kind: Lina", "Betreuungsangebote", "%%EOF"} {
		if !bytes.Contains(file.Data, []byte(want)) {
			t.Errorf("rendered PDF missing %q", want)
		}
	}
}

func TestRenderRecords_PaginatesAcrossPages(t *testing.T) {
	records := make([]Record, 0, 80)
	for i := 0; i < 80; i++ {
		records = append(records, sampleRecord("Familie Nr "+strings.Repeat("X", 3)))
	}
	doc := RecordDocument{Title: "Viele", GeneratedAt: time.Now(), Records: records}

	file, err := (&RendererService{}).RenderRecords(doc, "viele")
	if err != nil {
		t.Fatalf("RenderRecords: %v", err)
	}
	pages := bytes.Count(file.Data, []byte("/Type /Page /Parent"))
	if pages < 2 {
		t.Errorf("expected multiple pages for 80 records, got %d", pages)
	}
}

func TestRenderRecords_EmptyRecords(t *testing.T) {
	file, err := (&RendererService{}).RenderRecords(RecordDocument{Title: "Leer", GeneratedAt: time.Now()}, "leer")
	if err != nil {
		t.Fatalf("RenderRecords: %v", err)
	}
	if !bytes.Contains(file.Data, []byte("Keine Anmeldungen")) {
		t.Errorf("empty document should render the placeholder line")
	}
}

func TestRenderRecords_WritesGroupHeadings(t *testing.T) {
	doc := RecordDocument{
		Title:       "Anmeldungen Test",
		GeneratedAt: time.Date(2026, 6, 4, 9, 0, 0, 0, time.UTC),
		Groups: []RecordGroup{
			{Title: "Bestätigte Anmeldungen", Records: []Record{sampleRecord("Familie Muster")}},
		},
	}

	file, err := NewService().RenderRecords(doc, "Anmeldungen Test")
	if err != nil {
		t.Fatalf("RenderRecords: %v", err)
	}
	for _, want := range []string{"Bestätigte Anmeldungen", "Familie Muster"} {
		if !bytes.Contains(file.Data, []byte(pdfLiteralString(want))) {
			t.Errorf("rendered PDF missing %q", want)
		}
	}
}

func TestRenderRecordsDOCX_UsesRecordBlocksInsteadOfWideTable(t *testing.T) {
	doc := RecordDocument{
		Title:       "Anmeldungen Test",
		Subtitle:    "1 Anmeldungen, 1 Kinder",
		GeneratedAt: time.Date(2026, 6, 4, 9, 0, 0, 0, time.UTC),
		Footer:      "Vertraulich",
		Records:     []Record{sampleRecord("Familie Muster")},
	}

	file, err := NewService().RenderRecordsDOCX(doc, "Anmeldungen Test")
	if err != nil {
		t.Fatalf("RenderRecordsDOCX: %v", err)
	}
	if file.ContentType != "application/vnd.openxmlformats-officedocument.wordprocessingml.document" {
		t.Errorf("content type = %q, want DOCX content type", file.ContentType)
	}
	if !strings.HasSuffix(file.Filename, ".docx") {
		t.Errorf("filename = %q, want .docx suffix", file.Filename)
	}

	xml := readDocxDocumentXML(t, file.Data)
	if strings.Contains(xml, "<w:tbl>") {
		t.Fatal("record DOCX must not render the wide table layout")
	}
	for _, want := range []string{"Anmeldungen Test", "Familie Muster", "Kind: Lina", "E-Mail", "Betreuungsangebote", "Vertraulich"} {
		if !strings.Contains(xml, want) {
			t.Errorf("record DOCX missing %q", want)
		}
	}
}

func TestRenderRecordsDOCX_WritesGroupHeadings(t *testing.T) {
	doc := RecordDocument{
		Title:       "Anmeldungen Test",
		GeneratedAt: time.Date(2026, 6, 4, 9, 0, 0, 0, time.UTC),
		Groups: []RecordGroup{
			{Title: "Bestätigte Anmeldungen", Records: []Record{sampleRecord("Familie Muster")}},
		},
	}

	file, err := NewService().RenderRecordsDOCX(doc, "Anmeldungen Test")
	if err != nil {
		t.Fatalf("RenderRecordsDOCX: %v", err)
	}
	xml := readDocxDocumentXML(t, file.Data)
	for _, want := range []string{"Bestätigte Anmeldungen", "Familie Muster"} {
		if !strings.Contains(xml, want) {
			t.Errorf("record DOCX missing %q", want)
		}
	}
}

func readDocxDocumentXML(t *testing.T, data []byte) string {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
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
		return string(content)
	}
	t.Fatal("DOCX missing word/document.xml")
	return ""
}
