package listexport

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
	"time"
)

// #2079: a plan cell stacks facts of different rank — a shift window, the
// tasks inside it, a note about a cancellation — and the whole legibility of
// the printed plan rides on those ranks surviving to paper. The first version
// of the export drew every line in one weight and one colour and could not be
// read from across a room. These tests pin the three ranks and the colour bar
// where they are decided.

func planDocument() Document {
	return Document{
		Title:       "Dienstplan",
		Subtitle:    "KW 31 · 27.07.–31.07.2026",
		GeneratedAt: time.Date(2026, 7, 24, 8, 0, 0, 0, time.UTC),
		Filters:     []string{"Aushang", "Fett: Dienstzeit · darunter die Aufgaben"},
		Columns: []Column{
			{ID: ColumnPlanRowLabel, Label: "Mitarbeitende"},
			{ID: ColumnPlanMonday, Label: "Montag, 27.07."},
			{ID: ColumnPlanTuesday, Label: "Dienstag, 28.07."},
		},
		Rows: []Row{{Values: map[ColumnID]string{
			ColumnPlanRowLabel: StyledCell([]Line{{Text: "Kessener, Franziska", Style: LineStrong}}),
			ColumnPlanMonday: StyledCell([]Line{
				{Text: "07:30–14:00 · Ganztag", Style: LineStrong, Accent: "#83CD2D"},
				{Text: "12:00–13:00 Mensa · Speisesaal", Style: LineNormal},
				{Text: "ohne Schicht 12:30–13:00", Style: LineMuted},
				{Text: "15:00–16:30 · Randstunde", Style: LineStrong, Accent: "#5080D8"},
			}),
			ColumnPlanTuesday: StyledCell([]Line{{Text: "—", Style: LineMuted}}),
		}}},
	}
}

// The markers are an encoding detail of the cell string: they must survive a
// round trip and never reach a reader.
func TestStyledCellRoundTrip(t *testing.T) {
	lines := []Line{
		{Text: "07:30–14:00", Style: LineStrong, Accent: "#83CD2D"},
		{Text: "Mensa", Style: LineNormal},
		{Text: "Grund: Fortbildung", Style: LineMuted},
	}
	cell := StyledCell(lines)

	for i, encoded := range strings.Split(cell, "\n") {
		decoded, text := DecodeLine(encoded)
		if decoded != lines[i] {
			t.Fatalf("line %d decoded as %+v, want %+v", i, decoded, lines[i])
		}
		if text != lines[i].Text {
			t.Fatalf("line %d text = %q, want %q", i, text, lines[i].Text)
		}
	}

	stripped := StripStyleMarkers(cell)
	want := "07:30–14:00\nMensa\nGrund: Fortbildung"
	if stripped != want {
		t.Fatalf("stripped cell = %q, want %q", stripped, want)
	}
	for _, marker := range []string{markStrong, markMuted, accentMark} {
		if strings.Contains(stripped, marker) {
			t.Fatalf("stripped cell still carries a control character: %q", stripped)
		}
	}
}

// The gutter that holds the colour bar is derived from the rows, so a
// document without accents keeps the old full-width cells and every existing
// export is untouched.
func TestAccentGutterOnlyWhenAccentsExist(t *testing.T) {
	r, err := newDesignRenderer(planDocument())
	if err != nil {
		t.Fatalf("newDesignRenderer: %v", err)
	}
	if r.gutter != accentInset {
		t.Fatalf("gutter = %.1f, want the accent inset %.1f", r.gutter, accentInset)
	}

	plain, err := newDesignRenderer(designGroupedDocument())
	if err != nil {
		t.Fatalf("newDesignRenderer: %v", err)
	}
	if plain.gutter != 0 {
		t.Fatalf("gutter = %.1f on an accent-free document, want 0", plain.gutter)
	}
	if documentHasAccents(designGroupedDocument().Rows) {
		t.Fatal("an accent-free document must not be reported as accented")
	}
}

