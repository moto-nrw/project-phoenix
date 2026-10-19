package listexport

// The designed PDF renderer (issue #1568), built on signintech/gopdf.
// Renders listexport.Document in the app's design language: embedded Inter
// (real Unicode — no transliteration), moto logo, dotted page background,
// white content card, DataTable chrome. The shared page frame (pageChrome)
// is also used by the record/block renderer in records_design.go.

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	listrecords "github.com/moto-nrw/project-phoenix/modules/documentrendering/records"

	"github.com/signintech/gopdf"
	"golang.org/x/text/unicode/norm"
)

//go:embed assets/fonts/Inter-Regular.ttf
var interRegular []byte

//go:embed assets/fonts/Inter-SemiBold.ttf
var interSemiBold []byte

//go:embed assets/moto-logo.png
var motoLogoPNG []byte

const (
	fontFamily  = "inter"
	styleNormal = ""
	styleBold   = "B"
)

// norms normalizes to NFC. gopdf (like the whole gofpdf family) does no
// OpenType shaping: decomposed input (NFD — common from macOS) would drop
// its combining marks silently. Precomposed codepoints render correctly.
func norms(s string) string { return norm.NFC.String(s) }

// renderRow is one table row with every cell already wrapped to its
// column width, so pagination, splitting, and drawing all work on the
// same line counts and cannot disagree.
type styledLine struct {
	text   string
	style  listrecords.LineStyle
	accent string
	// opens marks the first line of an anchor: it starts a new accent group
	// whether or not it carries a colour. Without it an uncoloured anchor
	// would be indistinguishable from the lines below the anchor above it,
	// and the previous colour bar would run on over a group it does not
	// belong to.
	opens bool
}

// opensGroup reports whether the accent group above this line ends here. Any
// line carrying an accent of its own opens a group by definition; an anchor
// opens one even when its colour is missing or unusable.
func (l styledLine) opensGroup() bool { return l.opens || l.accent != "" }

type renderRow struct {
	cells [][]styledLine // per column: wrapped, styled lines
	lines int            // max line count across cells
}

func (rr renderRow) height() float64 {
	return float64(rr.lines)*rowLineHt + 2*cellPadY
}

type designPage struct {
	groupTitle string
	rows       []renderRow
}

// pageChrome is the shared page frame every designed PDF draws through:
// dotted background, logo header with title/subtitle/filter pills, and
// the footer carrying the confidentiality note + page numbers. Both the
// table renderer (listrecords.Document) and the record/block renderer (listrecords.RecordDocument)
// use it, so the two layouts cannot drift apart.
type pageChrome struct {
	pdf         *gopdf.GoPdf
	w, h        float64
	logo        gopdf.ImageHolder
	bgTpl       int
	title       string
	subtitle    string
	generatedAt time.Time
	filters     []string
	footer      string

	// Glyph fallback (see font_fallback.go): coverage of the primary and
	// fallback fonts, plus the current font state so fallback runs can be
	// drawn at the right size and the primary font restored afterwards.
	primaryCov  *fontCoverage
	fallbackCov *fontCoverage
	curStyle    string
	curSize     float64

	// pillRows is the pre-computed filter-pill layout (computed once in
	// newPageChrome): pills wrap onto additional rows instead of running
	// off the right page edge, and bodyTop grows with the extra rows.
	pillRows [][]string
}

type designRenderer struct {
	*pageChrome
	doc    listrecords.Document
	cols   []listrecords.Column
	widths []float64
	total  int
	// gutter is the left inset every body cell reserves for the accent bar.
	// It is derived from the document rather than passed in: the width has to
	// be the same for every cell, and a caller-supplied flag is one more thing
	// that can disagree with the rows it describes.
	gutter float64
}

const (
	accentBarW  = 2.5
	accentInset = 7.0
)

// documentHasAccents reports whether any cell opens an accent group.
func documentHasAccents(rows []listrecords.Row) bool {
	for _, row := range rows {
		for _, value := range row.Values {
			if strings.Contains(value, listrecords.AccentMarker) {
				return true
			}
		}
	}
	return false
}

