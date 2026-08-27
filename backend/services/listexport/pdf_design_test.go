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
	t.Parallel()

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

func TestSplitRenderRowCarriesAccentIntoContinuation(t *testing.T) {
	t.Parallel()

	row := renderRow{cells: [][]styledLine{{
		{text: "07:30–14:00", accent: "#83CD2D"},
		{text: "Aufgabe 1"},
		{text: "Aufgabe 2"},
	}}, lines: 3}

	parts := splitRenderRow(row, 2)
	if len(parts) != 2 {
		t.Fatalf("parts = %d, want 2", len(parts))
	}
	if got := parts[1].cells[0][0].accent; got != "#83CD2D" {
		t.Fatalf("continuation accent = %q, want active group accent", got)
	}
}

// Ported rule: when a group overflows, its heading repeats on every
// continuation page so each printed sheet is self-describing.
func TestDesignedPDFGroupHeadingRepeatsOnOverflowPages(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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

// A group marker with no body rows — consecutive markers, or a trailing
// one — must not produce a heading-only page.
func TestDesignedPDFEmptyGroupsProduceNoPages(t *testing.T) {
	t.Parallel()

	doc := designGroupedDocument()
	doc.Rows = []Row{
		{GroupTitle: "Klasse 1a"},
		{GroupTitle: "Klasse 2b"},
		{Values: map[ColumnID]string{ColumnName: "Conrad, Ida", ColumnSchoolClass: "2b"}},
		{GroupTitle: "Klasse 3c"},
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
		t.Fatalf("page count = %d, want 1 (heading-only pages dropped)", len(pages))
	}
	if pages[0].groupTitle != "Klasse 2b" {
		t.Fatalf("group title = %q, want %q", pages[0].groupTitle, "Klasse 2b")
	}
	if len(pages[0].rows) != 1 {
		t.Fatalf("row count = %d, want 1", len(pages[0].rows))
	}
}

// Ported rule: cell text wraps instead of being truncated — every word
// survives and no ellipsis is introduced.
func TestDesignedPDFWrapKeepsAllWords(t *testing.T) {
	t.Parallel()

	r, err := newDesignRenderer(designGroupedDocument())
	if err != nil {
		t.Fatalf("newDesignRenderer: %v", err)
	}
	if err := r.setFont(styleNormal, fontBody); err != nil {
		t.Fatalf("setFont: %v", err)
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
	t.Parallel()

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
	t.Parallel()

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

// P1 (review): names in scripts Inter lacks (Arabic, CJK, …) must reach
// the fallback font instead of being silently dropped by gopdf; runes no
// embedded font covers become a visible U+FFFD.
func TestDesignedPDFFallbackFontKeepsForeignNames(t *testing.T) {
	t.Parallel()

	r, err := newDesignRenderer(designGroupedDocument())
	if err != nil {
		t.Fatalf("newDesignRenderer: %v", err)
	}

	runs := splitFontRuns("Ali \u0645\u062d\u0645\u062f Wang \u738b", r.primaryCov, r.fallbackCov)
	var gotFallback, gotPrimary bool
	for _, run := range runs {
		if run.fallback {
			gotFallback = true
			continue
		}
		gotPrimary = true
	}
	if !gotFallback || !gotPrimary {
		t.Fatalf("expected mixed primary/fallback runs, got %+v", runs)
	}

	// A rune neither font covers (emoji, outside the BMP) must surface as
	// U+FFFD, never vanish.
	runs = splitFontRuns("Note \U0001F600", r.primaryCov, r.fallbackCov)
	var joined string
	for _, run := range runs {
		joined += run.text
	}
	if !strings.ContainsRune(joined, '\uFFFD') {
		t.Fatalf("uncovered rune not substituted visibly: %q", joined)
	}

	// End to end: an entirely Arabic name renders without error and the
	// fallback font is embedded in the output.
	doc := designGroupedDocument()
	doc.Rows = []Row{{Values: map[ColumnID]string{ColumnName: "\u0645\u062d\u0645\u062f \u0627\u0644\u0623\u0645\u064a\u0646", ColumnSchoolClass: "1a"}}}
	file, err := NewService().Render(doc, FormatPDF, "fallback")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !bytes.Contains(file.Data, []byte("/BaseFont /moto-fallback")) {
		t.Fatal("expected the fallback font to be embedded for the Arabic name")
	}
}

// P1 (review): a single row taller than the page body must be sliced into
// continuation rows — it used to draw straight through the footer and off
// the page, silently losing note text.
func TestDesignedPDFOversizedRowSplits(t *testing.T) {
	t.Parallel()

	longNote := strings.TrimSpace(strings.Repeat("sehr lange Tagesnotiz mit vielen Details ", 200))
	doc := designGroupedDocument()
	doc.Rows = []Row{{Values: map[ColumnID]string{
		ColumnName:        "Kind, Eines",
		ColumnSchoolClass: longNote,
	}}}

	r, err := newDesignRenderer(doc)
	if err != nil {
		t.Fatalf("newDesignRenderer: %v", err)
	}
	pages, err := r.paginate()
	if err != nil {
		t.Fatalf("paginate: %v", err)
	}
	if len(pages) < 2 {
		t.Fatalf("oversized row must spill onto continuation pages, got %d page(s)", len(pages))
	}

	// No slice may exceed what a page body can hold, and no wrapped line
	// may be lost across the slices.
	if err := r.setFont(styleNormal, fontBody); err != nil {
		t.Fatal(err)
	}
	full := r.buildRow(doc.Rows[0])
	// Ask the renderer for the group chrome instead of re-deriving it: this
	// document's rows carry no GroupTitle, so a hardcoded fontGroup+groupGap
	// would budget for a heading drawCard never draws and make the check
	// stricter than the page really is.
	avail := r.bodyBottom() - (r.bodyTop() + 2*cardPadY + r.groupChromeHeight() + r.tableHeaderHeight())
	maxLines := int((avail - 2*cellPadY) / rowLineHt)
	gotLines := 0
	for _, p := range pages {
		for _, rr := range p.rows {
			if rr.lines > maxLines {
				t.Fatalf("row slice has %d lines, page body holds %d", rr.lines, maxLines)
			}
			gotLines += len(rr.cells[1])
		}
	}
	if wantLines := len(full.cells[1]); gotLines != wantLines {
		t.Fatalf("lines across slices = %d, want %d (content lost)", gotLines, wantLines)
	}
}

// Review: column labels are administratively configured and unbounded. A
// label long enough to wrap past the header's line budget in a narrow column
// used to make tableHeaderHeight() exceed the whole card body — avail in
// paginate() went negative, the header drew through the footer and off the
// page, and the rows beneath it were clipped. The header band must stay
// bounded, leave room for at least one body row, and mark the truncated label
// with an ellipsis instead of losing it silently.
func TestDesignedPDFOversizedHeaderStaysBounded(t *testing.T) {
	t.Parallel()

	doc := designGroupedDocument()
	longLabel := strings.TrimSpace(strings.Repeat("Betreuungsanmeldestatuskategorie ", 60))
	doc.Columns = []Column{
		{ID: ColumnName, Label: "Name"},
		{ID: ColumnSchoolClass, Label: longLabel},
	}
	r, err := newDesignRenderer(doc)
	if err != nil {
		t.Fatalf("newDesignRenderer: %v", err)
	}
	if err := r.setFont(styleNormal, fontBody); err != nil {
		t.Fatal(err)
	}

	// The header plus at least one body row must fit the card region.
	region := r.bodyBottom() - r.bodyTop() - 2*cardPadY - r.groupChromeHeight()
	oneRow := rowLineHt + 2*cellPadY
	if hh := r.tableHeaderHeight(); hh+oneRow > region {
		t.Fatalf("header height %.1f + one row %.1f exceeds card region %.1f", hh, oneRow, region)
	}

	// paginate() must therefore leave real room for rows (avail positive) and
	// still place every body row.
	avail := r.bodyBottom() - (r.bodyTop() + 2*cardPadY + r.groupChromeHeight() + r.tableHeaderHeight())
	if avail < oneRow {
		t.Fatalf("avail for rows = %.1f, want at least one row (%.1f)", avail, oneRow)
	}
	pages, err := r.paginate()
	if err != nil {
		t.Fatalf("paginate: %v", err)
	}
	got := 0
	for _, p := range pages {
		got += len(p.rows)
	}
	if got != 3 {
		t.Fatalf("rows placed = %d, want 3 (none clipped by an oversized header)", got)
	}

	// The truncated label is marked, not silently dropped.
	lineCap := r.maxHeaderLines()
	_ = r.setFont(styleNormal, fontTableHd)
	drawn := r.wrapCapped(longLabel, r.widths[1]-cellPadX, lineCap)
	if len(drawn) > lineCap {
		t.Fatalf("drawn header lines = %d, exceed cap %d", len(drawn), lineCap)
	}
	if !strings.HasSuffix(drawn[len(drawn)-1], "…") {
		t.Fatalf("truncated header label = %q, want a trailing ellipsis", drawn[len(drawn)-1])
	}

	// End to end: a real render succeeds and stays a bounded document.
	if _, err := NewService().Render(doc, FormatPDF, "oversized-header"); err != nil {
		t.Fatalf("Render: %v", err)
	}
}

// A header that fits comfortably must not be touched — no ellipsis, every
// line intact. The cap only bites the pathological case above.
func TestDesignedPDFNormalHeaderNotTruncated(t *testing.T) {
	t.Parallel()

	doc := designGroupedDocument()
	doc.Columns = []Column{
		{ID: ColumnName, Label: "Name"},
		{ID: ColumnSchoolClass, Label: "Betreuungs- und Anmeldestatus"},
	}
	r, err := newDesignRenderer(doc)
	if err != nil {
		t.Fatalf("newDesignRenderer: %v", err)
	}
	lineCap := r.maxHeaderLines()
	if err := r.setFont(styleNormal, fontTableHd); err != nil {
		t.Fatal(err)
	}
	for i, col := range r.cols {
		capped := r.wrapCapped(col.Label, r.widths[i]-cellPadX, lineCap)
		free := r.wrap(col.Label, r.widths[i]-cellPadX)
		if len(capped) != len(free) {
			t.Fatalf("label %q capped to %d lines, free wrap gives %d", col.Label, len(capped), len(free))
		}
		if strings.HasSuffix(capped[len(capped)-1], "…") {
			t.Fatalf("label %q gained an ellipsis it did not need", col.Label)
		}
	}
}

// P2 (review): filter pills wrap onto additional header rows instead of
// running past the right page edge, and the body start moves down to make
// room.
func TestDesignedPDFFilterPillsWrapWithinPage(t *testing.T) {
	t.Parallel()

	doc := designGroupedDocument()
	for i := 0; i < 12; i++ {
		doc.Filters = append(doc.Filters, fmt.Sprintf("Betreuungsangebot: OGS Ganztag Variante %d", i))
	}
	r, err := newDesignRenderer(doc)
	if err != nil {
		t.Fatalf("newDesignRenderer: %v", err)
	}
	if len(r.pillRows) < 2 {
		t.Fatalf("expected pills to wrap onto multiple rows, got %d", len(r.pillRows))
	}

	if err := r.setFont(styleNormal, fontMeta); err != nil {
		t.Fatal(err)
	}
	usable := r.w - 2*pageMargin
	for i, row := range r.pillRows {
		rowW := 0.0
		for _, label := range row {
			tw, err := r.measure(label)
			if err != nil {
				t.Fatal(err)
			}
			rowW += tw + 14 + 6
		}
		if rowW-6 > usable {
			t.Fatalf("pill row %d width %.1f exceeds usable %.1f", i, rowW-6, usable)
		}
	}

	// The card must start below the extra pill rows.
	plain, err := newDesignRenderer(designGroupedDocument())
	if err != nil {
		t.Fatal(err)
	}
	if r.bodyTop() <= plain.bodyTop() {
		t.Fatalf("bodyTop = %.1f, want it below the single-row default %.1f", r.bodyTop(), plain.bodyTop())
	}
}

// P1 (review): filter metadata is caller-supplied and unbounded — the
// care-usage export joins every care-offering name into one label. Enough
// of it used to push bodyTop() past bodyBottom(): the record renderer then
// returned a successful PDF whose cards lay outside the media box, and the
// table renderer failed on invalid rectangle coordinates. The header must
// stay bounded and say how many filters it could not show.
func TestDesignedPDFFilterPillsBoundedToPage(t *testing.T) {
	t.Parallel()

	filters := make([]string, 0, 60)
	for i := 0; i < 60; i++ {
		filters = append(filters, fmt.Sprintf("Betreuungsangebot: %s %d", strings.Repeat("Ganztagsbetreuung ", 5), i))
	}

	for _, landscape := range []bool{false, true} {
		name := "portrait"
		if landscape {
			name = "landscape"
		}
		t.Run(name, func(t *testing.T) {
			c, err := newPageChrome(landscape, "Kinderliste — Nutzung", "42 Kinder", time.Now(), filters, "Vertraulich")
			if err != nil {
				t.Fatalf("newPageChrome: %v", err)
			}
			if got := c.bodyBottom() - c.bodyTop(); got < minBodyHeight {
				t.Fatalf("body height = %.1f, want at least %.1f", got, minBodyHeight)
			}
			if len(c.pillRows) > c.maxPillRows() {
				t.Fatalf("pill rows = %d, want at most %d", len(c.pillRows), c.maxPillRows())
			}

			// The dropped filters are accounted for, not silently gone.
			last := c.pillRows[len(c.pillRows)-1]
			if marker := last[len(last)-1]; !strings.HasPrefix(marker, "+") || !strings.Contains(marker, "Filter") {
				t.Fatalf("last pill = %q, want an overflow marker", marker)
			}

			// Rows still fit the page width, marker included.
			usable := c.w - 2*pageMargin
			for i, row := range c.pillRows {
				rowW := -pillGap
				for _, label := range row {
					pw, err := c.pillWidth(label)
					if err != nil {
						t.Fatal(err)
					}
					rowW += pw + pillGap
				}
				if rowW > usable {
					t.Fatalf("pill row %d width %.1f exceeds usable %.1f", i, rowW, usable)
				}
			}
		})
	}

	// Both renderers must produce a real document, not an empty-bodied one.
	doc := designGroupedDocument()
	doc.Filters = filters
	if _, err := renderPDFDesigned(doc); err != nil {
		t.Fatalf("renderPDFDesigned: %v", err)
	}

	recDoc := RecordDocument{
		Title:       "Anmeldungen — Nutzung",
		GeneratedAt: time.Now(),
		Filters:     filters,
		Records:     []Record{{Title: "Familie Muster", Fields: []Field{{Label: "E-Mail", Value: "a@example.test"}}}},
	}
	if _, err := renderRecordsPDFDesigned(recDoc); err != nil {
		t.Fatalf("renderRecordsPDFDesigned: %v", err)
	}
	chrome, err := newPageChrome(false, recDoc.Title, "", recDoc.GeneratedAt, filters, "")
	if err != nil {
		t.Fatal(err)
	}
	l := &recordLayout{pageChrome: chrome, doc: recDoc}
	if err := l.run(); err != nil {
		t.Fatalf("record layout: %v", err)
	}
	if !traceContains(l.trace, "Familie Muster") {
		t.Fatal("record content missing from the layout")
	}
	if l.y > l.bodyBottom() {
		t.Fatalf("record content ends at %.1f, past the page body %.1f", l.y, l.bodyBottom())
	}
}

// P2 (review): when a single filter wraps into more chunks than the row cap
// allows, the marker must not claim a further filter exists — every filter is
// on the page, the label just lost its tail.
func TestDesignedPDFFilterPillsTruncatedSingleFilter(t *testing.T) {
	t.Parallel()

	only := "Betreuungsangebot: " + strings.Repeat("Ganztagsbetreuung ", 400)

	c, err := newPageChrome(false, "Kinderliste — Nutzung", "42 Kinder", time.Now(), []string{only}, "")
	if err != nil {
		t.Fatalf("newPageChrome: %v", err)
	}
	last := c.pillRows[len(c.pillRows)-1]
	marker := last[len(last)-1]
	if strings.HasPrefix(marker, "+") {
		t.Fatalf("last pill = %q, want a truncation marker for a single filter", marker)
	}
	if marker != pillOverflowLabel(0) {
		t.Fatalf("last pill = %q, want %q", marker, pillOverflowLabel(0))
	}
}

// P3 (review): a decomposed title must not have the eyebrow's letter-spacing
// inserted between a base letter and its combining mark — text()'s NFC pass
// can no longer recompose the glyph afterwards, so the accent renders
// detached or drops out.
func TestSpaceOutKeepsCombiningMarksAttached(t *testing.T) {
	t.Parallel()

	const (
		nfd  = "Schu\u0308ler"      // u + combining diaeresis
		nfc  = "Sch\u00fcler"       // precomposed
		want = "S c h \u00fc l e r" // one space per grapheme, mark intact
	)
	if got := spaceOut(nfd); got != want {
		t.Fatalf("spaceOut(NFD) = %q, want %q", got, want)
	}
	if got := spaceOut(nfc); got != want {
		t.Fatalf("spaceOut(NFC) = %q, want %q", got, want)
	}

	// A combining mark with no precomposed form must stay with its base
	// letter too — NFC alone cannot rescue those.
	const cedilla = "z\u0327" // z + combining cedilla
	if got := spaceOut("a" + cedilla); got != "a "+cedilla {
		t.Fatalf("spaceOut(%q) = %q, split the combining mark off its base letter", "a"+cedilla, got)
	}
}

// The header reserve must equal the header actually drawn. bodyTop() used
// to be a flat "pageMargin + 104" sized for the fullest header (eyebrow +
// title + subtitle + one pill row); a Tagesliste has no eyebrow and often
// no filters, so the body started up to ~58pt below where the header
// ended. These two must never drift apart again.
func TestDesignedPDFHeaderReserveMatchesDrawnHeader(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		title    string
		subtitle string
		filters  []string
	}{
		{name: "full", title: "Kinderliste — OGS Wochenübersicht", subtitle: "42 Kinder", filters: []string{"Gruppe: A"}},
		{name: "tagesliste", title: "Tagesliste", subtitle: "42 Kinder", filters: nil},
		{name: "bare", title: "Tagesliste"},
		{name: "eyebrow only", title: "Kinderliste — Tagesliste"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := designGroupedDocument()
			doc.Title, doc.Subtitle, doc.Filters = tc.title, tc.subtitle, tc.filters
			r, err := newDesignRenderer(doc)
			if err != nil {
				t.Fatalf("newDesignRenderer: %v", err)
			}
			r.pdf.AddPage()
			drawn, err := r.drawHeader()
			if err != nil {
				t.Fatalf("drawHeader: %v", err)
			}
			if got := r.headerBottom(); got != drawn {
				t.Fatalf("headerBottom() = %.1f, but drawHeader ended at %.1f", got, drawn)
			}
			if r.bodyTop() <= drawn {
				t.Fatalf("bodyTop %.1f overlaps the header ending at %.1f", r.bodyTop(), drawn)
			}
		})
	}
}

// A user-supplied export title long enough to overrun the page must be cut
// with an ellipsis, not clipped at the page edge — and it must stay a single
// line so the header reserve keeps matching what drawHeader draws.
func TestDesignedPDFOversizedTitleStaysBounded(t *testing.T) {
	t.Parallel()

	longTitle := strings.TrimSpace(strings.Repeat("Betreuungsanmeldeliste ", 40))
	doc := designGroupedDocument()
	doc.Title, doc.Subtitle, doc.Filters = longTitle, "", nil
	r, err := newDesignRenderer(doc)
	if err != nil {
		t.Fatalf("newDesignRenderer: %v", err)
	}

	// The drawn headline fits the usable width and is marked as truncated.
	if err := r.setFont(styleBold, fontTitle); err != nil {
		t.Fatal(err)
	}
	usable := r.w - 2*pageMargin
	fitted := r.fitOneLine(titleHeadline(longTitle), usable)
	if wdt, err := r.measure(fitted); err != nil || wdt > usable {
		t.Fatalf("fitted title width = %.1f (err %v), want <= %.1f", wdt, err, usable)
	}
	if !strings.HasSuffix(fitted, "…") {
		t.Fatalf("oversized title = %q, want a trailing ellipsis", fitted)
	}

	// Single-line title keeps the header reserve honest.
	r.pdf.AddPage()
	drawn, err := r.drawHeader()
	if err != nil {
		t.Fatalf("drawHeader: %v", err)
	}
	if got := r.headerBottom(); got != drawn {
		t.Fatalf("headerBottom() = %.1f, but drawHeader ended at %.1f", got, drawn)
	}

	// End to end: a real render succeeds.
	if _, err := NewService().Render(doc, FormatPDF, "oversized-title"); err != nil {
		t.Fatalf("Render: %v", err)
	}
}

// A title that fits comfortably must render verbatim — no ellipsis.
func TestDesignedPDFNormalTitleNotTruncated(t *testing.T) {
	t.Parallel()

	doc := designGroupedDocument()
	doc.Title, doc.Subtitle, doc.Filters = "Tagesliste", "", nil
	r, err := newDesignRenderer(doc)
	if err != nil {
		t.Fatalf("newDesignRenderer: %v", err)
	}
	if err := r.setFont(styleBold, fontTitle); err != nil {
		t.Fatal(err)
	}
	usable := r.w - 2*pageMargin
	if fitted := r.fitOneLine(titleHeadline("Tagesliste"), usable); fitted != "Tagesliste" {
		t.Fatalf("normal title changed to %q, want %q", fitted, "Tagesliste")
	}
}

// An ungrouped document must not reserve room for a group heading that
// drawCard never draws: the card is sized from the same terms paginate()
// budgets with, so a full page's card must reach the page body's bottom
// to within one row.
func TestDesignedPDFUngroupedPagesFillBody(t *testing.T) {
	t.Parallel()

	doc := Document{
		Title:       "Tagesliste",
		Subtitle:    "60 Kinder",
		GeneratedAt: time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC),
		Columns:     []Column{{ID: ColumnName, Label: "Name"}, {ID: ColumnSchoolClass, Label: "Klasse"}},
	}
	for i := 0; i < 60; i++ {
		doc.Rows = append(doc.Rows, Row{Values: map[ColumnID]string{
			ColumnName: fmt.Sprintf("Muster, Kind %d", i), ColumnSchoolClass: "1a",
		}})
	}
	r, err := newDesignRenderer(doc)
	if err != nil {
		t.Fatalf("newDesignRenderer: %v", err)
	}
	if got := r.groupChromeHeight(); got != 0 {
		t.Fatalf("ungrouped document reserves %.1fpt of group chrome, want 0", got)
	}
	pages, err := r.paginate()
	if err != nil {
		t.Fatalf("paginate: %v", err)
	}
	if len(pages) < 2 {
		t.Fatalf("expected the list to spill across pages, got %d", len(pages))
	}
	if err := r.setFont(styleNormal, fontBody); err != nil {
		t.Fatal(err)
	}

	// Page 1 is full by construction (more rows follow). Its card bottom
	// must land within one row height of the body bottom — anything more
	// is space the pagination believed it did not have.
	used := 0.0
	for _, rr := range pages[0].rows {
		used += rr.height()
	}
	cardBottom := r.bodyTop() + 2*cardPadY + r.tableHeaderHeight() + used
	gap := r.bodyBottom() - cardBottom
	if gap < 0 {
		t.Fatalf("card overruns the page body by %.1fpt", -gap)
	}
	if maxGap := pages[0].rows[0].height(); gap > maxGap {
		t.Fatalf("dead space below a full card = %.1fpt, want <= one row (%.1fpt)", gap, maxGap)
	}
}

