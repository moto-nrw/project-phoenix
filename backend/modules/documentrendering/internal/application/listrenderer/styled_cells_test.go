package listexport

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
	"time"

	listrecords "github.com/moto-nrw/project-phoenix/modules/documentrendering/records"
)

// #2079: a plan cell stacks facts of different rank — a shift window, the
// tasks inside it, a note about a cancellation — and the whole legibility of
// the printed plan rides on those ranks surviving to paper. The first version
// of the export drew every line in one weight and one colour and could not be
// read from across a room. These tests pin the three ranks and the colour bar
// where they are decided.

func planDocument() listrecords.Document {
	return listrecords.Document{
		Title:       "Dienstplan",
		Subtitle:    "KW 31 · 27.07.–31.07.2026",
		GeneratedAt: time.Date(2026, 7, 24, 8, 0, 0, 0, time.UTC),
		Filters:     []string{"Aushang", "Fett: Dienstzeit · darunter die Aufgaben"},
		Columns: []listrecords.Column{
			{ID: listrecords.ColumnPlanRowLabel, Label: "Mitarbeitende"},
			{ID: listrecords.ColumnPlanMonday, Label: "Montag, 27.07."},
			{ID: listrecords.ColumnPlanTuesday, Label: "Dienstag, 28.07."},
		},
		Rows: []listrecords.Row{{Values: map[listrecords.ColumnID]string{
			listrecords.ColumnPlanRowLabel: listrecords.StyledCell([]listrecords.Line{{Text: "Kessener, Franziska", Style: listrecords.LineStrong}}),
			listrecords.ColumnPlanMonday: listrecords.StyledCell([]listrecords.Line{
				{Text: "07:30–14:00 · Ganztag", Style: listrecords.LineStrong, Accent: "#83CD2D"},
				{Text: "12:00–13:00 Mensa · Speisesaal", Style: listrecords.LineNormal},
				{Text: "ohne Schicht 12:30–13:00", Style: listrecords.LineMuted},
				{Text: "15:00–16:30 · Randstunde", Style: listrecords.LineStrong, Accent: "#5080D8"},
			}),
			listrecords.ColumnPlanTuesday: listrecords.StyledCell([]listrecords.Line{{Text: "—", Style: listrecords.LineMuted}}),
		}}},
	}
}

// The markers are an encoding detail of the cell string: they must survive a
// round trip and never reach a reader.
func TestStyledCellRoundTrip(t *testing.T) {
	t.Parallel()

	lines := []listrecords.Line{
		{Text: "07:30–14:00", Style: listrecords.LineStrong, Accent: "#83CD2D"},
		{Text: "Mensa", Style: listrecords.LineNormal},
		{Text: "Grund: Fortbildung", Style: listrecords.LineMuted},
	}
	cell := listrecords.StyledCell(lines)

	for i, encoded := range strings.Split(cell, "\n") {
		decoded, text := listrecords.DecodeLine(encoded)
		if decoded != lines[i] {
			t.Fatalf("line %d decoded as %+v, want %+v", i, decoded, lines[i])
		}
		if text != lines[i].Text {
			t.Fatalf("line %d text = %q, want %q", i, text, lines[i].Text)
		}
	}

	stripped := listrecords.StripStyleMarkers(cell)
	want := "07:30–14:00\nMensa\nGrund: Fortbildung"
	if stripped != want {
		t.Fatalf("stripped cell = %q, want %q", stripped, want)
	}
	for _, marker := range []string{listrecords.StrongMarker, listrecords.MutedMarker, listrecords.AccentMarker} {
		if strings.Contains(stripped, marker) {
			t.Fatalf("stripped cell still carries a control character: %q", stripped)
		}
	}
}

// The gutter that holds the colour bar is derived from the rows, so a
// document without accents keeps the old full-width cells and every existing
// export is untouched.
func TestAccentGutterOnlyWhenAccentsExist(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	r, err := newDesignRenderer(planDocument())
	if err != nil {
		t.Fatalf("newDesignRenderer: %v", err)
	}

	cell := listrecords.StyledCell([]listrecords.Line{
		{Text: "07:30–14:00 Ganztagsbetreuung mit Randstunde und Mittagessen", Style: listrecords.LineStrong, Accent: "#83CD2D"},
		{Text: "Mensa", Style: listrecords.LineNormal},
		{Text: "Hinweis", Style: listrecords.LineMuted},
	})
	lines := r.wrapStyled(cell, 60) // narrow: forces the first line to wrap
	if len(lines) < 4 {
		t.Fatalf("wrapped lines = %+v, want the heading to wrap", lines)
	}
	if lines[0].style != listrecords.LineStrong || lines[0].accent != "#83CD2D" {
		t.Fatalf("first line = %+v, want a strong accented anchor", lines[0])
	}

	accents, styles := 0, map[listrecords.LineStyle]int{}
	for _, line := range lines {
		if line.accent != "" {
			accents++
		}
		styles[line.style]++
	}
	if accents != 1 {
		t.Fatalf("accent openers = %d, want exactly one per group", accents)
	}
	if styles[listrecords.LineNormal] != 1 || styles[listrecords.LineMuted] != 1 {
		t.Fatalf("styles = %v, want the normal and muted lines preserved", styles)
	}
	if fontStyleFor(listrecords.LineStrong) != styleBold || fontStyleFor(listrecords.LineNormal) != styleNormal {
		t.Fatal("strong lines must be drawn in the bold face")
	}
	if lineColorFor(listrecords.LineMuted) != colorMuted || lineColorFor(listrecords.LineStrong) != colorBody {
		t.Fatal("only muted lines recede in colour")
	}
}