// newDesignRenderer prepares a renderer with fonts loaded but nothing
// drawn — split out so pagination decisions are unit-testable.
func newDesignRenderer(doc listrecords.Document) (*designRenderer, error) {
	cols := doc.Columns
	if len(cols) == 0 {
		cols = listrecords.ResolveColumns(nil, listrecords.PresetOGSWeekly)
	}

	chrome, err := newPageChrome(true, doc.Title, doc.Subtitle, doc.GeneratedAt, doc.Filters, doc.Footer)
	if err != nil {
		return nil, err
	}

	r := &designRenderer{pageChrome: chrome, doc: doc, cols: cols}
	if documentHasAccents(doc.Rows) {
		r.gutter = accentInset
	}
	r.widths = pdfColumnWidths(cols, r.w-2*pageMargin-2*cardPadX)
	return r, nil
}

func renderPDFDesigned(doc listrecords.Document) ([]byte, error) {
	r, err := newDesignRenderer(doc)
	if err != nil {
		return nil, err
	}

	pages, err := r.paginate()
	if err != nil {
		return nil, err
	}
	r.total = len(pages)

	for i, p := range pages {
		if err := r.drawPage(p, i+1); err != nil {
			return nil, err
		}
	}

	var buf bytes.Buffer
	if _, err := r.pdf.WriteTo(&buf); err != nil {
		return nil, fmt.Errorf("write pdf: %w", err)
	}
	return buf.Bytes(), nil
}

// pdfColumnWidths distributes the usable width across columns by
// per-column weight.
func pdfColumnWidths(cols []listrecords.Column, total float64) []float64 {
	weights := make([]float64, len(cols))
	sum := 0.0
	for i, col := range cols {
		weight := 1.0
		switch col.ID {
		case listrecords.ColumnName:
			weight = 1.4
		case listrecords.ColumnSchoolClass:
			weight = 0.65
		case listrecords.ColumnGroup:
			weight = 1.2
		case listrecords.ColumnWeeklyMonday, listrecords.ColumnWeeklyTuesday, listrecords.ColumnWeeklyWednesday, listrecords.ColumnWeeklyThursday, listrecords.ColumnWeeklyFriday:
			weight = 0.9
		case listrecords.ColumnDeparture:
			weight = 1.5
		case listrecords.ColumnGuardianContacts:
			weight = 2.7
		case listrecords.ColumnIBAN:
			// A German IBAN is 22 characters and must not wrap: a bank
			// transfer typed off a broken line is a wrong transfer.
			weight = 2.0
		case listrecords.ColumnHealthInfo:
			// A free-text health note is the widest cell on the Notfallliste
			// (#2609): it has to wrap into readable lines rather than into a
			// one-word-per-line column, so it gets roughly twice a name's width.
			weight = 2.4
		case listrecords.ColumnPlanRowLabel:
			// A plan matrix inverts the child-list balance: the row label is
			// one name or one area, the day cells carry everything else.
			weight = 1.1
		case listrecords.ColumnPlanMonday, listrecords.ColumnPlanTuesday, listrecords.ColumnPlanWednesday, listrecords.ColumnPlanThursday,
			listrecords.ColumnPlanFriday, listrecords.ColumnPlanSaturday, listrecords.ColumnPlanSunday:
			weight = 1.6
		}
		weights[i] = weight
		sum += weight
	}
	widths := make([]float64, len(cols))
	for i, weight := range weights {
		widths[i] = total * weight / sum
	}
	return widths
}

// groupChromeHeight is the vertical cost of a group heading inside the
// card, or 0 for an ungrouped document. Grouped documents start every
// group on a fresh page and carry the title onto continuation pages, so
// every page of a grouped document pays it and no page of an ungrouped
// one does — which is exactly what drawCard draws.
func (r *designRenderer) groupChromeHeight() float64 {
	for _, row := range r.doc.Rows {
		if row.GroupTitle != "" {
			return fontGroup + groupGap
		}
	}
	return 0
}

