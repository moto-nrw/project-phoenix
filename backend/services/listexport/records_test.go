package listexport

import (
	"bytes"
	"strings"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
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

// traceRecords runs the measuring pass of the designed record layout and
// returns its page count and emitted-text trace. The old writer's tests
// asserted content by string search in raw PDF bytes; compressed streams +
// subset fonts make that impossible, so the rules assert here instead.
func traceRecords(t *testing.T, doc RecordDocument) (int, []string) {
	t.Helper()
	chrome, err := newPageChrome(false, doc.Title, doc.Subtitle, doc.GeneratedAt, doc.Filters, doc.Footer)
	if err != nil {
		t.Fatalf("newPageChrome: %v", err)
	}
	l := &recordLayout{pageChrome: chrome, doc: doc}
	if err := l.run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	return l.page, l.trace
}

func traceContains(trace []string, want string) bool {
	for _, s := range trace {
		if strings.Contains(s, want) {
			return true
		}
	}
	return false
}

func TestRenderRecords_ProducesValidPDF(t *testing.T) {
	svc := NewService()
	doc := RecordDocument{
		Title:       "Anmeldungen – Test",
		Subtitle:    "1 Anmeldung, 1 Kind",
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
	if !bytes.Contains(file.Data, []byte("/ToUnicode")) {
		t.Error("expected embedded ToUnicode CMap so PDF text stays extractable")
	}

	_, trace := traceRecords(t, doc)
	for _, want := range []string{"E-Mail", "Kind: Lina", "Betreuungsangebote"} {
		if !traceContains(trace, want) {
			t.Errorf("record layout missing %q", want)
		}
	}
}

func TestRenderRecords_PaginatesAcrossPages(t *testing.T) {
	records := make([]Record, 0, 80)
	for i := 0; i < 80; i++ {
		records = append(records, sampleRecord("Familie Nr "+strings.Repeat("X", 3)))
	}
	doc := RecordDocument{Title: "Viele", GeneratedAt: time.Now(), Records: records}

	pages, trace := traceRecords(t, doc)
	if pages < 2 {
		t.Errorf("expected multiple pages for 80 records, got %d", pages)
	}
	// No record dropped: every heading appears in the trace.
	count := 0
	for _, s := range trace {
		if strings.HasPrefix(s, "Familie Nr ") {
			count++
		}
	}
	if count != 80 {
		t.Errorf("headings in layout = %d, want 80", count)
	}
}

func TestRenderRecords_EmptyRecords(t *testing.T) {
	doc := RecordDocument{Title: "Leer", GeneratedAt: time.Now()}
	_, trace := traceRecords(t, doc)
	if !traceContains(trace, "Keine Anmeldungen") {
		t.Errorf("empty document should render the placeholder line")
	}

	if _, err := (&RendererService{}).RenderRecords(doc, "leer"); err != nil {
		t.Fatalf("RenderRecords: %v", err)
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

	_, trace := traceRecords(t, doc)
	for _, want := range []string{"Bestätigte Anmeldungen", "Familie Muster"} {
		if !traceContains(trace, want) {
			t.Errorf("record layout missing %q", want)
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

	xml := testpkg.ReadDocxDocumentXML(t, file.Data)
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
	xml := testpkg.ReadDocxDocumentXML(t, file.Data)
	for _, want := range []string{"Bestätigte Anmeldungen", "Familie Muster"} {
		if !strings.Contains(xml, want) {
			t.Errorf("record DOCX missing %q", want)
		}
	}
}