// #2079: a cell composed from several facts separates them with "\n" and
// gets one line each, regardless of how much width is left over. Without
// this the plan matrix ("07:30–14:00", task, room) reads as one run-on
// line, because wrap() used to tokenize with strings.Fields.
func TestDesignedPDFWrapHonoursExplicitLineBreaks(t *testing.T) {
	t.Parallel()

	r, err := newDesignRenderer(designGroupedDocument())
	if err != nil {
		t.Fatalf("newDesignRenderer: %v", err)
	}
	if err := r.setFont(styleNormal, fontBody); err != nil {
		t.Fatalf("setFont: %v", err)
	}

	lines := r.wrap("07:30–14:00\nKüche\nRaum 2", 400) // wide: width alone would keep one line
	want := []string{"07:30–14:00", "Küche", "Raum 2"}
	if len(lines) != len(want) {
		t.Fatalf("wrap lines = %v, want %v", lines, want)
	}
	for i, line := range lines {
		if line != want[i] {
			t.Fatalf("wrap line %d = %q, want %q", i, line, want[i])
		}
	}
}

// A segment that is itself too wide still wraps inside its own line break,
// so the two mechanisms compose instead of one disabling the other.
func TestDesignedPDFWrapWrapsWithinLineBreaks(t *testing.T) {
	t.Parallel()

	r, err := newDesignRenderer(designGroupedDocument())
	if err != nil {
		t.Fatalf("newDesignRenderer: %v", err)
	}
	if err := r.setFont(styleNormal, fontBody); err != nil {
		t.Fatalf("setFont: %v", err)
	}

	lines := r.wrap("07:30–14:00\nBetreuung der Randstunde mit Frühstück", 60)
	if len(lines) < 3 {
		t.Fatalf("wrap lines = %v, want the second segment to wrap further", lines)
	}
	if lines[0] != "07:30–14:00" {
		t.Fatalf("wrap line 0 = %q, want the untouched first segment", lines[0])
	}
	if joined := strings.Join(lines[1:], " "); joined != "Betreuung der Randstunde mit Frühstück" {
		t.Fatalf("wrap altered the wrapped segment: %q", joined)
	}
}