// paginate wraps every cell, splits rows into pages, and slices rows
// taller than a page body into continuation rows. Group titles start a
// new page, mirroring the old renderer's behaviour (each class gets its
// own sheet).
func (r *designRenderer) paginate() ([]designPage, error) {
	if err := r.setFont(styleNormal, fontBody); err != nil {
		return nil, err
	}

	// Rows that fit inside one card: the card's own chrome (padding, an
	// optional group heading, the table header) comes off the top first.
	// The heading is only deducted when the document actually has groups —
	// an ungrouped list (the Tagesliste builds its rows without a
	// GroupTitle) had 24pt reserved on every page for a heading drawCard
	// never draws, which cost a full row per page.
	avail := r.bodyBottom() - (r.bodyTop() + 2*cardPadY + r.groupChromeHeight() + r.tableHeaderHeight())
	// The tallest single row a fresh page can hold; taller rows are
	// sliced so no content draws past the page body (they used to run
	// through the footer and off the page).
	maxRowLines := int((avail - 2*cellPadY) / rowLineHt)
	if maxRowLines < 1 {
		maxRowLines = 1
	}

	pages := []designPage{}
	cur := designPage{}
	used := 0.0
	wroteBody := false

	// A page is only kept once it holds a body row: a group marker followed
	// by another marker (or by nothing) would otherwise print a heading-only
	// sheet.
	flush := func() {
		if wroteBody {
			pages = append(pages, cur)
		}
	}

	for _, row := range r.doc.Rows {
		if row.GroupTitle != "" {
			flush()
			cur = designPage{groupTitle: row.GroupTitle}
			used, wroteBody = 0, false
			continue
		}
		for _, rr := range splitRenderRow(r.buildRow(row), maxRowLines) {
			hgt := rr.height()
			if used+hgt > avail && wroteBody {
				pages = append(pages, cur)
				cur = designPage{groupTitle: cur.groupTitle, rows: nil}
				used, wroteBody = 0, false
			}
			cur.rows = append(cur.rows, rr)
			used += hgt
			wroteBody = true
		}
	}
	flush()
	if len(pages) == 0 {
		pages = []designPage{{}}
	}
	return pages, nil
}

// buildRow wraps every cell of row to its column width, keeping each line's
// style. A segment is wrapped with its own font set, so a bold line is
// measured as bold and cannot overrun its column.
func (r *designRenderer) buildRow(row listrecords.Row) renderRow {
	rr := renderRow{cells: make([][]styledLine, len(r.cols)), lines: 1}
	for i, col := range r.cols {
		rr.cells[i] = r.wrapStyled(norms(row.Values[col.ID]), r.widths[i]-cellPadX-r.gutter)
		if n := len(rr.cells[i]); n > rr.lines {
			rr.lines = n
		}
	}
	return rr
}

// wrapStyled splits a cell into its styled lines and wraps each to maxW.
func (r *designRenderer) wrapStyled(cell string, maxW float64) []styledLine {
	cell = strings.ReplaceAll(strings.ReplaceAll(cell, "\r\n", "\n"), "\r", "\n")
	out := []styledLine{}
	for _, segment := range strings.Split(cell, "\n") {
		decoded, text := listrecords.DecodeLine(segment)
		// Measure in the face the line will be drawn in; a bold run is wider
		// than the same string in the regular face.
		if err := r.setFont(fontStyleFor(decoded.Style), fontBody); err != nil {
			return []styledLine{{text: text}}
		}
		for i, line := range r.wrapSegment(text, maxW) {
			wrapped := styledLine{text: line, style: decoded.Style}
			// Only the first wrapped line opens the accent group; the bar
			// then runs on until the next anchor, so a heading that wraps
			// does not restart its own bar.
			if i == 0 {
				wrapped.accent = decoded.Accent
				wrapped.opens = decoded.Style == listrecords.LineStrong
			}
			out = append(out, wrapped)
		}
	}
	// Restore the body face so callers that measure afterwards are unaffected.
	_ = r.setFont(styleNormal, fontBody)
	if len(out) == 0 {
		return []styledLine{{}}
	}
	return out
}

func fontStyleFor(style listrecords.LineStyle) string {
	if style == listrecords.LineStrong {
		return styleBold
	}
	return styleNormal
}