// The bar colour comes straight from the colour columns of the database, which
// store "#RRGGBB" or the "#RGB" shorthand. Anything else is ignored rather than
// guessed at — a broken value costs the bar, never the row.
func TestParseHexColor(t *testing.T) {
	t.Parallel()

	if got, ok := parseHexColor(" #83CD2D "); !ok || got != (rgb{0x83, 0xCD, 0x2D}) {
		t.Fatalf("parseHexColor = %v, %v", got, ok)
	}
	if got, ok := parseHexColor("83CD2D"); !ok || got != (rgb{0x83, 0xCD, 0x2D}) {
		t.Fatalf("a hash-less value must still parse, got %v, %v", got, ok)
	}
	// The colour columns validate against ^#([A-Fa-f0-9]{6}|[A-Fa-f0-9]{3})$,
	// so the shorthand reaches the renderer and must keep its bar.
	if got, ok := parseHexColor("#abc"); !ok || got != (rgb{0xAA, 0xBB, 0xCC}) {
		t.Fatalf("the #RGB shorthand must expand, got %v, %v", got, ok)
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
	t.Parallel()

	file, err := NewService().Render(planDocument(), listrecords.FormatPDF, "dienstplan")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !bytes.HasPrefix(file.Data, []byte("%PDF-1.")) {
		t.Fatalf("PDF header = %q", file.Data[:8])
	}

	broken := planDocument()
	broken.Rows[0].Values[listrecords.ColumnPlanMonday] = listrecords.StyledCell([]listrecords.Line{
		{Text: "07:30–14:00 · Ganztag", Style: listrecords.LineStrong, Accent: "nicht-hex"},
		{Text: "12:00–13:00 Mensa", Style: listrecords.LineNormal},
	})
	if _, err := NewService().Render(broken, listrecords.FormatPDF, "dienstplan"); err != nil {
		t.Fatalf("an unusable accent must not fail the render: %v", err)
	}
}

// XLSX has no per-line emphasis, so the markers are stripped — but the line
// structure survives, which is what makes a plan cell readable in a
// spreadsheet at all.
func TestPlanDocumentRendersToXLSXWithoutMarkers(t *testing.T) {
	t.Parallel()

	file, err := NewService().Render(planDocument(), listrecords.FormatXLSX, "dienstplan")
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
	for _, marker := range []string{listrecords.StrongMarker, listrecords.MutedMarker, listrecords.AccentMarker} {
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
	t.Parallel()

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

// A Schichtart without a stored colour still ends the group above it. Without
// that, the bar of the coloured shift above would run down over the uncoloured
// one and assign it to a category it does not belong to — the colour coding
// would then be actively wrong rather than merely absent.
func TestAnchorWithoutColourEndsTheAccentGroup(t *testing.T) {
	t.Parallel()

	r, err := newDesignRenderer(planDocument())
	if err != nil {
		t.Fatalf("newDesignRenderer: %v", err)
	}

	cell := listrecords.StyledCell([]listrecords.Line{
		{Text: "07:30–14:00 · Ganztag", Style: listrecords.LineStrong, Accent: "#83CD2D"},
		{Text: "12:00–13:00 Mensa", Style: listrecords.LineNormal},
		{Text: "15:00–16:30", Style: listrecords.LineStrong},
		{Text: "16:00–16:30 Abholung", Style: listrecords.LineNormal},
	})
	lines := r.wrapStyled(cell, 400) // wide: one wrapped line per input line
	if len(lines) != 4 {
		t.Fatalf("wrapped lines = %+v, want one per input line", lines)
	}

	opens := []bool{true, false, true, false}
	for i, want := range opens {
		if lines[i].opensGroup() != want {
			t.Fatalf("line %d (%q) opens = %v, want %v", i, lines[i].text, lines[i].opensGroup(), want)
		}
	}
	if lines[2].accent != "" {
		t.Fatalf("uncoloured anchor accent = %q, want none", lines[2].accent)
	}
}

// The same across a page break: a slice starting inside a group re-opens it,
// but only with the colour of the anchor it actually sits under.
func TestSplitRenderRowDoesNotCarryAccentPastAnUncolouredAnchor(t *testing.T) {
	t.Parallel()

	row := renderRow{cells: [][]styledLine{{
		{text: "07:30–14:00", style: listrecords.LineStrong, accent: "#83CD2D", opens: true},
		{text: "Mensa", style: listrecords.LineNormal},
		{text: "15:00–16:30", style: listrecords.LineStrong, opens: true},
		{text: "Abholung", style: listrecords.LineNormal},
	}}, lines: 4}

	parts := splitRenderRow(row, 3)
	if len(parts) != 2 {
		t.Fatalf("parts = %d, want 2", len(parts))
	}
	if got := parts[1].cells[0][0]; got.accent != "" {
		t.Fatalf("continuation accent = %q, want none — the group above it carries no colour", got.accent)
	}
}
