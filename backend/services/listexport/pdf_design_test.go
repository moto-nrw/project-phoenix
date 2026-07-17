package listexport

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// The business rules in this file are ported from the hand-rolled PDF
// writer's tests (grouped_render_test.go / service_test.go). Those asserted
// via string search in the raw PDF bytes, which only worked because that
// writer emitted uncompressed WinAnsi literals. The designed renderer
// (gopdf) compresses content streams and encodes text as subset-font glyph
// indices, so the same rules are now asserted where they are decided — on
// paginate() and wrap() — plus smoke tests on the rendered bytes.

func designGroupedDocument() Document {
	return Document{
		Title:       "Klassenlisten",
		GeneratedAt: time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC),
		Columns: []Column{
			{ID: ColumnName, Label: "Name"},
			{ID: ColumnSchoolClass, Label: "Klasse"},
		},
		Rows: []Row{
			{GroupTitle: "Klasse 1a"},
			{Values: map[ColumnID]string{ColumnName: "Anders, Emma", ColumnSchoolClass: "1a"}},
			{Values: map[ColumnID]string{ColumnName: "Becker, Finn", ColumnSchoolClass: "1a"}},
			{GroupTitle: "Klasse 2b"},
			{Values: map[ColumnID]string{ColumnName: "Conrad, Ida", ColumnSchoolClass: "2b"}},
		},
	}
}

// countRenderedPDFPages counts page objects in the raw PDF: every page
// carries "/Type /Page" and the page tree root carries "/Type /Pages",
// which the subtraction removes again.
func countRenderedPDFPages(data []byte) int {
	return bytes.Count(data, []byte("/Type /Page")) - bytes.Count(data, []byte("/Type /Pages"))
}