func lineColorFor(style listrecords.LineStyle) rgb {
	if style == listrecords.LineMuted {
		return colorMuted
	}
	return colorBody
}

// splitRenderRow slices a row taller than maxLines into continuation
// rows of at most maxLines wrapped lines each — no line is dropped.
func splitRenderRow(rr renderRow, maxLines int) []renderRow {
	if rr.lines <= maxLines {
		return []renderRow{rr}
	}
	out := []renderRow{}
	for start := 0; start < rr.lines; start += maxLines {
		part := renderRow{cells: make([][]styledLine, len(rr.cells)), lines: 1}
		for i, lines := range rr.cells {
			part.cells[i] = splitStyledLines(lines, start, maxLines)
			if n := len(part.cells[i]); n > part.lines {
				part.lines = n
			}
		}
		out = append(out, part)
	}
	return out
}

// tableHeaderHeight wraps every column label to its column width (with
// the header font pinned) and sizes the header band for the tallest one —
// long labels ("Betreuungs-/Anmeldestatus") stack instead of overlapping
// their neighbour column.
func (r *designRenderer) tableHeaderHeight() float64 {
	return float64(r.headerLineCount())*rowLineHt + 2*cellPadY + 2
}

// maxHeaderLines caps how many wrapped lines the table header may occupy so
// the header band always leaves room for at least one body row. listrecords.Column
// labels are administratively configured and unbounded: a label long enough
// to wrap past this cap in a narrow column would push the header past the
// footer and clip every row drawn beneath it (avail in paginate() goes
// negative). The header wraps freely up to the cap and only truncates in
// that pathological case — a normal header stays far below it.
func (r *designRenderer) maxHeaderLines() int {
	// The card region below the header band that the header and the first
	// body row must share (mirrors paginate()'s avail decomposition).
	region := r.bodyBottom() - r.bodyTop() - 2*cardPadY - r.groupChromeHeight()
	// Reserve the header band's own vertical padding (2*cellPadY + 2) plus one
	// body row (rowLineHt + 2*cellPadY); the rest is the header's line budget.
	n := int((region - (2*cellPadY + 2) - (rowLineHt + 2*cellPadY)) / rowLineHt)
	if n < 1 {
		n = 1
	}
	return n
}

func (r *designRenderer) headerLineCount() int {
	// Measure with the header font, then RESTORE the caller's font —
	// leaking fontTableHd into paginate() made body cells wrap narrower
	// in the measuring pass than in the drawing pass (lost lines).
	prevStyle, prevSize := r.curStyle, r.curSize
	_ = r.setFont(styleNormal, fontTableHd)
	lineCap := r.maxHeaderLines()
	maxLines := 1
	for i, col := range r.cols {
		if n := len(r.wrapCapped(col.Label, r.widths[i]-cellPadX, lineCap)); n > maxLines {
			maxLines = n
		}
	}
	if prevSize > 0 {
		_ = r.setFont(prevStyle, prevSize)
	}
	return maxLines
}

func (r *designRenderer) drawPage(p designPage, num int) error {
	r.pdf.AddPage()
	if err := r.drawBackground(); err != nil {
		return err
	}
	if _, err := r.drawHeader(); err != nil {
		return err
	}
	if err := r.drawCard(p); err != nil {
		return err
	}
	return r.drawFooter(num, r.total)
}

// dotBackgroundPDF renders the page background (gray-50 + the guide's dot
// grid) once, as a standalone single-page PDF held in memory. The real
// document imports that page as a Form XObject and stamps it per page —
// thousands of unique bezier circles compress terribly (~170 KB per page
// when drawn inline), a referenced template costs that once per document.
func dotBackgroundPDF(w, h float64) ([]byte, error) {
	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: gopdf.Rect{W: w, H: h}})
	pdf.AddPage()

	pdf.SetFillColor(colorPageBg.R, colorPageBg.G, colorPageBg.B)
	pdf.RectFromUpperLeftWithStyle(0, 0, w, h, "F")

	// gopdf's Oval strokes only (its cache emits a bezier path ending in
	// "S"), so a filled dot is drawn as a stroked circle of half the radius
	// with the line width set to the full radius: the stroke then covers
	// r=0..dotRadiusPt, giving a solid disc.
	pdf.SetStrokeColor(colorDot.R, colorDot.G, colorDot.B)
	pdf.SetLineWidth(dotRadiusPt)
	half := dotRadiusPt / 2
	for x := dotOffsetPt; x < w; x += dotSpacingPt {
		for y := dotOffsetPt; y < h; y += dotSpacingPt {
			pdf.Oval(x-half, y-half, x+half, y+half)
		}
	}

	var buf bytes.Buffer
	if _, err := pdf.WriteTo(&buf); err != nil {
		return nil, fmt.Errorf("render dot background: %w", err)
	}
	return buf.Bytes(), nil
}