// Line breaks must survive pagination too: a three-line cell is a
// three-line row, otherwise rows overlap on the page.
func TestDesignedPDFLineBreaksGrowRowHeight(t *testing.T) {
	t.Parallel()

	doc := designGroupedDocument()
	doc.Rows = []Row{
		{Values: map[ColumnID]string{ColumnName: "Kessener, Franziska", ColumnSchoolClass: "a\nb\nc"}},
	}
	r, err := newDesignRenderer(doc)
	if err != nil {
		t.Fatalf("newDesignRenderer: %v", err)
	}
	if err := r.setFont(styleNormal, fontBody); err != nil {
		t.Fatalf("setFont: %v", err)
	}
	if got := r.buildRow(doc.Rows[0]).lines; got != 3 {
		t.Fatalf("row lines = %d, want 3", got)
	}
}

// The Notfallliste (#2609) prints a free-text health note next to a phone
// number. Whoever grabs that sheet in an emergency must get the WHOLE note:
// a silently clipped "Nussallergie, Epipen im…" is worse than no note at all.
// Two properties, both on the real six-column emergency layout: the health
// column is wide enough to wrap into readable lines, and wrapping keeps every
// word without introducing an ellipsis.
func TestDesignedPDFHealthColumnWrapsWithoutTruncation(t *testing.T) {
	t.Parallel()

	const note = "Erdnussallergie und Hausstaubmilbenallergie, Notfallset (Epipen) hängt im Gruppenraum neben der Tür, bei Atemnot sofort Notarzt rufen und die Mutter unter der ersten Nummer erreichen"

	doc := Document{
		Title:       "Notfallliste",
		GeneratedAt: time.Date(2026, 8, 26, 8, 0, 0, 0, time.UTC),
		Columns: []Column{
			{ID: ColumnName, Label: "Name"},
			{ID: ColumnSchoolClass, Label: "Klasse"},
			{ID: ColumnCurrentLocation, Label: "Ort / Raum"},
			{ID: ColumnContactPhone, Label: "Telefonnummer"},
			{ID: ColumnContactName, Label: "Kontakt"},
			{ID: ColumnHealthInfo, Label: "Gesundheit / Allergien"},
		},
		Rows: []Row{{Values: map[ColumnID]string{
			ColumnName:            "Albrecht, Mila",
			ColumnSchoolClass:     "3b",
			ColumnCurrentLocation: "Kreativraum",
			ColumnContactPhone:    "02551 111",
			ColumnContactName:     "Lea Albrecht",
			ColumnHealthInfo:      note,
		}}},
	}

	r, err := newDesignRenderer(doc)
	if err != nil {
		t.Fatalf("newDesignRenderer: %v", err)
	}
	if err := r.setFont(styleNormal, fontBody); err != nil {
		t.Fatal(err)
	}

	// The health column must be the widest on the sheet — a long note in a
	// name-sized column wraps into one word per line and stops being readable.
	healthWidth := r.widths[5]
	for i, w := range r.widths[:5] {
		if healthWidth <= w {
			t.Fatalf("health column width %.1f is not wider than column %d (%.1f)", healthWidth, i, w)
		}
	}

	row := r.buildRow(doc.Rows[0])
	lines := make([]string, 0, len(row.cells[5]))
	for _, ln := range row.cells[5] {
		if strings.Contains(ln.text, "...") || strings.Contains(ln.text, "…") {
			t.Fatalf("health note was truncated: %q", ln.text)
		}
		lines = append(lines, ln.text)
	}
	if len(lines) < 2 {
		t.Fatalf("health note rendered in %d line(s), expected it to wrap", len(lines))
	}
	if got := strings.Join(lines, " "); got != note {
		t.Fatalf("wrapping dropped or altered the health note:\n got: %q\nwant: %q", got, note)
	}

	// And the whole row still renders through the real pipeline.
	file, err := NewService().Render(doc, FormatPDF, "notfallliste")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !bytes.HasPrefix(file.Data, []byte("%PDF-1.")) {
		t.Fatalf("PDF header = %q", file.Data[:8])
	}
}
