package listexport

import (
	"bytes"
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