// importDotBackground registers the shared background template on pdf.
func importDotBackground(pdf *gopdf.GoPdf, w, h float64) (int, error) {
	bg, err := dotBackgroundPDF(w, h)
	if err != nil {
		return 0, err
	}
	rs := io.ReadSeeker(bytes.NewReader(bg))
	// gofpdi template IDs are zero-based — the first import returns 0,
	// negative values signal failure.
	tpl := pdf.ImportPageStream(&rs, 1, "/MediaBox")
	if tpl < 0 {
		return 0, fmt.Errorf("import dot background template failed")
	}
	return tpl, nil
}

func (c *pageChrome) drawBackground() error {
	c.pdf.UseImportedTemplate(c.bgTpl, 0, 0, c.w, c.h)
	return nil
}

func (r *designRenderer) drawCard(p designPage) error {
	top := r.bodyTop()
	left := pageMargin
	right := r.w - pageMargin

	if err := r.drawCardSurface(p, top, left, right); err != nil {
		return err
	}

	y := top + cardPadY
	textLeft := left + cardPadX
	textRight := right - cardPadX

	// Group heading — the card's title: InfoCard's "text-base font-semibold
	// text-gray-900" with an mb-4 gap under it. Plain text, no filled band
	// (the app never fills one).
	if p.groupTitle != "" {
		if err := r.setFont(styleBold, fontGroup); err != nil {
			return err
		}
		r.setText(colorInk)
		if err := r.text(textLeft, y+fontGroup-2, p.groupTitle); err != nil {
			return err
		}
		y += fontGroup + groupGap
	}

	// Table header: muted labels over a hairline — DataTable's
	// "border-b border-gray-100 text-xs font-medium text-gray-500".
	// Labels wrap to their column width, top-aligned in the band.
	var err error
	y, err = r.drawCardTableHeader(textLeft, textRight, y)
	if err != nil {
		return err
	}

	return r.drawCardRows(p.rows, textLeft, textRight, y)
}

func (r *designRenderer) drawCardSurface(p designPage, top, left, right float64) error {
	// The card hugs its content (as the guide's cards do) instead of
	// stretching to the page foot, so a short class list doesn't render as
	// a mostly-empty sheet.
	// rowHeight measures with the *current* font — pin it to the body font
	// before measuring, or the pill font (set in drawHeader) is used.
	if err := r.setFont(styleNormal, fontBody); err != nil {
		return err
	}
	content := 2*cardPadY + r.tableHeaderHeight()
	if p.groupTitle != "" {
		content += fontGroup + groupGap
	}
	for _, row := range p.rows {
		content += row.height()
	}
	bottom := top + content
	if maxY := r.bodyBottom(); bottom > maxY {
		bottom = maxY
	}

	// Card surface.
	r.setFill(colorSurface)
	r.setStroke(colorBorder)
	r.pdf.SetLineWidth(0.6)
	return r.pdf.Rectangle(left, top, right, bottom, "FD", cardRadius, 12)

}