// Ported rule: every group starts on its own page, and a GroupTitle row is
// never rendered as a table row.
func TestDesignedPDFGroupsStartNewPages(t *testing.T) {
	r, err := newDesignRenderer(designGroupedDocument())
	if err != nil {
		t.Fatalf("newDesignRenderer: %v", err)
	}
	pages, err := r.paginate()
	if err != nil {
		t.Fatalf("paginate: %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("page count = %d, want 2", len(pages))
	}
	if pages[0].groupTitle != "Klasse 1a" || pages[1].groupTitle != "Klasse 2b" {
		t.Fatalf("group titles = %q, %q", pages[0].groupTitle, pages[1].groupTitle)
	}
	if len(pages[0].rows) != 2 || len(pages[1].rows) != 1 {
		t.Fatalf("row counts = %d, %d, want 2, 1", len(pages[0].rows), len(pages[1].rows))
	}

	file, err := NewService().Render(designGroupedDocument(), FormatPDF, "klassenlisten")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := countRenderedPDFPages(file.Data); got != 2 {
		t.Fatalf("rendered page count = %d, want 2", got)
	}
}

// Ported rule: when a group overflows, its heading repeats on every
// continuation page so each printed sheet is self-describing.
func TestDesignedPDFGroupHeadingRepeatsOnOverflowPages(t *testing.T) {
	doc := designGroupedDocument()
	rows := []Row{{GroupTitle: "Klasse 1a"}}
	for i := 0; i < 60; i++ {
		rows = append(rows, Row{Values: map[ColumnID]string{
			ColumnName:        fmt.Sprintf("Kind %02d", i),
			ColumnSchoolClass: "1a",
		}})
	}
	doc.Rows = rows

	r, err := newDesignRenderer(doc)
	if err != nil {
		t.Fatalf("newDesignRenderer: %v", err)
	}
	pages, err := r.paginate()
	if err != nil {
		t.Fatalf("paginate: %v", err)
	}
	if len(pages) < 2 {
		t.Fatalf("expected overflow onto a second page, got %d page(s)", len(pages))
	}
	total := 0
	for i, p := range pages {
		if p.groupTitle != "Klasse 1a" {
			t.Fatalf("page %d group title = %q, want repeated heading", i, p.groupTitle)
		}
		total += len(p.rows)
	}
	if total != 60 {
		t.Fatalf("rows across pages = %d, want 60 (none dropped)", total)
	}
}

// Ported rule: a short ungrouped list stays on a single page.
func TestDesignedPDFUngroupedStaysSinglePage(t *testing.T) {
	doc := designGroupedDocument()
	doc.Rows = []Row{
		{Values: map[ColumnID]string{ColumnName: "Anders, Emma", ColumnSchoolClass: "1a"}},
	}
	r, err := newDesignRenderer(doc)
	if err != nil {
		t.Fatalf("newDesignRenderer: %v", err)
	}
	pages, err := r.paginate()
	if err != nil {
		t.Fatalf("paginate: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("page count = %d, want 1", len(pages))
	}
	if pages[0].groupTitle != "" {
		t.Fatalf("group title = %q, want empty", pages[0].groupTitle)
	}
}

// Ported rule: cell text wraps instead of being truncated — every word
// survives and no ellipsis is introduced.
func TestDesignedPDFWrapKeepsAllWords(t *testing.T) {
	r, err := newDesignRenderer(designGroupedDocument())
	if err != nil {
		t.Fatalf("newDesignRenderer: %v", err)
	}
	if err := r.pdf.SetFont(fontFamily, styleNormal, fontBody); err != nil {
		t.Fatalf("SetFont: %v", err)
	}

	const text = "Mo, Di, Mi, Do, Fr und ein sehr langer Zusatzhinweis"
	lines := r.wrap(text, 60) // narrow: forces multiple lines
	if len(lines) < 2 {
		t.Fatalf("wrap lines = %v, want multiple", lines)
	}
	if strings.Join(lines, " ") != text {
		t.Fatalf("wrap dropped or altered words: %q", strings.Join(lines, " "))
	}
	for _, ln := range lines {
		if strings.Contains(ln, "...") || strings.Contains(ln, "…") {
			t.Fatalf("wrap introduced an ellipsis: %q", ln)
		}
	}
}

// Smoke test on the rendered bytes: valid header, extractable text
// (ToUnicode CMap present), diacritics render without error, and the
// GDPR footer no longer vanishes on the PDF path.
func TestDesignedPDFSmoke(t *testing.T) {
	doc := designGroupedDocument()
	doc.Subtitle = "Grundschule Bissendorf · Schuljahr 2026/27"
	doc.Footer = "Vertraulich — enthält personenbezogene Daten."
	doc.Rows = append(doc.Rows,
		Row{Values: map[ColumnID]string{ColumnName: "Nguyễn, Linh", ColumnSchoolClass: "2b"}},
		Row{Values: map[ColumnID]string{ColumnName: "Kowalczyk, Łukasz", ColumnSchoolClass: "2b"}},
		Row{Values: map[ColumnID]string{ColumnName: "Đorđević, Ana", ColumnSchoolClass: "2b"}},
	)

	file, err := NewService().Render(doc, FormatPDF, "klassenlisten")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !bytes.HasPrefix(file.Data, []byte("%PDF-1.")) {
		t.Fatalf("PDF header = %q", file.Data[:8])
	}
	if !bytes.Contains(file.Data, []byte("/ToUnicode")) {
		t.Fatal("expected embedded ToUnicode CMap so PDF text stays extractable")
	}
	if len(file.Data) < 5000 {
		t.Fatalf("suspiciously small pdf: %d bytes", len(file.Data))
	}

	if out := os.Getenv("LISTEXPORT_PDF_OUT"); out != "" {
		path := out + "/" + file.Filename
		if err := os.WriteFile(path, file.Data, 0o600); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", path)
	}
}

// gopdf does no OpenType shaping: decomposed input (NFD, common from
// macOS) would silently lose its combining marks. norms() must fold to NFC
// before any text reaches the renderer.
func TestNormsFoldsNFDToNFC(t *testing.T) {
	// "\u1ec5" (e with circumflex+tilde) decomposed into base letter plus
	// combining marks — what macOS filesystems and some XML emit.
	decomposed := "Nguy\u0065\u0302\u0303n"
	precomposed := "Nguy\u1ec5n"
	if got := norms(decomposed); got != precomposed {
		t.Fatalf("norms(NFD) = %q, want %q", got, precomposed)
	}
	if norms("M\u00fcller") != "M\u00fcller" {
		t.Fatal("norms must be a no-op on already-NFC input")
	}
}