// A heading long enough to wrap keeps one bar: the accent opens the group and
// runs on, rather than restarting on the heading's own second line.
func TestWrapStyledKeepsRanksAndOpensOneAccentGroup(t *testing.T) {
	r, err := newDesignRenderer(planDocument())
	if err != nil {
		t.Fatalf("newDesignRenderer: %v", err)
	}

	cell := StyledCell([]Line{
		{Text: "07:30–14:00 Ganztagsbetreuung mit Randstunde und Mittagessen", Style: LineStrong, Accent: "#83CD2D"},
		{Text: "Mensa", Style: LineNormal},
		{Text: "Hinweis", Style: LineMuted},
	})
	lines := r.wrapStyled(cell, 60) // narrow: forces the first line to wrap
	if len(lines) < 4 {
		t.Fatalf("wrapped lines = %+v, want the heading to wrap", lines)
	}
	if lines[0].style != LineStrong || lines[0].accent != "#83CD2D" {
		t.Fatalf("first line = %+v, want a strong accented anchor", lines[0])
	}

	accents, styles := 0, map[LineStyle]int{}
	for _, line := range lines {
		if line.accent != "" {
			accents++
		}
		styles[line.style]++
	}
	if accents != 1 {
		t.Fatalf("accent openers = %d, want exactly one per group", accents)
	}
	if styles[LineNormal] != 1 || styles[LineMuted] != 1 {
		t.Fatalf("styles = %v, want the normal and muted lines preserved", styles)
	}
	if fontStyleFor(LineStrong) != styleBold || fontStyleFor(LineNormal) != styleNormal {
		t.Fatal("strong lines must be drawn in the bold face")
	}
	if lineColorFor(LineMuted) != colorMuted || lineColorFor(LineStrong) != colorBody {
		t.Fatal("only muted lines recede in colour")
	}
}

// The bar colour comes straight from the colour columns of the database, which
// store "#RRGGBB". Anything else is ignored rather than guessed at — a broken
// value costs the bar, never the row.
func TestParseHexColor(t *testing.T) {
	if got, ok := parseHexColor(" #83CD2D "); !ok || got != (rgb{0x83, 0xCD, 0x2D}) {
		t.Fatalf("parseHexColor = %v, %v", got, ok)
	}
	if got, ok := parseHexColor("83CD2D"); !ok || got != (rgb{0x83, 0xCD, 0x2D}) {
		t.Fatalf("a hash-less value must still parse, got %v, %v", got, ok)
	}
	for _, bad := range []string{"", "#83CD2", "#83CD2DFF", "#GGCD2D", "rot"} {
		if _, ok := parseHexColor(bad); ok {
			t.Fatalf("parseHexColor(%q) accepted an unusable value", bad)
		}
	}
}

// End to end on the PDF path: a plan renders, and a broken accent value takes
// the bar with it and nothing else.
func TestPlanDocumentRendersToPDF(t *testing.T) {
	file, err := NewService().Render(planDocument(), FormatPDF, "dienstplan")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !bytes.HasPrefix(file.Data, []byte("%PDF-1.")) {
		t.Fatalf("PDF header = %q", file.Data[:8])
	}

	broken := planDocument()
	broken.Rows[0].Values[ColumnPlanMonday] = StyledCell([]Line{
		{Text: "07:30–14:00 · Ganztag", Style: LineStrong, Accent: "nicht-hex"},
		{Text: "12:00–13:00 Mensa", Style: LineNormal},
	})
	if _, err := NewService().Render(broken, FormatPDF, "dienstplan"); err != nil {
		t.Fatalf("an unusable accent must not fail the render: %v", err)
	}
}

// XLSX has no per-line emphasis, so the markers are stripped — but the line
// structure survives, which is what makes a plan cell readable in a
// spreadsheet at all.
func TestPlanDocumentRendersToXLSXWithoutMarkers(t *testing.T) {
	file, err := NewService().Render(planDocument(), FormatXLSX, "dienstplan")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(file.Data), int64(len(file.Data)))
	if err != nil {
		t.Fatalf("open xlsx: %v", err)
	}
	var sheet bytes.Buffer
	for _, entry := range reader.File {
		if !strings.HasPrefix(entry.Name, "xl/") {
			continue
		}
		rc, err := entry.Open()
		if err != nil {
			t.Fatalf("open %s: %v", entry.Name, err)
		}
		if _, err := sheet.ReadFrom(rc); err != nil {
			t.Fatalf("read %s: %v", entry.Name, err)
		}
		_ = rc.Close()
	}

	content := sheet.String()
	for _, marker := range []string{markStrong, markMuted, accentMark} {
		if strings.Contains(content, marker) {
			t.Fatal("style markers reached the spreadsheet")
		}
	}
	if !strings.Contains(content, "07:30") || !strings.Contains(content, "Speisesaal") {
		t.Fatal("plan cell content missing from the spreadsheet")
	}
}

// A cell holding only whitespace is one empty line, not a dropped row.
func TestWrapBlankCellKeepsOneLine(t *testing.T) {
	r, err := newDesignRenderer(planDocument())
	if err != nil {
		t.Fatalf("newDesignRenderer: %v", err)
	}
	if err := r.setFont(styleNormal, fontBody); err != nil {
		t.Fatal(err)
	}
	if got := r.wrap("   ", 200); len(got) != 1 || got[0] != "" {
		t.Fatalf("wrap(blank) = %q, want a single empty line", got)
	}
}