// drawAccentBars paints one bar per accent group: it starts at the accented
// line and runs down to the line before the next anchor, so a shift and the
// tasks under it read as one block. An anchor without a usable colour ends
// the bar above it and starts none of its own — otherwise the preceding
// colour would visually claim a group that belongs to another category.
func (r *designRenderer) drawAccentBars(x, top float64, lines []styledLine) {
	if r.gutter == 0 {
		return
	}
	start, color := -1, rgb{}
	flush := func(end int) {
		if start < 0 {
			return
		}
		barTop := top + float64(start)*rowLineHt + 1
		barBottom := top + float64(end)*rowLineHt - 1
		r.setFill(color)
		r.setStroke(color)
		_ = r.pdf.Rectangle(x, barTop, x+accentBarW, barBottom, "F", 1, 4)
		start = -1
	}
	for i, ln := range lines {
		if !ln.opensGroup() {
			continue
		}
		flush(i)
		if parsed, ok := parseHexColor(ln.accent); ok {
			start, color = i, parsed
		}
	}
	flush(len(lines))
}

// parseHexColor reads "#RRGGBB" and the shorthand "#RGB" — both shapes the
// colour columns accept (activities.Category, schedule.ShiftType,
// facilities.Room all validate against the same pattern). Anything else is
// ignored rather than guessed at.
func parseHexColor(value string) (rgb, bool) {
	value = strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(value) == 3 {
		// "#abc" means "#aabbcc": each digit stands for both nibbles.
		var expanded strings.Builder
		for _, digit := range value {
			expanded.WriteRune(digit)
			expanded.WriteRune(digit)
		}
		value = expanded.String()
	}
	if len(value) != 6 {
		return rgb{}, false
	}
	var out [3]uint8
	for i := range out {
		component, err := strconv.ParseUint(value[i*2:i*2+2], 16, 8)
		if err != nil {
			return rgb{}, false
		}
		out[i] = uint8(component)
	}
	return rgb{out[0], out[1], out[2]}, true
}

func (r *designRenderer) drawCardTableHeader(textLeft, textRight, y float64) (float64, error) {
	hh := r.tableHeaderHeight()
	if err := r.setFont(styleNormal, fontTableHd); err != nil {
		return 0, err
	}
	r.setText(colorHeaderText)
	// Cap the drawn labels at the same budget the band was sized with, or a
	// label that wrapped past the cap would draw more lines than the header
	// reserved and spill onto the rows below.
	lineCap := r.maxHeaderLines()
	cx := textLeft
	for i, col := range r.cols {
		ty := y + cellPadY + fontTableHd
		for _, ln := range r.wrapCapped(col.Label, r.widths[i]-cellPadX, lineCap) {
			if err := r.text(cx, ty, ln); err != nil {
				return 0, err
			}
			ty += rowLineHt
		}
		cx += r.widths[i]
	}
	y += hh
	r.setStroke(colorHeaderRule)
	r.pdf.SetLineWidth(0.7)
	r.pdf.Line(textLeft, y, textRight, y)

	return y, nil
}

func (r *designRenderer) drawCardRows(rows []renderRow, textLeft, textRight, y float64) error {
	// Body rows.
	if err := r.setFont(styleNormal, fontBody); err != nil {
		return err
	}
	for _, row := range rows {
		cx := textLeft
		for i := range r.cols {
			top := y + cellPadY
			r.drawAccentBars(cx, top, row.cells[i])
			ty := top + rowLineHt - 2
			for _, ln := range row.cells[i] {
				if err := r.setFont(fontStyleFor(ln.style), fontBody); err != nil {
					return err
				}
				r.setText(lineColorFor(ln.style))
				if err := r.text(cx+r.gutter, ty, ln.text); err != nil {
					return err
				}
				ty += rowLineHt
			}
			cx += r.widths[i]
		}
		y += row.height()
		r.setStroke(colorRowLine)
		r.pdf.SetLineWidth(0.5)
		r.pdf.Line(textLeft, y, textRight, y)
	}
	return nil
}

func splitStyledLines(lines []styledLine, start, maxLines int) []styledLine {
	if start >= len(lines) {
		return nil
	}
	end := start + maxLines
	if end > len(lines) {
		end = len(lines)
	}
	part := append([]styledLine(nil), lines[start:end]...)
	if start > 0 && !part[0].opensGroup() {
		for previous := start - 1; previous >= 0; previous-- {
			if !lines[previous].opensGroup() {
				continue
			}
			part[0].accent = lines[previous].accent
			part[0].opens = true
			break
		}
	}
	return part
}
